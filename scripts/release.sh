#!/bin/bash
# Usage: scripts/release.sh vX.Y.Z [--dry-run]
# Builds the universal app, zips it with a checksum, and (unless --dry-run)
# publishes a GitHub release.
set -euo pipefail

USAGE="Usage: scripts/release.sh vX.Y.Z [--dry-run]"
TAG="${1:-}"
DRY_RUN=0
if [ "$#" -gt 2 ]; then echo "$USAGE" >&2; exit 1; fi
if [ "$#" -eq 2 ]; then
  [ "$2" = "--dry-run" ] || { echo "$USAGE" >&2; exit 1; }
  DRY_RUN=1
fi
case "$TAG" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "$USAGE" >&2; exit 1 ;;
esac
if ! printf '%s' "$TAG" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "$USAGE" >&2; exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
PCS_REPO="${PCS_REPO:-kushanparikh/personalizationSetup_ClaudeCode}"

if [ -n "$(git status --porcelain)" ]; then
  echo "Working tree has uncommitted changes." >&2
  exit 1
fi

"$ROOT_DIR/scripts/check-version.sh" "$TAG"

command -v rustup >/dev/null 2>&1 || { echo "rustup not found on PATH; install Rust from https://rustup.rs" >&2; exit 1; }
installed="$(rustup target list --installed)"
for t in aarch64-apple-darwin x86_64-apple-darwin; do
  if ! printf '%s\n' "$installed" | grep -qx "$t"; then
    echo "Run: rustup target add x86_64-apple-darwin aarch64-apple-darwin" >&2
    exit 1
  fi
done

echo "==> Running test suites"
(cd sidecar && go test ./...)
(cd app && npm test && npm run check)
(cd app/src-tauri && cargo test)
"$ROOT_DIR/scripts/test/run.sh"

echo "==> Building universal sidecar and app"
bash app/scripts/build-sidecar.sh --universal
(cd app && npx tauri build --target universal-apple-darwin --bundles app)

BUNDLE="app/src-tauri/target/universal-apple-darwin/release/bundle/macos/Personalization Setup.app"
[ -d "$BUNDLE" ] || { echo "Bundle not found: $BUNDLE" >&2; exit 1; }

OUT="dist/release"
ZIP="$OUT/Personalization-Setup-macos.zip"
SHA="$ZIP.sha256"
mkdir -p "$OUT"
rm -f "$ZIP" "$SHA"
ditto -c -k --keepParent "$BUNDLE" "$ZIP"
(cd "$OUT" && shasum -a 256 "Personalization-Setup-macos.zip" >"Personalization-Setup-macos.zip.sha256")
echo "==> Wrote $ZIP and $SHA"

if [ "$DRY_RUN" -eq 1 ]; then
  echo "Dry run: not publishing $TAG."
  exit 0
fi
if [ -z "$(git branch -r --contains HEAD 2>/dev/null)" ]; then
  echo "Push this commit first: git push" >&2
  exit 1
fi
gh release create "$TAG" "$ZIP" "$SHA" --repo "$PCS_REPO" \
  --target "$(git rev-parse HEAD)" \
  --title "Personalization Setup $TAG" --generate-notes
