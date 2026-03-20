#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="dns-updater"
VERSION="$(date -u +%Y%m%d.%H%M).$(git rev-parse --short HEAD)"
LDFLAGS="-X main.Version=${VERSION}"
OUT_DIR="dist/bin"

# All supported targets: name|GOOS|GOARCH|GOARM|suffix
TARGETS=(
    "linux-amd64|linux|amd64||"
    "linux-armv7l|linux|arm|7|"
    "darwin-amd64|darwin|amd64||"
    "darwin-arm64|darwin|arm64||"
    "windows-amd64|windows|amd64||.exe"
)

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Build ${BINARY_NAME} binaries for one or all supported targets.

Options:
  --target=TARGET   Build only the specified target (see list below)
  --list            List all available targets
  --help, -h        Show this help message

Available targets:
  linux-amd64       Linux x86_64
  linux-armv7l      Linux ARMv7 (32-bit)
  darwin-amd64      macOS Intel
  darwin-arm64      macOS Apple Silicon
  windows-amd64     Windows x86_64

Examples:
  $(basename "$0")                        # Build all targets
  $(basename "$0") --target=linux-amd64   # Build only Linux x86_64
  $(basename "$0") --target=darwin-arm64  # Build only macOS Apple Silicon
EOF
    exit 0
}

list_targets() {
    echo "Available targets:"
    for entry in "${TARGETS[@]}"; do
        IFS='|' read -r name goos goarch goarm suffix <<< "$entry"
        printf "  %-20s %s/%s" "$name" "$goos" "$goarch"
        [[ -n "$goarm" ]] && printf " (GOARM=%s)" "$goarm"
        echo
    done
    exit 0
}

build_target() {
    local name="$1" goos="$2" goarch="$3" goarm="$4" suffix="$5"
    local output="${OUT_DIR}/${BINARY_NAME}-${name}${suffix}"

    printf "  %-24s" "${goos}/${goarch}${goarm:+/v${goarm}}..."
    export GOOS="$goos" GOARCH="$goarch"
    if [[ -n "$goarm" ]]; then
        export GOARM="$goarm"
    else
        unset GOARM 2>/dev/null || true
    fi
    go build -ldflags "${LDFLAGS}" -o "$output" .
    echo "ok"
}

find_target() {
    local want="$1"
    for entry in "${TARGETS[@]}"; do
        IFS='|' read -r name goos goarch goarm suffix <<< "$entry"
        if [[ "$name" == "$want" ]]; then
            echo "$entry"
            return 0
        fi
    done
    return 1
}

# Parse arguments
SELECTED_TARGET=""
for arg in "$@"; do
    case "$arg" in
        --help|-h)  usage ;;
        --list)     list_targets ;;
        --target=*) SELECTED_TARGET="${arg#--target=}" ;;
        *)
            echo "Error: unknown argument '${arg}'"
            echo "Run '$(basename "$0") --help' for usage."
            exit 1
            ;;
    esac
done

mkdir -p "$OUT_DIR"
echo "==> Building ${BINARY_NAME} v${VERSION} -> ${OUT_DIR}/"

if [[ -n "$SELECTED_TARGET" ]]; then
    entry=$(find_target "$SELECTED_TARGET") || {
        echo "Error: unknown target '${SELECTED_TARGET}'"
        echo
        list_targets
        exit 1
    }
    IFS='|' read -r name goos goarch goarm suffix <<< "$entry"
    build_target "$name" "$goos" "$goarch" "$goarm" "$suffix"
else
    for entry in "${TARGETS[@]}"; do
        IFS='|' read -r name goos goarch goarm suffix <<< "$entry"
        build_target "$name" "$goos" "$goarch" "$goarm" "$suffix"
    done
fi

echo "==> Done"
