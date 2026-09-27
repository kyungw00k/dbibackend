#!/bin/bash
# build-macos-app.sh — assemble dbibackend.app (universal) and a drag-install DMG.
#
# Usage: build-macos-app.sh [version]
#
# Optional environment:
#   MACOS_SIGNING_ID   Apple "Developer ID Application: ..." identity.
#                      When set, the app is signed with it and notarized
#                      (requires App Store Connect key env vars below).
#                      Otherwise the app is ad-hoc signed.
#   NOTARY_KEY_PATH / NOTARY_KEY_ID / NOTARY_ISSUER_ID   notarytool credentials
set -euo pipefail

echo "==> Building universal binary (v${VERSION#v})"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

DIST="$ROOT/dist"
APP="$DIST/dbibackend.app"
DMG="$DIST/dbibackend_${VERSION}_macos_universal.dmg"
LDFLAGS="-s -w -X github.com/kyungw00k/dbibackend/cmd.Version=${VERSION}"

echo "==> Building universal binary (v${VERSION})"
rm -rf "$APP" "$DMG"
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o "$DIST/dbibackend.arm64" ./cmd/dbibackend
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "$DIST/dbibackend.amd64" ./cmd/dbibackend
lipo -create -output "$DIST/dbibackend.universal" "$DIST/dbibackend.arm64" "$DIST/dbibackend.amd64"
rm -f "$DIST/dbibackend.arm64" "$DIST/dbibackend.amd64"

echo "==> Assembling dbibackend.app"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$DIST/dbibackend.universal" "$APP/Contents/MacOS/dbibackend"
chmod 755 "$APP/Contents/MacOS/dbibackend"

echo "==> Generating AppIcon.icns"
ICONSET="$DIST/AppIcon.iconset"
rm -rf "$ICONSET"; mkdir -p "$ICONSET"
for size in 16 32 128 256 512; do
    sips -z "$size" "$size" assets/appicon.png --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
    sips -z $((size * 2)) $((size * 2)) assets/appicon.png --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns"
rm -rf "$ICONSET"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>en</string>
    <key>CFBundleDisplayName</key>
    <string>DBI Backend</string>
    <key>CFBundleExecutable</key>
    <string>dbibackend</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundleIdentifier</key>
    <string>io.github.kyungw00k.dbibackend</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>dbibackend</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION#v}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION#v}</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
PLIST

echo "==> Code signing"
if [ -n "${MACOS_SIGNING_ID:-}" ]; then
    codesign --force --options runtime --timestamp -s "$MACOS_SIGNING_ID" "$APP"
    if [ -n "${NOTARY_KEY_PATH:-}" ]; then
        echo "==> Notarizing"
        xcrun notarytool submit "$APP" --wait \
            --key "$NOTARY_KEY_PATH" --key-id "$NOTARY_KEY_ID" --issuer "$NOTARY_ISSUER_ID" || true
        xcrun stapler staple "$APP" || true
    fi
else
    codesign --force -s - "$APP"
fi

echo "==> Creating DMG"
STAGING="$DIST/dmg-staging"
rm -rf "$STAGING"; mkdir -p "$STAGING"
cp -R "$APP" "$STAGING/"
ln -s /Applications "$STAGING/Applications"
hdiutil create -volname "DBI Backend" -srcfolder "$STAGING" \
    -format UDZO -ov "$DMG" >/dev/null
rm -rf "$STAGING"

echo ""
echo "✓ $APP"
echo "✓ $DMG ($(du -h "$DMG" | cut -f1))"
