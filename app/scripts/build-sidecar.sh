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
go build -o "$APP_DIR/src-tauri/binaries/pcs-$TRIPLE" ./cmd/pcs
echo "build-sidecar: built src-tauri/binaries/pcs-$TRIPLE"
