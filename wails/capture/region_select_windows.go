//go:build windows

package capture

// selectRegionNative replaces the PowerShell-based region selector.
// It creates a full-screen borderless Win32 window directly in-process,
// eliminating the ~800–1300 ms PowerShell cold-start delay.

import (
	"context"
	"fmt"
	"image"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/kbinani/screenshot"
)

// ── Win32 constants ──────────────────────────────────────────────────────────

const (
	rsWS_POPUP   uintptr = 0x80000000
	rsWS_VISIBLE uintptr = 0x10000000

	rsWS_EX_TOPMOST    uintptr = 0x00000008
	rsWS_EX_TOOLWINDOW uintptr = 0x00000080

	rsWM_DESTROY     uint32 = 0x0002
	rsWM_PAINT       uint32 = 0x000F
	rsWM_KEYDOWN     uint32 = 0x0100
	rsWM_LBUTTONDOWN uint32 = 0x0201
	rsWM_MOUSEMOVE   uint32 = 0x0200
	rsWM_LBUTTONUP   uint32 = 0x0202
	rsWM_QUIT        uint32 = 0x0012

	rsVK_ESCAPE uintptr = 0x1B

	rsSM_XVIRTUALSCREEN  uintptr = 76
	rsSM_YVIRTUALSCREEN  uintptr = 77
	rsSM_CXVIRTUALSCREEN uintptr = 78
	rsSM_CYVIRTUALSCREEN uintptr = 79

	rsCS_HREDRAW uint32 = 0x0002
	rsCS_VREDRAW uint32 = 0x0001

	rsSRCCOPY   uintptr = 0x00CC0020
	rsNULL_BRUSH uintptr = 5
	rsPS_SOLID   uintptr = 0
	rsSW_SHOW    uintptr = 5

	// COLORREF for red: 0x00BBGGRR → 0x000000FF
	rsColorRed uintptr = 0x000000FF

	rsDIB_RGB_COLORS uintptr = 0
	rsBI_RGB         uint32  = 0
)

// ── Win32 structs ────────────────────────────────────────────────────────────

type rsWNDCLASSEXW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type rsMSG struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      winPOINT // reuse existing type from capture_windows.go
}

type rsPAINTSTRUCT struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rsRECT
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type rsRECT struct{ left, top, right, bottom int32 }

type rsBITMAPINFOHEADER struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32 // negative = top-down
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type rsBITMAPINFO struct {
	bmiHeader rsBITMAPINFOHEADER
	bmiColors [1]uint32
}

// ── Win32 procs (use existing user32 var from capture_windows.go) ────────────

