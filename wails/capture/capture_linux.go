//go:build linux

package capture

import "context"

func NewPlatformCapturer() Capturer { return stubCapturer{platform: "linux"} }

type stubCapturer struct {
	platform string
}

func (s stubCapturer) CaptureFullScreen(context.Context) (PNG, Meta, error) {
	return nil, Meta{}, &UnsupportedPlatformError{Platform: s.platform}
}

func (s stubCapturer) CaptureInteractiveRegion(context.Context) (PNG, Meta, error) {
	return nil, Meta{}, &UnsupportedPlatformError{Platform: s.platform}
}
