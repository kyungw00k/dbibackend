#!/bin/bash
# prepare-libusb.sh — download/build libusb binaries for embedding
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/../internal/protocol/lib"

mkdir -p "$LIB_DIR"

# ── macOS universal dylib ──
if [ ! -f "$LIB_DIR/darwin_universal.dylib" ]; then
    echo "Preparing macOS universal dylib..."
    brew install libusb 2>/dev/null || true
    ARM64_DYLIB="/opt/homebrew/lib/libusb-1.0.dylib"

    if [ ! -f "$ARM64_DYLIB" ]; then
        echo "ERROR: arm64 dylib not found at $ARM64_DYLIB"
        exit 1
    fi

    # Detect installed version
    INSTALLED_VERSION=$(brew info libusb --json | python3 -c "import sys,json; print(json.load(sys.stdin)[0]['installed'][0]['version'])" 2>/dev/null || echo "1.0.27")
    echo "Homebrew libusb version: ${INSTALLED_VERSION}"

    TMP=$(mktemp -d)
    function cleanup() { rm -rf "$TMP"; }
    trap cleanup EXIT

    echo "Building amd64 slice from source (v${INSTALLED_VERSION})..."
    curl -sL "https://github.com/libusb/libusb/releases/download/v${INSTALLED_VERSION}/libusb-${INSTALLED_VERSION}.tar.bz2" \
        -o "$TMP/libusb.tar.bz2"
    tar xjf "$TMP/libusb.tar.bz2" -C "$TMP"

    cd "$TMP/libusb-${INSTALLED_VERSION}"
    ./configure --host=x86_64-apple-darwin \
        CC="cc -arch x86_64" \
        --disable-static \
        --enable-shared \
        --prefix="$TMP/install"
    make -j"$(sysctl -n hw.ncpu)"
    make install

    echo "Creating universal dylib..."
    lipo -create "$ARM64_DYLIB" "$TMP/install/lib/libusb-1.0.dylib" \
        -output "$LIB_DIR/darwin_universal.dylib"
    echo "✓ darwin_universal.dylib ($(du -h "$LIB_DIR/darwin_universal.dylib" | cut -f1))"
else
    echo "✓ darwin_universal.dylib (already exists)"
fi

# ── Windows DLLs ──
NEED_AMD64=false
NEED_ARM64=false
[ ! -f "$LIB_DIR/windows_amd64.dll" ] && NEED_AMD64=true
[ ! -f "$LIB_DIR/windows_arm64.dll" ] && NEED_ARM64=true

if $NEED_AMD64 || $NEED_ARM64; then
    echo "Downloading Windows DLLs from libusb v1.0.30..."
    TMP=$(mktemp -d)
    function cleanup() { rm -rf "$TMP"; }
    trap cleanup EXIT

    curl -sL "https://github.com/libusb/libusb/releases/download/v1.0.30/libusb-1.0.30.7z" \
        -o "$TMP/libusb.7z"

    # Prefer 7zz (newer, supports ARM64 BCJ), fall back to 7z
    EXTRACT="7zz"
    if ! command -v 7zz &>/dev/null; then
        EXTRACT="7z"
    fi

    if $NEED_AMD64; then
        echo "Extracting windows_amd64.dll (MinGW64)..."
        $EXTRACT x "$TMP/libusb.7z" -o"$TMP/out_amd64" "MinGW64/dll/libusb-1.0.dll" -y >/dev/null
        cp "$TMP/out_amd64/MinGW64/dll/libusb-1.0.dll" "$LIB_DIR/windows_amd64.dll"
        echo "✓ windows_amd64.dll ($(du -h "$LIB_DIR/windows_amd64.dll" | cut -f1))"
    fi
    if $NEED_ARM64; then
        echo "Extracting windows_arm64.dll (VS2025/ARM64)..."
        $EXTRACT x "$TMP/libusb.7z" -o"$TMP/out_arm64" "VS2025/ARM64/dll/libusb-1.0.dll" -y >/dev/null
        cp "$TMP/out_arm64/VS2025/ARM64/dll/libusb-1.0.dll" "$LIB_DIR/windows_arm64.dll"
        echo "✓ windows_arm64.dll ($(du -h "$LIB_DIR/windows_arm64.dll" | cut -f1))"
    fi
else
    echo "✓ windows_amd64.dll (already exists)"
    echo "✓ windows_arm64.dll (already exists)"
fi

echo ""
echo "All libusb binaries prepared in $LIB_DIR"
ls -lh "$LIB_DIR"
