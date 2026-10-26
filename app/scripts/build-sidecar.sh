#!/usr/bin/env bash
# Builds the Go sidecar into src-tauri/binaries/pcs-<host triple>, the name
# Tauri's externalBin expects.
set -euo pipefail

command -v go >/dev/null 2>&1 || { echo "build-sidecar: 'go' not found on PATH" >&2; exit 1; }
command -v rustc >/dev/null 2>&1 || { echo "build-sidecar: 'rustc' not found on PATH" >&2; exit 1; }

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TRIPLE="$(rustc -vV | sed -n 's/^host: //p')"
[ -n "$TRIPLE" ] || { echo "build-sidecar: could not read host triple from rustc -vV" >&2; exit 1; }

mkdir -p "$APP_DIR/src-tauri/binaries"
cd "$APP_DIR/../sidecar"
if [ "${1:-}" = "--universal" ]; then
  BIN="$APP_DIR/src-tauri/binaries"
  command -v lipo >/dev/null 2>&1 || { echo "build-sidecar: 'lipo' not found on PATH" >&2; exit 1; }
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  for arch in arm64 amd64; do
    CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" go build -o "$tmp/pcs-$arch" ./cmd/pcs
  done
  lipo -create "$tmp/pcs-arm64" "$tmp/pcs-amd64" -output "$BIN/pcs-universal-apple-darwin"
  # Tauri's build script checks for the per-arch name during each arch's cargo
  # build, so provide those too (copies of the universal binary).
  cp "$BIN/pcs-universal-apple-darwin" "$BIN/pcs-aarch64-apple-darwin"
  cp "$BIN/pcs-universal-apple-darwin" "$BIN/pcs-x86_64-apple-darwin"
  echo "build-sidecar: built src-tauri/binaries/pcs-universal-apple-darwin"
  exit 0
fi
rm -f "$APP_DIR/src-tauri/binaries/pcs-$TRIPLE"  # may be a lipo copy; go refuses to overwrite non-object files
go build -o "$APP_DIR/src-tauri/binaries/pcs-$TRIPLE" ./cmd/pcs
echo "build-sidecar: built src-tauri/binaries/pcs-$TRIPLE"
