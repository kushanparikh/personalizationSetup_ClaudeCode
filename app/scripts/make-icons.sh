#!/bin/bash
# Renders the SVG sources in src-tauri/icons/source and regenerates the app
# icon set and the menu-bar template glyph. macOS built-ins only.
set -euo pipefail
APP="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$APP/src-tauri/icons/source"
OUT="$APP/src-tauri/icons"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

cat > "$TMP/render.swift" <<'SWIFT'
import AppKit
let a = CommandLine.arguments
guard a.count == 5, let w = Int(a[3]), let h = Int(a[4]),
      let img = NSImage(contentsOf: URL(fileURLWithPath: a[1])),
      let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: w, pixelsHigh: h,
          bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
          colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0),
      let ctx = NSGraphicsContext(bitmapImageRep: rep) else {
    FileHandle.standardError.write("usage: render <svg> <png> <w> <h> (or load failed)\n".data(using: .utf8)!)
    exit(1)
}
rep.size = NSSize(width: w, height: h)
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = ctx
ctx.imageInterpolation = .high
img.draw(in: NSRect(x: 0, y: 0, width: w, height: h), from: .zero, operation: .copy, fraction: 1)
NSGraphicsContext.restoreGraphicsState()
guard let png = rep.representation(using: .png, properties: [:]) else { exit(1) }
try png.write(to: URL(fileURLWithPath: a[2]))
SWIFT

render() { # svg png w h
  swift "$TMP/render.swift" "$1" "$2" "$3" "$4"
  local pw ph
  pw=$(sips -g pixelWidth "$2" | awk '/pixelWidth/{print $2}')
  ph=$(sips -g pixelHeight "$2" | awk '/pixelHeight/{print $2}')
  if [ "$pw" != "$3" ] || [ "$ph" != "$4" ]; then
    echo "make-icons: $2 is ${pw}x${ph}, expected $3x$4" >&2; exit 1
  fi
}

render "$SRC/app-icon.svg" "$TMP/app-icon.png" 1024 1024
(cd "$APP" && npx tauri icon "$TMP/app-icon.png")
# Drop outputs the macOS bundle does not reference.
rm -rf "$OUT/android" "$OUT/ios" "$OUT"/Square*Logo.png "$OUT/StoreLogo.png" "$OUT/64x64.png"
render "$SRC/tray-glyph.svg" "$OUT/tray-template.png" 40 36
echo "make-icons: done"
