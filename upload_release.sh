#!/bin/bash
# WarpLocal Release Upload
#
# Zip the built app, auto-bump version, and upload to GitHub Releases.
#
# Usage:
#   ./upload_release.sh                — Auto bump patch version (v1.0.0 -> v1.0.1)
#   ./upload_release.sh --minor        — Bump minor version (v1.0.0 -> v1.1.0)
#   ./upload_release.sh --major        — Bump major version (v1.0.0 -> v2.0.0)
#   ./upload_release.sh --tag v1.2.3   — Use exact tag
#   ./upload_release.sh --draft        — Create as draft
#   ./upload_release.sh --prerelease   — Create as prerelease
#
# Prerequisites:
#   - WarpLocal.app built (by build_and_bundle.sh)
#   - gh CLI authenticated

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RELEASE_DIR="$SCRIPT_DIR/release"
APP_PATH="$SCRIPT_DIR/WarpLocal.app"
REPO="sasuke39/open-warp"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ─── Parse args ──

TAG=""
BUMP="patch"
DRAFT=""
PRERELEASE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --tag)        TAG="$2"; shift 2 ;;
        --minor)      BUMP="minor"; shift ;;
        --major)      BUMP="major"; shift ;;
        --draft)      DRAFT="--draft"; shift ;;
        --prerelease) PRERELEASE="--prerelease"; shift ;;
        *)            shift ;;
    esac
done

# ─── Validate ──

if [[ ! -d "$APP_PATH" ]]; then
    error "WarpLocal.app not found. Run build_and_bundle.sh first."
fi

if ! command -v gh &>/dev/null; then
    error "gh CLI not found. Install from https://cli.github.com"
fi

# ─── Determine tag ──

if [[ -z "$TAG" ]]; then
    # Get latest tag from remote
    LATEST_TAG="$(git -C "$SCRIPT_DIR" ls-remote --tags --sort=-v:refname origin 'v*' 2>/dev/null | head -1 | sed 's/.*refs\/tags\///' || echo "v0.0.0")"
    if [[ -z "$LATEST_TAG" ]]; then
        LATEST_TAG="v0.0.0"
    fi

    VERSION="${LATEST_TAG#v}"
    IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

    case "$BUMP" in
        major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
        minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
        patch) PATCH=$((PATCH + 1)) ;;
    esac

    TAG="v${MAJOR}.${MINOR}.${PATCH}"
fi

info "Latest remote tag: ${LATEST_TAG:-none}"
info "New tag: $TAG"
echo ""

# ─── Confirm ──

read -rp "Upload release $TAG? [Y/n] " confirm
if [[ "$(echo "$confirm" | tr '[:upper:]' '[:lower:]')" == "n" ]]; then
    info "Aborted."
    exit 0
fi

# ─── Write build-info.json into app bundle ──

BUILD_INFO="$APP_PATH/Contents/Resources/build-info.json"
VERSION="${TAG#v}"
GIT_COMMIT="$(git -C "$SCRIPT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)"
GIT_BRANCH="$(git -C "$SCRIPT_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
BUILD_ARCH="$(uname -m)"
GO_VERSION="$(go version 2>/dev/null | awk '{print $3}' || echo unknown)"

cat > "$BUILD_INFO" <<BUILDINFO
{
  "version": "$VERSION",
  "git_commit": "$GIT_COMMIT",
  "git_branch": "$GIT_BRANCH",
  "build_time": "$BUILD_TIME",
  "build_arch": "$BUILD_ARCH",
  "go_version": "$GO_VERSION"
}
BUILDINFO

info "build-info.json: version=$VERSION commit=$GIT_COMMIT arch=$BUILD_ARCH"

# ─── Zip ──

mkdir -p "$RELEASE_DIR"
ZIP_NAME="WarpLocal.app.zip"
ZIP_PATH="$RELEASE_DIR/$ZIP_NAME"

rm -f "$ZIP_PATH"

info "Zipping WarpLocal.app..."
cd "$SCRIPT_DIR"
ditto -c -k --keepParent "WarpLocal.app" "$ZIP_PATH"

ZIP_SIZE="$(du -h "$ZIP_PATH" | cut -f1 | tr -d ' ')"
info "Zipped: $ZIP_PATH ($ZIP_SIZE)"

# ─── SHA256 ──

SHA_PATH="$RELEASE_DIR/$ZIP_NAME.sha256"
SHASUM="$(shasum -a 256 "$ZIP_PATH" | awk '{print $1}')"
echo "$SHASUM  $ZIP_NAME" > "$SHA_PATH"

info "SHA256: $SHASUM"

# ─── Create git tag and push ──

info "Creating git tag $TAG..."
git -C "$SCRIPT_DIR" tag "$TAG"
git -C "$SCRIPT_DIR" push origin "$TAG"

# ─── Create release and upload ──

info "Creating GitHub release $TAG..."

RELEASE_ARGS=(--repo "$REPO" --title "WarpLocal ${TAG#v}")
[[ -n "$DRAFT" ]] && RELEASE_ARGS+=("$DRAFT")
[[ -n "$PRERELEASE" ]] && RELEASE_ARGS+=("$PRERELEASE")
RELEASE_ARGS+=(--notes "Production bundle for WarpLocal ${TAG#v}.")

gh release create "$TAG" "${RELEASE_ARGS[@]}"

info "Uploading $ZIP_NAME..."
gh release upload "$TAG" "$ZIP_PATH" "$SHA_PATH" --repo "$REPO"

echo ""
info "Done! Release $TAG published:"
info "  https://github.com/$REPO/releases/tag/$TAG"
