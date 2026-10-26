#!/bin/bash
# Usage: scripts/check-version.sh vX.Y.Z
# Exits 0 when X.Y.Z matches the version in tauri.conf.json, Cargo.toml and package.json.
set -euo pipefail

TAG="${1:-}"
case "$TAG" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "Usage: scripts/check-version.sh vX.Y.Z" >&2; exit 1 ;;
esac
WANT="${TAG#v}"
ROOT_DIR="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

json_version() { sed -n 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -n1; }
toml_version() { sed -n 's/^version[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$1" | head -n1; }

bad=0
check() { # relpath version
  if [ "$2" != "$WANT" ]; then
    echo "Version mismatch: $1 has '${2:-<none>}', expected '$WANT'" >&2
    bad=1
  fi
}
check app/src-tauri/tauri.conf.json "$(json_version "$ROOT_DIR/app/src-tauri/tauri.conf.json")"
check app/src-tauri/Cargo.toml "$(toml_version "$ROOT_DIR/app/src-tauri/Cargo.toml")"
check app/package.json "$(json_version "$ROOT_DIR/app/package.json")"
exit "$bad"
