#!/bin/bash
# Build and bundle WarpLocal.app with Warp as the main app and the local adapter as a helper.
#
# Usage:
#   WARP_SRC=/path/to/warp-source ./build_and_bundle.sh [--launch]
#
# Prerequisites:
#   - Go toolchain
#   - Rust toolchain + Warp source (for building local Warp client)

set -euo pipefail
export PATH="$HOME/.cargo/bin:$HOME/go/bin:$PATH"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WARP_SRC="${WARP_SRC:-}"
BUNDLE_DIR="$SCRIPT_DIR/WarpLocal.app"
ASSETS_DIR="$SCRIPT_DIR/assets"
GO_CACHE_DIR="$SCRIPT_DIR/.gocache"
GO_TMP_DIR="$SCRIPT_DIR/.gotmp"

if [[ -z "$WARP_SRC" ]]; then
  for candidate in \
    "$SCRIPT_DIR/../warp-v0.2026.04.29.08.56.stable_00-src/warp-0.2026.04.29.08.56.stable_00" \
    "$HOME/warp" \
    "$SCRIPT_DIR/../warp"
  do
    if [[ -f "$candidate/Cargo.toml" ]]; then
      WARP_SRC="$candidate"
      break
    fi
  done
fi

if [[ -z "$WARP_SRC" ]]; then
  echo "Warp source not found."
  echo "Set WARP_SRC to a local patched Warp source tree before running this script."
  echo "Example:"
  echo "  WARP_SRC=/path/to/warp-source ./build_and_bundle.sh"
  exit 1
fi

echo "Using WARP_SRC=$WARP_SRC"

echo "=== Step 1/5: Building warp-local-adapter (Go server) ==="
cd "$SCRIPT_DIR"
mkdir -p "$SCRIPT_DIR/bin" "$GO_CACHE_DIR" "$GO_TMP_DIR"
GOCACHE="$GO_CACHE_DIR" GOTMPDIR="$GO_TMP_DIR" GOFLAGS="-buildvcs=false" \
  go build -o "$SCRIPT_DIR/bin/warp-local-adapter" ./cmd/server
echo "  -> bin/warp-local-adapter"

echo ""
echo "=== Step 2/5: Building warp (WarpLocal client binary) ==="
cd "$WARP_SRC"
cargo build --bin warp -F skip_firebase_anonymous_user
echo "  -> target/debug/warp"

echo ""
echo "=== Step 3/5: Creating app bundle ==="
mkdir -p "$BUNDLE_DIR/Contents/MacOS"
mkdir -p "$BUNDLE_DIR/Contents/Helpers"
mkdir -p "$BUNDLE_DIR/Contents/Resources"
rm -f "$BUNDLE_DIR/Contents/MacOS/warplocal" "$BUNDLE_DIR/Contents/Helpers/warp-core"

# Copy binaries
cp "$WARP_SRC/target/debug/warp" "$BUNDLE_DIR/Contents/MacOS/warp"
chmod +x "$BUNDLE_DIR/Contents/MacOS/warp"

cp "$SCRIPT_DIR/bin/warp-local-adapter" "$BUNDLE_DIR/Contents/Helpers/warp-local-adapter"
chmod +x "$BUNDLE_DIR/Contents/Helpers/warp-local-adapter"

# Copy example config
cp "$SCRIPT_DIR/config.example.yaml" "$BUNDLE_DIR/Contents/Resources/config.example.yaml"

# Copy diagnostics script
cp "$SCRIPT_DIR/diagnostics.sh" "$BUNDLE_DIR/Contents/Resources/diagnostics.sh"
chmod +x "$BUNDLE_DIR/Contents/Resources/diagnostics.sh"

# Copy icon if present
ICON_DEST="$BUNDLE_DIR/Contents/Resources/iconfile.icns"
if [[ -f "$ASSETS_DIR/iconfile.icns" ]]; then
  cp "$ASSETS_DIR/iconfile.icns" "$ICON_DEST"
elif [[ -f "$ASSETS_DIR/AppIcon.icns" ]]; then
  cp "$ASSETS_DIR/AppIcon.icns" "$ICON_DEST"
elif [[ -f "$WARP_SRC/app/channels/local/icon/no-padding/512x512.png" ]]; then
  cp "$WARP_SRC/app/channels/local/icon/no-padding/512x512.png" "$BUNDLE_DIR/Contents/Resources/warp-local-icon.png"
fi

# Write Info.plist
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
	<string>warp</string>
	<key>CFBundleIdentifier</key>
	<string>dev.warp.Warp-Local</string>
	<key>CFBundleIconFile</key>
	<string>iconfile</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>WarpLocal</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>0.2.0</string>
	<key>CFBundleVersion</key>
	<string>0.2.0</string>
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

echo ""
echo "=== Step 4/5: Registering URL scheme ==="
LSREGISTER=$(find /System/Library/Frameworks/CoreServices.framework -name lsregister 2>/dev/null | head -1)
"$LSREGISTER" -f "$BUNDLE_DIR" 2>/dev/null || true

echo ""
echo "=== Done ==="
echo ""
echo "Bundle: $BUNDLE_DIR"
echo ""
echo "Contents:"
echo "  MacOS/warp               (WarpLocal main application)"
echo "  Helpers/warp-local-adapter (AI backend)"
echo "  Resources/config.example.yaml"
echo ""
echo "To launch, run:"
echo "  open $BUNDLE_DIR"
echo ""

# Optionally launch
if [[ "${1:-}" == "--launch" ]]; then
    echo "Launching..."
    open "$BUNDLE_DIR"
fi
