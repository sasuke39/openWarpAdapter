#!/bin/bash
# Build and bundle a local Warp client app for use with this adapter.
# This is an optional helper for contributors who are working with a local,
# patched Warp source tree. It is not required for running the adapter itself.

set -euo pipefail
export PATH="$HOME/.cargo/bin:$PATH"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WARP_SRC="${WARP_SRC:-}"
BUNDLE_DIR="$SCRIPT_DIR/WarpLocal.app"

if [[ -z "$WARP_SRC" ]]; then
  echo "WARP_SRC is not set."
  echo "Please point it to a local patched Warp source tree before running this script."
  echo "Example:"
  echo "  WARP_SRC=/path/to/warp-source ./build_and_bundle.sh"
  exit 1
fi

BINARY_SRC="$WARP_SRC/target/debug/warp-oss"
BINARY_DST="$BUNDLE_DIR/Contents/MacOS/warp-oss"

echo "=== Building warp-oss ==="
cd "$WARP_SRC"
cargo build --bin warp-oss -F skip_firebase_anonymous_user

echo ""
echo "=== Creating app bundle ==="
mkdir -p "$BUNDLE_DIR/Contents/MacOS"
mkdir -p "$BUNDLE_DIR/Contents/Resources"

cp "$BINARY_SRC" "$BINARY_DST"
chmod +x "$BINARY_DST"

# Write Info.plist (idempotent)
cat > "$BUNDLE_DIR/Contents/Info.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>English</string>
	<key>CFBundleDisplayName</key>
	<string>WarpLocal</string>
	<key>CFBundleExecutable</key>
	<string>warp-oss</string>
	<key>CFBundleIdentifier</key>
	<string>dev.warp.WarpLocal</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>WarpLocal</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>0.1.0</string>
	<key>LSApplicationCategoryType</key>
	<string>public.app-category.developer-tools</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>CFBundleURLTypes</key>
	<array>
		<dict>
			<key>CFBundleURLName</key>
			<string>WarpLocal URL Scheme</string>
			<key>CFBundleURLSchemes</key>
			<array>
				<string>warplocal</string>
			</array>
		</dict>
	</array>
</dict>
</plist>
PLIST

echo "=== Registering URL scheme ==="
LSREGISTER=$(find /System/Library/Frameworks/CoreServices.framework -name lsregister 2>/dev/null | head -1)
"$LSREGISTER" -f "$BUNDLE_DIR" 2>/dev/null

echo ""
echo "=== Done ==="
echo "Bundle: $BUNDLE_DIR"
echo ""
echo "To launch, run:"
echo "  open $BUNDLE_DIR"
echo ""
echo "Or double-click WarpLocal.app in Finder."

# Optionally launch
if [[ "${1:-}" == "--launch" ]]; then
    echo "Launching..."
    open "$BUNDLE_DIR"
fi
