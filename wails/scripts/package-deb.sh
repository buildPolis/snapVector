#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PACKAGE_NAME="${PACKAGE_NAME:-snapvector}"
APP_NAME="${APP_NAME:-SnapVector}"
ARCH="${ARCH:-$(dpkg --print-architecture)}"
OUTPUT_DIR="${OUTPUT_DIR:-$ROOT_DIR/build/packages}"
BIN_PATH="$ROOT_DIR/build/bin/snapvector"
ICON_PATH="$ROOT_DIR/build/appicon.png"
WAILS_BUILD_TAGS="${WAILS_BUILD_TAGS:-webkit2_41 production}"
DEPENDS="${DEPENDS:-libgtk-3-0 | libgtk-3-0t64, libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-37, librsvg2-bin, xclip | wl-clipboard}"
RECOMMENDS="${RECOMMENDS:-xdg-desktop-portal, gnome-screenshot | grim}"
HOMEPAGE="${HOMEPAGE:-}"

version_from_source() {
  sed -n 's/^const versionString = "\(.*\)"$/\1/p' "$ROOT_DIR/cli/cli.go"
}

default_maintainer() {
  local name email
  name="$(git -C "$ROOT_DIR" config user.name 2>/dev/null || true)"
  email="$(git -C "$ROOT_DIR" config user.email 2>/dev/null || true)"
  if [[ -n "$name" && -n "$email" ]]; then
    printf '%s <%s>' "$name" "$email"
    return
  fi
  printf 'SnapVector <noreply@snapvector.local>'
}

VERSION="${VERSION:-$(version_from_source)}"
MAINTAINER="${MAINTAINER:-$(default_maintainer)}"

if [[ -z "$VERSION" ]]; then
  echo "failed to detect version from cli/cli.go; set VERSION=... explicitly" >&2
  exit 1
fi

for cmd in wails npm dpkg-deb desktop-file-validate; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "missing required command: $cmd" >&2
    exit 1
  fi
done

if [[ ! -f "$ICON_PATH" ]]; then
  echo "missing app icon: $ICON_PATH" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

PKG_ROOT="$TMP_DIR/${PACKAGE_NAME}_${VERSION}_${ARCH}"
mkdir -p \
  "$PKG_ROOT/DEBIAN" \
  "$PKG_ROOT/usr/bin" \
  "$PKG_ROOT/usr/share/applications" \
  "$PKG_ROOT/usr/share/icons/hicolor/512x512/apps"

echo "building production binary with Wails..."
(
  cd "$ROOT_DIR"
  wails build -clean -tags "$WAILS_BUILD_TAGS"
)

if [[ ! -x "$BIN_PATH" ]]; then
  echo "expected built binary at $BIN_PATH" >&2
  exit 1
fi

install -m 755 "$BIN_PATH" "$PKG_ROOT/usr/bin/$PACKAGE_NAME"
install -m 644 "$ICON_PATH" "$PKG_ROOT/usr/share/icons/hicolor/512x512/apps/$PACKAGE_NAME.png"

DESKTOP_FILE="$PKG_ROOT/usr/share/applications/$PACKAGE_NAME.desktop"
cat >"$DESKTOP_FILE" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=$APP_NAME
Comment=Cross-platform screenshot and vector annotation tool
Terminal=false
Categories=Graphics;Utility;
StartupNotify=true
StartupWMClass=Snapvector
Exec=/usr/bin/$PACKAGE_NAME %U
TryExec=/usr/bin/$PACKAGE_NAME
Icon=$PACKAGE_NAME
EOF

desktop-file-validate "$DESKTOP_FILE"

CONTROL_FILE="$PKG_ROOT/DEBIAN/control"
{
  echo "Package: $PACKAGE_NAME"
  echo "Version: $VERSION"
  echo "Section: graphics"
  echo "Priority: optional"
  echo "Architecture: $ARCH"
  echo "Maintainer: $MAINTAINER"
  echo "Depends: $DEPENDS"
  if [[ -n "$RECOMMENDS" ]]; then
    echo "Recommends: $RECOMMENDS"
  fi
  if [[ -n "$HOMEPAGE" ]]; then
    echo "Homepage: $HOMEPAGE"
  fi
  cat <<'EOF'
Description: SnapVector screenshot and vector annotation tool
 SnapVector is a cross-platform screenshot and annotation tool with a Wails GUI
 and CLI-first workflow. This package installs the Linux desktop launcher,
 hicolor app icon, and the snapvector binary under /usr/bin.
EOF
} >"$CONTROL_FILE"

POSTINST_FILE="$PKG_ROOT/DEBIAN/postinst"
cat >"$POSTINST_FILE" <<'EOF'
#!/bin/sh
set -e

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -f -t /usr/share/icons/hicolor || true
fi
EOF

POSTRM_FILE="$PKG_ROOT/DEBIAN/postrm"
cat >"$POSTRM_FILE" <<'EOF'
#!/bin/sh
set -e

if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database /usr/share/applications || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -f -t /usr/share/icons/hicolor || true
fi
EOF

chmod 755 "$POSTINST_FILE" "$POSTRM_FILE"

mkdir -p "$OUTPUT_DIR"
DEB_PATH="$OUTPUT_DIR/${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"
rm -f "$DEB_PATH"

dpkg-deb --build --root-owner-group "$PKG_ROOT" "$DEB_PATH" >/dev/null

echo "built $DEB_PATH"
