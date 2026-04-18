//go:build darwin

// Package capture's cgo bridge for macOS.
//
// Rationale for cgo: the fork-based approach (spawning /usr/sbin/screencapture
// and a `swift` subprocess) cost 200–400 ms per capture on a typical Mac — two
// process forks plus disk round-trips through a temp PNG. Going to the native
// CoreGraphics API removes both. Wails on macOS already pulls in cgo for the
// Cocoa WebView, so the incremental build/cold-start cost of this file is
// effectively zero.
//
// Surface area is kept minimal: this file only exposes thin wrappers around
// CoreGraphics + ImageIO calls. All higher-level orchestration (cursor
// matching, multi-display compose, error classification) lives in
// capture_darwin.go in pure Go.
//
// CGDisplayCreateImage is marked deprecated on macOS 14+, but remains functional
// and is still the simplest sync screenshot API. ScreenCaptureKit is the
// long-term successor; migration is tracked as a phase-2 follow-up.

package capture

/*
#cgo LDFLAGS: -framework CoreGraphics -framework ImageIO -framework CoreFoundation
#cgo CFLAGS: -Wno-deprecated-declarations -mmacosx-version-min=10.15

#include <CoreGraphics/CoreGraphics.h>
#include <ImageIO/ImageIO.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>
#include <string.h>

// CGDisplayCreateImage is marked unavailable in the macOS 15 SDK header, but
// the symbol is still exported by CoreGraphics.framework and functional at
// runtime on macOS 10.15 through 15. We re-declare it via asm alias so the
// compiler doesn't refuse to link. Migration to ScreenCaptureKit is tracked
// as phase-2 work; until then this alias is the canonical workaround used by
// most open-source macOS screenshot tools.
extern CGImageRef sv_CGDisplayCreateImage(CGDirectDisplayID display) __asm__("_CGDisplayCreateImage");

// Return number of active displays, or -1 on error.
// Fills ids with up to maxCount IDs.
static int snapvector_list_displays(uint32_t* ids, int maxCount) {
    uint32_t count = 0;
    if (CGGetActiveDisplayList((uint32_t)maxCount, ids, &count) != kCGErrorSuccess) {
        return -1;
    }
    return (int)count;
}

// Fills geometry for one display. Bounds are in global display coordinates
// (points, top-left origin). pixels are backing pixels.
static void snapvector_display_geometry(
    uint32_t displayID,
    double* x, double* y, double* width, double* height,
    int* pixelsWide, int* pixelsHigh
) {
    CGRect bounds = CGDisplayBounds(displayID);
    *x = bounds.origin.x;
    *y = bounds.origin.y;
    *width = bounds.size.width;
    *height = bounds.size.height;
    *pixelsWide = (int)CGDisplayPixelsWide(displayID);
    *pixelsHigh = (int)CGDisplayPixelsHigh(displayID);
}

// Return 1 if Screen Recording permission is granted, 0 otherwise.
// CGPreflightScreenCaptureAccess is macOS 10.15+; LSMinimumSystemVersion
// gates this at the plist level.
static int snapvector_preflight_screen_capture(void) {
    return CGPreflightScreenCaptureAccess() ? 1 : 0;
}

// Trigger the Screen Recording permission dialog and register this app in
// the Privacy panel. Returns 1 if permission is already granted at the time
// of call, 0 otherwise (the dialog appears asynchronously; the NEXT capture
// attempt will succeed once the user grants).
static int snapvector_request_screen_capture(void) {
    return CGRequestScreenCaptureAccess() ? 1 : 0;
}

// Cursor location in global display coords (points, top-left).
static void snapvector_cursor_location(double* x, double* y) {
    CGEventRef evt = CGEventCreate(NULL);
    CGPoint p = CGEventGetLocation(evt);
    CFRelease(evt);
    *x = p.x;
    *y = p.y;
}

// Capture one display to a malloc'd PNG buffer.
// Return codes:
//    0 = success, *outBuf / *outLen populated, caller must free(*outBuf)
//   -1 = CGDisplayCreateImage returned NULL (permission or disconnected display)
//   -2 = PNG encoding failed
static int snapvector_capture_display_png(
    uint32_t displayID,
    unsigned char** outBuf,
    int* outLen
) {
    CGImageRef img = sv_CGDisplayCreateImage(displayID);
    if (!img) return -1;

    CFMutableDataRef data = CFDataCreateMutable(NULL, 0);
    if (!data) { CGImageRelease(img); return -2; }

    CGImageDestinationRef dest = CGImageDestinationCreateWithData(
        data, CFSTR("public.png"), 1, NULL);
    if (!dest) {
        CFRelease(data);
        CGImageRelease(img);
        return -2;
    }

    CGImageDestinationAddImage(dest, img, NULL);
    bool ok = CGImageDestinationFinalize(dest);
    CFRelease(dest);
    CGImageRelease(img);

    if (!ok) {
        CFRelease(data);
        return -2;
    }

    CFIndex len = CFDataGetLength(data);
    unsigned char* buf = (unsigned char*)malloc((size_t)len);
    if (!buf) {
        CFRelease(data);
        return -2;
    }
    CFDataGetBytes(data, CFRangeMake(0, len), buf);
    CFRelease(data);

    *outBuf = buf;
    *outLen = (int)len;
    return 0;
}

// Capture one display into a row-packed RGBA8 buffer. This skips the
// ImageIO PNG encode/decode round-trip that snapvector_capture_display_png
// pays for; for multi-display compose the encoded PNG is a throwaway since
// composeDisplayCaptures tears it apart again into an image.RGBA anyway.
// Return codes:
//    0 = success, *outBuf populated with width*height*4 bytes of RGBA
//        (premultiplied alpha, byte order R,G,B,A), caller must free(*outBuf)
//   -1 = CGDisplayCreateImage returned NULL
//   -2 = bitmap context alloc / draw failed
static int snapvector_capture_display_rgba(
    uint32_t displayID,
    unsigned char** outBuf,
    int* outW,
    int* outH
) {
    CGImageRef img = sv_CGDisplayCreateImage(displayID);
    if (!img) return -1;

    size_t w = CGImageGetWidth(img);
    size_t h = CGImageGetHeight(img);
    size_t bytesPerRow = 4 * w;

    CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
    if (!cs) { CGImageRelease(img); return -2; }

    unsigned char* buf = (unsigned char*)calloc(bytesPerRow * h, 1);
    if (!buf) {
        CGColorSpaceRelease(cs);
        CGImageRelease(img);
        return -2;
    }

    // kCGImageAlphaPremultipliedLast + kCGBitmapByteOrder32Big = R,G,B,A byte
    // order, which matches Go's image.RGBA pixel layout one-to-one.
    CGContextRef ctx = CGBitmapContextCreate(
        buf, w, h, 8, bytesPerRow, cs,
        kCGImageAlphaPremultipliedLast | kCGBitmapByteOrder32Big);
    CGColorSpaceRelease(cs);
    if (!ctx) {
        free(buf);
        CGImageRelease(img);
        return -2;
    }

    CGContextDrawImage(ctx, CGRectMake(0, 0, (CGFloat)w, (CGFloat)h), img);
    CGContextRelease(ctx);
    CGImageRelease(img);

    *outBuf = buf;
    *outW = (int)w;
    *outH = (int)h;
    return 0;
}
*/
import "C"

