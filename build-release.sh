#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="dns-updater"
VERSION="$(date -u +%Y%m%d.%H%M).$(git rev-parse --short HEAD)"
LDFLAGS="-X main.Version=${VERSION}"
OUT_DIR="dist/bin"

echo "==> Building ${BINARY_NAME} v${VERSION} -> ${OUT_DIR}/"

# Build all targets via build.sh
./build.sh

echo "==> All binaries built successfully"

# Generate SHA256 checksums
echo "==> Generating checksums..."

# Generate checksums using release display names
hash_file() {
    if command -v sha256sum &>/dev/null; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

rm -f "${OUT_DIR}/checksums.txt"
for bin in "${OUT_DIR}/${BINARY_NAME}"-*; do
    name="$(basename "$bin")"
    echo "$(hash_file "$bin")  ${name}" >> "${OUT_DIR}/checksums.txt"
done

echo "==> Checksums written to ${OUT_DIR}/checksums.txt"

# Check if gh CLI is available and authenticated
if ! command -v gh &>/dev/null; then
    echo "==> gh CLI not found, skipping release upload"
    exit 0
fi

if ! gh auth status &>/dev/null; then
    echo "==> gh CLI not authenticated, skipping release upload"
    exit 0
fi

echo "==> Creating GitHub release v${VERSION}"

# Generate release notes from recent commits since last tag (or all if no tags)
LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
if [ -n "${LAST_TAG}" ]; then
    NOTES=$(git log --pretty=format:"- %s" "${LAST_TAG}..HEAD")
else
    NOTES=$(git log --pretty=format:"- %s" -10)
fi

ASSETS=()
for bin in "${OUT_DIR}/${BINARY_NAME}"-*; do
    ASSETS+=("$bin")
done
ASSETS+=("${OUT_DIR}/checksums.txt#checksums.txt")

gh release create "v${VERSION}" \
    --title "v${VERSION}" \
    --notes "${NOTES}" \
    "${ASSETS[@]}"

echo "==> Release v${VERSION} published"
