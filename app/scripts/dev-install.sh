#!/usr/bin/env bash
# Build the release bundle and install it to ~/Applications, replacing any copy.
# WARNING: this installs the REAL app, which manages ~/.claude/settings.json hooks.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ -f "$HOME/.cargo/env" ]; then
  # shellcheck disable=SC1091
  source "$HOME/.cargo/env"
fi

APP_NAME="Personalization Setup"
BUILT="$APP_DIR/src-tauri/target/release/bundle/macos/$APP_NAME.app"
DEST_DIR="$HOME/Applications"
DEST="$DEST_DIR/$APP_NAME.app"
DEST_BIN="$DEST/Contents/MacOS/personalization-setup"
LOG_DIR="$HOME/Library/Logs/$APP_NAME"

echo "==> Building release bundle"
(cd "$APP_DIR" && npm run tauri build -- --bundles app)
if [ ! -d "$BUILT" ]; then
  echo "ERROR: bundle was not produced at: $BUILT" >&2
  exit 1
fi

is_running() { pgrep -xf "$DEST_BIN" >/dev/null 2>&1; }

echo "==> Quitting any running copy"
osascript -e 'quit app "Personalization Setup"' >/dev/null 2>&1 || true
for _ in $(seq 1 25); do
  is_running || break
  sleep 0.2
done
if is_running; then
  echo "    still running; killing $DEST_BIN"
  pkill -xf "$DEST_BIN" || true
  sleep 1
fi

echo "==> Installing to $DEST"
mkdir -p "$DEST_DIR"
if [ -e "$DEST" ]; then
  rm -rf "$HOME/Applications/Personalization Setup.app"
fi
ditto "$BUILT" "$DEST"

echo "==> Opening"
open "$DEST"

echo "Done."
echo "  Installed: $DEST"
echo "  Logs:      $LOG_DIR/app.log"
echo "             $LOG_DIR/pcs.log"