import (
	"fmt"
	"image"
	"math"
	"unsafe"
)

const maxDisplaysProbed = 32

// cgPreflightScreenCapture reports whether the current process has Screen
// Recording permission, without raising a system dialog.
func cgPreflightScreenCapture() bool {
	return C.snapvector_preflight_screen_capture() != 0
}

// cgRequestScreenCapture triggers the macOS Screen Recording permission
// dialog and registers this process in the Privacy panel. Returns true if
// permission was already granted; false means the dialog is showing or the
// user has previously denied. Must be called at least once per app
// installation so TCC knows the app exists — without it, preflight will
// always report false and no capture will succeed.
func cgRequestScreenCapture() bool {
	return C.snapvector_request_screen_capture() != 0
}

// cgListDisplays enumerates active displays via CoreGraphics and returns
// darwinDisplay records with X/Y/Width/Height in backing pixels (matching
// the historical contract from the Swift probe).
func cgListDisplays() ([]darwinDisplay, error) {
	var ids [maxDisplaysProbed]C.uint32_t
	n := C.snapvector_list_displays(&ids[0], C.int(maxDisplaysProbed))
	if n < 0 {
		return nil, fmt.Errorf("CGGetActiveDisplayList failed")
	}
	if n == 0 {
		return nil, fmt.Errorf("no active displays")
	}

	var cursorX, cursorY C.double
	C.snapvector_cursor_location(&cursorX, &cursorY)
	cx, cy := float64(cursorX), float64(cursorY)

	displays := make([]darwinDisplay, 0, int(n))
	for i := 0; i < int(n); i++ {
		var px, py, pw, ph C.double
		var pixW, pixH C.int
		C.snapvector_display_geometry(ids[i], &px, &py, &pw, &ph, &pixW, &pixH)

		scale := 1.0
		if pw > 0 {
			scale = float64(pixW) / float64(pw)
		}

		contains := cx >= float64(px) && cx < float64(px)+float64(pw) &&
			cy >= float64(py) && cy < float64(py)+float64(ph)

		// Store X/Y/Width/Height in POINTS (CGDisplayBounds' unified global
		// coord space). The earlier form multiplied by each display's own
		// scale to produce "per-display pixel" values — in mixed-DPI setups
		// (Retina + 1x external, or 150%/200% scale choice) the different
		// scales produced overlapping or gap-ridden layouts during compose.
		// Keeping points means all displays live in one coordinate system;
		// compose pins a shared canvas pixel density via targetScale.
		displays = append(displays, darwinDisplay{
			Index:          i + 1,
			X:              int(math.Round(float64(px))),
			Y:              int(math.Round(float64(py))),
			Width:          int(math.Round(float64(pw))),
			Height:         int(math.Round(float64(ph))),
			ScaleFactor:    scale,
			ContainsCursor: contains,
			cgDisplayID:    uint32(ids[i]),
		})
	}
	return displays, nil
}

