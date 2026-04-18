# Third-party binary: resvg

This directory contains a pre-built `resvg` CLI binary bundled into snapvector
via `go:embed` and used by `convert_darwin.go` as the macOS SVG rasterizer
(replacing macOS `sips`, which has text-anchor bugs).

- **Name:** resvg
- **Version:** 0.47.0
- **Source:** https://github.com/linebender/resvg
- **License:** Mozilla Public License 2.0 (MPL-2.0)
- **Upstream releases:** https://github.com/linebender/resvg/releases/tag/v0.47.0

The binary is a universal Mach-O (arm64 + x86_64) produced by `lipo -create`
from the official `resvg-macos-aarch64.zip` and `resvg-macos-x86_64.zip`
release artifacts. It is unmodified.

Per MPL-2.0 §3.2, source code for resvg remains available at the Source URL
above.
