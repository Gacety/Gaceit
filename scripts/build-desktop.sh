#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
APP='build/408研习.app'
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources/Licenses"
cp internal/nativeapp/assets/phosphor/LICENSE "$APP/Contents/Resources/Licenses/Phosphor-Icons-MIT.txt"
go run ./scripts/mygo-tablet-patch
go build -trimpath -ldflags='-s -w' -o "$APP/Contents/MacOS/study408" .
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleExecutable</key><string>study408</string>
<key>CFBundleIdentifier</key><string>local.gacety.study408</string>
<key>CFBundleName</key><string>408研习</string>
<key>CFBundleDisplayName</key><string>408 研习</string>
<key>CFBundleVersion</key><string>1</string>
<key>CFBundleShortVersionString</key><string>0.2.0</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>NSHighResolutionCapable</key><true/>
<key>LSMinimumSystemVersion</key><string>12.0</string>
</dict></plist>
PLIST
xattr -cr "$APP"
codesign --force --sign - "$APP" || { xattr -cr "$APP"; codesign --force --sign - "$APP"; }
codesign --verify "$APP"
printf '\nBuilt %s\n' "$APP"