var (
	gdi32dll    = syscall.NewLazyDLL("gdi32.dll")
	kernel32dll = syscall.NewLazyDLL("kernel32.dll")

	procGetModuleHandleW    = kernel32dll.NewProc("GetModuleHandleW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procUnregisterClassW    = user32.NewProc("UnregisterClassW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procBeginPaint          = user32.NewProc("BeginPaint")
	procEndPaint            = user32.NewProc("EndPaint")
	procInvalidateRect      = user32.NewProc("InvalidateRect")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")

	procCreateCompatibleDC = gdi32dll.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32dll.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32dll.NewProc("CreateDIBSection")
	procSelectObject       = gdi32dll.NewProc("SelectObject")
	procDeleteObject       = gdi32dll.NewProc("DeleteObject")
	procBitBltGDI          = gdi32dll.NewProc("BitBlt")
	procCreatePen          = gdi32dll.NewProc("CreatePen")
	procGetStockObject     = gdi32dll.NewProc("GetStockObject")
	procRectangleGDI       = gdi32dll.NewProc("Rectangle")
)

// ── Shared state (WndProc lives on the locked OS thread) ─────────────────────

type rsState struct {
	hdcMem         uintptr
	vsX, vsY       int32
	vsW, vsH       int32
	startX, startY int32
	endX, endY     int32
	dragging       bool
	cancelled      bool
}

// rsGlobal is set just before ShowWindow and cleared on return from
// selectRegionNative. Access is safe because WndProc runs on the same
// OS-locked thread as the message loop.
var rsGlobal *rsState

func rsWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	s := rsGlobal
	if s == nil {
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	}

	switch uint32(msg) {
	case rsWM_PAINT:
		var ps rsPAINTSTRUCT
		dc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		// Blit captured screen into window.
		procBitBltGDI.Call(dc, 0, 0, uintptr(s.vsW), uintptr(s.vsH),
			s.hdcMem, 0, 0, rsSRCCOPY)
		// Draw rubber-band while dragging (all in one paint → no flicker).
		if s.dragging {
			x1, y1, x2, y2 := rsSelCoords(s)
			pen, _, _ := procCreatePen.Call(rsPS_SOLID, 2, rsColorRed)
			nullBr, _, _ := procGetStockObject.Call(rsNULL_BRUSH)
			oldPen, _, _ := procSelectObject.Call(dc, pen)
			oldBr, _, _ := procSelectObject.Call(dc, nullBr)
			procRectangleGDI.Call(dc,
				uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2))
			procSelectObject.Call(dc, oldPen)
			procSelectObject.Call(dc, oldBr)
			procDeleteObject.Call(pen)
		}
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0

	case rsWM_LBUTTONDOWN:
		s.startX = int32(int16(lParam & 0xFFFF))
		s.startY = int32(int16((lParam >> 16) & 0xFFFF))
		s.dragging = true
		procSetCapture.Call(hwnd)
		return 0

	case rsWM_MOUSEMOVE:
		if s.dragging {
			s.endX = int32(int16(lParam & 0xFFFF))
			s.endY = int32(int16((lParam >> 16) & 0xFFFF))
			procInvalidateRect.Call(hwnd, 0, 1) // erase=true redraws background cleanly
		}
		return 0

	case rsWM_LBUTTONUP:
		s.endX = int32(int16(lParam & 0xFFFF))
		s.endY = int32(int16((lParam >> 16) & 0xFFFF))
		s.dragging = false
		procReleaseCapture.Call()
		procDestroyWindow.Call(hwnd)
		return 0

	case rsWM_KEYDOWN:
		if wParam == rsVK_ESCAPE {
			s.cancelled = true
			procDestroyWindow.Call(hwnd)
		}
		return 0

	case rsWM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func rsSelCoords(s *rsState) (x1, y1, x2, y2 int32) {
	if s.startX < s.endX {
		x1, x2 = s.startX, s.endX
	} else {
		x1, x2 = s.endX, s.startX
	}
	if s.startY < s.endY {
		y1, y2 = s.startY, s.endY
	} else {
		y1, y2 = s.endY, s.startY
	}
	return
}

// selectRegionNative shows a full-screen Win32 overlay and lets the user drag
// a rubber-band selection. Returns the selected rectangle in physical virtual-
// screen coordinates (same coordinate space as screenshot.CaptureRect).
func selectRegionNative(ctx context.Context) (image.Rectangle, error) {
	// Win32 windows must be created on the same OS thread as their message loop.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Physical-pixel coordinates must match screenshot.CaptureRect.
	procSetProcessDPIAware.Call()

	// ── Virtual screen bounds ────────────────────────────────────────────────
	vsXr, _, _ := procGetSystemMetrics.Call(rsSM_XVIRTUALSCREEN)
	vsYr, _, _ := procGetSystemMetrics.Call(rsSM_YVIRTUALSCREEN)
	vsWr, _, _ := procGetSystemMetrics.Call(rsSM_CXVIRTUALSCREEN)
	vsHr, _, _ := procGetSystemMetrics.Call(rsSM_CYVIRTUALSCREEN)
	vsX, vsY := int32(vsXr), int32(vsYr)
	vsW, vsH := int32(vsWr), int32(vsHr)

	// ── Capture desktop as overlay background ────────────────────────────────
	// Use the same screenshot library used for CaptureRect so pixels align.
	bounds := image.Rect(int(vsX), int(vsY), int(vsX+vsW), int(vsY+vsH))
	bgImg, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return image.Rectangle{}, fmt.Errorf("capture background: %w", err)
	}

	// Convert RGBA image → Win32 HBITMAP (BGRA, top-down).
	hInst, _, _ := procGetModuleHandleW.Call(0)
	hdcMem, _, _ := procCreateCompatibleDC.Call(0)
	if hdcMem == 0 {
		return image.Rectangle{}, fmt.Errorf("CreateCompatibleDC failed")
	}

	bmi := rsBITMAPINFO{
		bmiHeader: rsBITMAPINFOHEADER{
			biSize:        uint32(unsafe.Sizeof(rsBITMAPINFOHEADER{})),
			biWidth:       vsW,
			biHeight:      -vsH, // negative = top-down
			biPlanes:      1,
			biBitCount:    32,
			biCompression: rsBI_RGB,
		},
	}
	var pvBits uintptr
	hbmMem, _, _ := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&bmi)),
		rsDIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&pvBits)),
		0, 0,
	)
	if hbmMem == 0 || pvBits == 0 {
		procDeleteDC.Call(hdcMem)
		return image.Rectangle{}, fmt.Errorf("CreateDIBSection failed")
	}

	// Copy RGBA → BGRA directly into the DIB pixel buffer.
	pixCount := int(vsW) * int(vsH)
	dst := unsafe.Slice((*byte)(unsafe.Pointer(pvBits)), pixCount*4)
	src := bgImg.Pix
	for i := 0; i < pixCount; i++ {
		s4, d4 := i*4, i*4
		dst[d4+0] = src[s4+2] // B
		dst[d4+1] = src[s4+1] // G
		dst[d4+2] = src[s4+0] // R
		dst[d4+3] = 0xFF
	}

	oldBm, _, _ := procSelectObject.Call(hdcMem, hbmMem)

	// ── Register window class ────────────────────────────────────────────────
	className, _ := syscall.UTF16PtrFromString("SnapVectorRegionSel")
	cursor, _, _ := procLoadCursorW.Call(0, 32515) // IDC_CROSS

	wcx := rsWNDCLASSEXW{
		cbSize:        uint32(unsafe.Sizeof(rsWNDCLASSEXW{})),
		style:         rsCS_HREDRAW | rsCS_VREDRAW,
		lpfnWndProc:   syscall.NewCallback(rsWndProc),
		hInstance:     hInst,
		hCursor:       cursor,
		hbrBackground: 0, // no GDI background erase; WM_PAINT owns all drawing
		lpszClassName: className,
	}
	atom, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcx)))
	if atom == 0 {
		procSelectObject.Call(hdcMem, oldBm)
		procDeleteObject.Call(hbmMem)
		procDeleteDC.Call(hdcMem)
		return image.Rectangle{}, fmt.Errorf("RegisterClassExW failed")
	}

	// ── Create full-screen topmost borderless window ─────────────────────────
	hwnd, _, _ := procCreateWindowExW.Call(
		rsWS_EX_TOPMOST|rsWS_EX_TOOLWINDOW,
		uintptr(unsafe.Pointer(className)),
		0,
		rsWS_POPUP|rsWS_VISIBLE,
		uintptr(uint32(vsX)), uintptr(uint32(vsY)), // uint32 cast for correct Win32 sign handling
		uintptr(vsW), uintptr(vsH),
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), hInst)
		procSelectObject.Call(hdcMem, oldBm)
		procDeleteObject.Call(hbmMem)
		procDeleteDC.Call(hdcMem)
		return image.Rectangle{}, fmt.Errorf("CreateWindowExW failed")
	}

	// ── Activate overlay ─────────────────────────────────────────────────────
	s := &rsState{
		hdcMem: hdcMem,
		vsX: vsX, vsY: vsY,
		vsW: vsW, vsH: vsH,
	}
	rsGlobal = s

	procShowWindow.Call(hwnd, rsSW_SHOW)
	procSetForegroundWindow.Call(hwnd)

	// ── Message loop ─────────────────────────────────────────────────────────
	var msg rsMSG
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	// ── Cleanup ───────────────────────────────────────────────────────────────
	rsGlobal = nil
	procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), hInst)
	procSelectObject.Call(hdcMem, oldBm)
	procDeleteObject.Call(hbmMem)
	procDeleteDC.Call(hdcMem)

	if s.cancelled {
		return image.Rectangle{}, nil
	}

	x1, y1, x2, y2 := rsSelCoords(s)
	// Convert client coords → virtual screen coords.
	x1 += vsX; y1 += vsY
	x2 += vsX; y2 += vsY
	if x1 == x2 || y1 == y2 {
		return image.Rectangle{}, nil // zero-size selection
	}
	return image.Rect(int(x1), int(y1), int(x2), int(y2)), nil
}