// cgCaptureDisplayPNG captures a single display identified by its CGDisplayID
// and returns PNG bytes encoded via ImageIO. The caller must NOT free the
// returned slice — Go owns it after C.GoBytes copies out of the C buffer.
func cgCaptureDisplayPNG(id uint32) ([]byte, error) {
	var buf *C.uchar
	var length C.int
	rc := C.snapvector_capture_display_png(C.uint32_t(id), &buf, &length)
	switch rc {
	case 0:
		defer C.free(unsafe.Pointer(buf))
		return C.GoBytes(unsafe.Pointer(buf), length), nil
	case -1:
		return nil, fmt.Errorf("CGDisplayCreateImage returned NULL (display=%d)", id)
	case -2:
		return nil, fmt.Errorf("PNG encoding failed (display=%d)", id)
	default:
		return nil, fmt.Errorf("unexpected capture rc=%d (display=%d)", rc, id)
	}
}

// cgCaptureDisplayRGBA captures a display straight into Go's native
// image.RGBA pixel layout (row-packed, R,G,B,A byte order, premultiplied
// alpha). Used by the multi-display compose path: the intermediate PNG
// encode/decode round-trip that cgCaptureDisplayPNG pays for is pure waste
// when composeDisplayCaptures is going to decode right back to RGBA anyway.
// The image shares the underlying C buffer only through the returned slice;
// image.RGBA owns a copy so the caller does not need to free anything.
func cgCaptureDisplayRGBA(id uint32) (*image.RGBA, error) {
	var buf *C.uchar
	var w, h C.int
	rc := C.snapvector_capture_display_rgba(C.uint32_t(id), &buf, &w, &h)
	switch rc {
	case 0:
		defer C.free(unsafe.Pointer(buf))
		width, height := int(w), int(h)
		stride := width * 4
		pix := C.GoBytes(unsafe.Pointer(buf), C.int(stride*height))
		return &image.RGBA{
			Pix:    pix,
			Stride: stride,
			Rect:   image.Rect(0, 0, width, height),
		}, nil
	case -1:
		return nil, fmt.Errorf("CGDisplayCreateImage returned NULL (display=%d)", id)
	case -2:
		return nil, fmt.Errorf("bitmap context create/draw failed (display=%d)", id)
	default:
		return nil, fmt.Errorf("unexpected capture rc=%d (display=%d)", rc, id)
	}
}
