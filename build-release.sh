#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="dns-updater"
VERSION="$(date -u +%Y%m%d.%H%M).$(git rev-parse --short HEAD)"
LDFLAGS="-X main.Version=${VERSION}"

echo "==> Building ${BINARY_NAME} v${VERSION}"

# Build all platform binaries
echo "  Building windows/amd64..."
GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o "${BINARY_NAME}.exe" .

echo "  Building linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o "${BINARY_NAME}-linux" .

echo "  Building darwin/amd64..."
GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o "${BINARY_NAME}-darwin" .

echo "  Building darwin/arm64..."
GOOS=darwin GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o "${BINARY_NAME}-darwin-arm64" .

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

rm -f checksums.txt
echo "$(hash_file "${BINARY_NAME}.exe")  ${BINARY_NAME}-windows-amd64.exe" >> checksums.txt
echo "$(hash_file "${BINARY_NAME}-linux")  ${BINARY_NAME}-linux-amd64" >> checksums.txt
echo "$(hash_file "${BINARY_NAME}-darwin")  ${BINARY_NAME}-darwin-amd64" >> checksums.txt
echo "$(hash_file "${BINARY_NAME}-darwin-arm64")  ${BINARY_NAME}-darwin-arm64" >> checksums.txt

echo "==> Checksums written to checksums.txt"

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

gh release create "v${VERSION}" \
    --title "v${VERSION}" \
    --notes "${NOTES}" \
    "${BINARY_NAME}.exe#${BINARY_NAME}-windows-amd64.exe" \
    "${BINARY_NAME}-linux#${BINARY_NAME}-linux-amd64" \
    "${BINARY_NAME}-darwin#${BINARY_NAME}-darwin-amd64" \
    "${BINARY_NAME}-darwin-arm64#${BINARY_NAME}-darwin-arm64" \
    "checksums.txt#checksums.txt"

echo "==> Release v${VERSION} published"
