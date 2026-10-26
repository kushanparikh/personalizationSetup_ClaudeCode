#!/bin/bash
# Removes Personalization Setup: its hooks, saved answers, logs and the app.
# Usage: ./uninstall.sh [--yes]
# Exit codes: 0 done or cancelled; 1 failure (the app is kept if hooks couldn't be removed).
set -euo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SELF_DIR/scripts/lib/app.sh"

LOG_DIR="$HOME/Library/Logs/$APP_NAME"

YES=0
while [ $# -gt 0 ]; do
  case "$1" in
    --yes) YES=1; shift ;;
    *) echo "Usage: ./uninstall.sh [--yes]" >&2; exit 1 ;;
  esac
done

# Everything printed is also appended to install.log (with a timestamp per run).
# pcs deletes this folder later, so the log is gone after a full uninstall.
mkdir -p "$LOG_DIR"
printf '=== uninstall.sh %s ===\n' "$(date '+%Y-%m-%d %H:%M:%S')" >>"$LOG_DIR/install.log"
exec > >(tee -a "$LOG_DIR/install.log") 2>&1

# 1. Confirm (no TTY counts as No)
if [ "$YES" -eq 0 ]; then
  ans=""
  tty_dev="${PCS_TTY:-/dev/tty}"
  if ( : <"$tty_dev" ) 2>/dev/null; then
    printf "This removes Personalization Setup's hooks from ~/.claude/settings.json, then the app, its saved answers and its logs. Your other hooks and settings are kept. Continue? [y/N] "
    read -r ans <"$tty_dev" 2>/dev/null || ans=""
  fi
  case "$ans" in
    y|Y) ;;
    *) echo "Cancelled."; exit 0 ;;
  esac
fi

DEST="$(app_dest)"
PCS="$DEST/Contents/MacOS/pcs"

# 2. The app must be gone before its files are removed
app_quit || { echo "Couldn't quit Personalization Setup. Quit it, then run ./uninstall.sh again."; exit 1; }

# 3. The app is needed to remove its hooks; without it keep ~/.pcs (the hooks still call its launcher)
if [ ! -x "$PCS" ]; then
  echo "The app is already gone, so its hooks can't be removed automatically. They do nothing while the app is missing. To finish: reinstall with ./install.sh, then run ./uninstall.sh again."
  exit 1
fi

rc=0
out="$("$PCS" uninstall --yes 2>&1)" || rc=$?
# Parse the JSON with plutil (always on macOS; jq is not).
json_get() { printf '%s' "$out" | /usr/bin/plutil -extract "$1" raw -o - -- - 2>/dev/null || true; }
if [ "$rc" -ne 0 ]; then
  err="$(json_get error)"
  echo "Couldn't remove the hooks: ${err:-${out:-unknown error}}"
  echo "Nothing else was removed. Fix that, then run ./uninstall.sh again."
  exit 1
fi
n="$(json_get removedHooks)"
backup="$(json_get backup)"
echo "Removed ${n:-0} hook(s)."
[ -n "$backup" ] && echo "Backup of your settings: $backup"

rm -rf "$DEST"
# A hook may have relaunched the app since the first quit, and that copy
# rewrites ~/.pcs/bin/pcs-hook at start. pcs succeeded, so it had deleted
# ~/.pcs: stop the copy, then delete the launcher it wrote again.
app_quit "$DEST" "$(app_old_dest)" || echo "A copy of Personalization Setup is still running. Quit it from the menu bar."
if [ -e "$HOME/.pcs/bin/pcs-hook" ]; then
  rm -f "$HOME/.pcs/bin/pcs-hook"
  rmdir "$HOME/.pcs/bin" "$HOME/.pcs" 2>/dev/null || true
fi
if [ -f "$HOME/.claude/settings.json.pcs-original" ]; then
  echo "Uninstalled. Your original settings copy is still at ~/.claude/settings.json.pcs-original"
else
  echo "Uninstalled."
fi
