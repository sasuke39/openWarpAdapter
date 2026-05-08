#!/bin/bash
# WarpLocal Installer & Diagnostics
#
# Usage:
#   ./install.sh                    — Download pre-built WarpLocal.app
#   ./install.sh --build            — Build from source (requires Rust + Go)
#   ./install.sh doctor             — Generate diagnostics for bug reports
#
# Options:
#   --build          Build from source instead of downloading
#   --warp-src DIR   Path to Warp source tree (for --build, or set WARP_SRC)
#   --launch         Launch after installation

set -euo pipefail

REPO="https://github.com/sasuke39/open-warp"
INSTALL_DIR="/Applications"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ─── Pre-built download path ────────────────────────────────────────────────

install_prebuilt() {
    info "Downloading pre-built WarpLocal.app..."
    local release_url="$REPO/releases/latest/download/WarpLocal.app.zip"
    local tmp_zip="/tmp/WarpLocal.app.zip"

    if command -v curl &>/dev/null; then
        if curl -fL "$release_url" -o "$tmp_zip"; then
            info "Downloaded latest release from GitHub"
            rm -rf "$INSTALL_DIR/WarpLocal.app"
            ditto -x -k "$tmp_zip" "$INSTALL_DIR"
            xattr -cr "$INSTALL_DIR/WarpLocal.app" 2>/dev/null || true
            info "Installed to $INSTALL_DIR/WarpLocal.app (quarantine cleared)"
            return
        fi
        warn "Latest GitHub release is not available yet; falling back to local bundle detection"
    fi

    if [[ -d "$SCRIPT_DIR/WarpLocal.app" ]]; then
        info "Found local WarpLocal.app bundle"
        rm -rf "$INSTALL_DIR/WarpLocal.app"
        cp -R "$SCRIPT_DIR/WarpLocal.app" "$INSTALL_DIR/WarpLocal.app"
        xattr -cr "$INSTALL_DIR/WarpLocal.app" 2>/dev/null || true
        info "Installed to $INSTALL_DIR/WarpLocal.app (quarantine cleared)"
        return
    fi
    error "Pre-built WarpLocal.app not available yet. Use --build to compile from source."
}

# ─── Build from source path ─────────────────────────────────────────────────

check_toolchain() {
    local missing=()

    if ! command -v go &>/dev/null; then
        missing+=("Go (https://go.dev/dl/)")
    fi
    if ! command -v cargo &>/dev/null; then
        missing+=("Rust (https://rustup.rs)")
    fi
    if [[ ${#missing[@]} -gt 0 ]]; then
        error "Missing toolchain: ${missing[*]}"
    fi
    info "All toolchain requirements met"
}

install_build() {
    check_toolchain

    local WARP_SRC="${WARP_SRC:-}"
    # Check --warp-src argument
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --warp-src) WARP_SRC="$2"; shift 2 ;;
            *) shift ;;
        esac
    done

    if [[ -z "$WARP_SRC" ]]; then
        # Try to find Warp source nearby
        for candidate in "$SCRIPT_DIR/../warp-v0.2026.04.29.08.56.stable_00-src/warp-0.2026.04.29.08.56.stable_00" \
                         "$HOME/warp" \
                         "$SCRIPT_DIR/../warp"; do
            if [[ -f "$candidate/Cargo.toml" ]]; then
                WARP_SRC="$candidate"
                break
            fi
        done
    fi

    if [[ -z "$WARP_SRC" ]]; then
        error "Warp source not found. Set WARP_SRC or use --warp-src DIR"
    fi
    info "Warp source: $WARP_SRC"

    # Apply patches if not already applied
    info "Applying patches to Warp source..."
    for patch in "$SCRIPT_DIR/patches/"*.patch; do
        if patch -p1 --dry-run -d "$WARP_SRC" < "$patch" &>/dev/null; then
            patch -p1 -d "$WARP_SRC" < "$patch"
            info "  Applied $(basename "$patch")"
        else
            warn "  Skipped $(basename "$patch") (already applied or conflicts)"
        fi
    done

    # Build
    export WARP_SRC
    info "Building WarpLocal.app..."
    "$SCRIPT_DIR/build_and_bundle.sh"

    # Install
    if [[ -d "$SCRIPT_DIR/WarpLocal.app" ]]; then
        cp -R "$SCRIPT_DIR/WarpLocal.app" "$INSTALL_DIR/WarpLocal.app"
        xattr -cr "$INSTALL_DIR/WarpLocal.app" 2>/dev/null || true
        info "Installed to $INSTALL_DIR/WarpLocal.app (quarantine cleared)"
    else
        error "Build failed — WarpLocal.app not found"
    fi
}

# ─── Diagnostics (doctor) ─────────────────────────────────────────────────────

run_doctor() {
    local diag_script="$SCRIPT_DIR/diagnostics.sh"
    if [[ -f "$diag_script" ]]; then
        bash "$diag_script"
    else
        # Fallback: download and run from GitHub
        info "diagnostics.sh not found locally, downloading..."
        bash <(curl -fsSL https://raw.githubusercontent.com/sasuke39/open-warp/main/diagnostics.sh)
    fi
}

# ─── Main ────────────────────────────────────────────────────────────────────

LAUNCH=false
BUILD=false
DOCTOR=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --build)    BUILD=true; shift ;;
        --launch)   LAUNCH=true; shift ;;
        --warp-src) shift 2 ;; # handled in install_build
        doctor)     DOCTOR=true; shift ;;
        *)          shift ;;
    esac
done

if $DOCTOR; then
    run_doctor
    exit 0
fi

info "WarpLocal Installer"
echo ""

if $BUILD; then
    install_build "$@"
else
    install_prebuilt
fi

echo ""
info "Installation complete!"
echo ""
echo "Next steps:"
echo "  1. Open WarpLocal from /Applications or Spotlight"
echo "  2. Open Settings -> Local Adapter to configure your LLM provider"
echo "  3. Start chatting with AI in Warp!"
echo ""

if $LAUNCH; then
    info "Launching WarpLocal..."
    open "$INSTALL_DIR/WarpLocal.app"
fi
