//go:build !linux

package gui

// bustWebViewCacheIfBinaryChanged is a no-op on non-Linux platforms.
// The cache staleness problem is specific to webkit2gtk's asset cache;
// macOS (WKWebView) and Windows (WebView2) use different cache models
// that do not require this bust step when the binary is replaced.
func bustWebViewCacheIfBinaryChanged() {}
