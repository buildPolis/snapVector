package capture

import "context"

type PNG []byte

type Meta struct {
	DisplayID   string
	X           int
	Y           int
	Width       int
	Height      int
	ScaleFactor float64
}

type Capturer interface {
	CaptureFullScreen(ctx context.Context) (PNG, Meta, error)
	CaptureInteractiveRegion(ctx context.Context) (PNG, Meta, error)
}

type PermissionDeniedError struct {
	Platform string
	Stderr   string
}

func (e *PermissionDeniedError) Error() string {
	return "screen capture permission denied"
}

type UnsupportedPlatformError struct {
	Platform string
}

func (e *UnsupportedPlatformError) Error() string {
	return "screen capture is not implemented on " + e.Platform
}
