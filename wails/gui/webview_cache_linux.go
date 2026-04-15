//go:build linux

package gui

import (
	"fmt"
	"os"
	"path/filepath"
)

// binaryFingerprintStampFile is the filename inside the per-user WebKit
// cache directory where we persist the fingerprint of the binary that
// last populated that cache. When the running binary's fingerprint
// differs, we wipe the cached WebKit assets so a replaced (upgraded)
// binary does not keep serving stale front-end files.
const binaryFingerprintStampFile = ".binary-fp"

// webKitCacheSubdirs are the webkit2gtk cache subdirectories that can
// cause stale HTML/JS to be served after a binary upgrade. storage,
// hsts, mediakeys and other user-visible state are deliberately left
// alone — we only invalidate the asset cache, not user data.
var webKitCacheSubdirs = []string{"WebKitCache", "CacheStorage"}

// bustWebViewCacheIfBinaryChanged wipes the WebKit asset caches under
// $XDG_CACHE_HOME/snapvector when the running executable differs from
// the one that last populated the cache. It is a no-op when the binary
// is unchanged, and silently skips any filesystem errors so it cannot
// block GUI startup.
func bustWebViewCacheIfBinaryChanged() {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	fp, ok := currentBinaryFingerprint()
	if !ok {
		return
	}
	bustWebViewCacheIfChanged(filepath.Join(cacheDir, "snapvector"), fp)
}

// bustWebViewCacheIfChanged is the filesystem-only core of the cache
// bust: compare fp against the persisted stamp under appCache, wipe
// webkit asset dirs on mismatch, then rewrite the stamp. Pulled out as
// its own function so tests can drive it with a temp directory.
func bustWebViewCacheIfChanged(appCache, fp string) {
	stamp := filepath.Join(appCache, binaryFingerprintStampFile)
	if prev, err := os.ReadFile(stamp); err == nil && string(prev) == fp {
		return
	}

	for _, sub := range webKitCacheSubdirs {
		_ = os.RemoveAll(filepath.Join(appCache, sub))
	}
	if err := os.MkdirAll(appCache, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(stamp, []byte(fp), 0o644)
}

// currentBinaryFingerprint returns a short identifier for the running
// executable based on size and mtime. dpkg install, install(1) and
// `wails build` all touch mtime, so size+mtime is sufficient to detect
// a replaced binary without paying the cost of hashing 9MB on every
// launch. A spurious `touch` only costs one WebKit cache rebuild.
func currentBinaryFingerprint() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", false
	}
	info, err := os.Stat(exe)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%d-%d", info.Size(), info.ModTime().UnixNano()), true
}
