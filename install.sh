#!/bin/bash
# Installs (or upgrades) Personalization Setup into ~/Applications.
# Usage: ./install.sh [--zip <file>] [--version vX.Y.Z] [--install-claude]
# Exit codes: 0 installed and launched; 1 prerequisite/download/verify failure
# (nothing installed); 2 installed but the launch check failed.
set -euo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SELF_DIR/scripts/lib/app.sh"

PCS_REPO="${PCS_REPO:-kushanparikh/personalizationSetup_ClaudeCode}"
ASSET="Personalization-Setup-macos.zip"
LOG_DIR="$HOME/Library/Logs/$APP_NAME"

ZIP_ARG=""; VERSION=""; INSTALL_CLAUDE=0
while [ $# -gt 0 ]; do
  case "$1" in
    --zip) [ $# -ge 2 ] || { echo "--zip needs a file." >&2; exit 1; }; ZIP_ARG="$2"; shift 2 ;;
    --version) [ $# -ge 2 ] || { echo "--version needs a value like v1.2.3." >&2; exit 1; }; VERSION="$2"; shift 2 ;;
    --install-claude) INSTALL_CLAUDE=1; shift ;;
    *) echo "Usage: ./install.sh [--zip <file>] [--version vX.Y.Z] [--install-claude]" >&2; exit 1 ;;
  esac
done

# Everything printed is also appended to install.log (with a timestamp per run).
mkdir -p "$LOG_DIR"
printf '=== install.sh %s ===\n' "$(date '+%Y-%m-%d %H:%M:%S')" >>"$LOG_DIR/install.log"
exec > >(tee -a "$LOG_DIR/install.log") 2>&1

TMP="$(mktemp -d "${TMPDIR:-/tmp}/pcs-install.XXXXXX")"
DEST="$(app_dest)"; NEW="$(app_new_dest)"; OLD="$(app_old_dest)"
# On any exit: put the old bundle back if the swap stopped half-way, then
# remove the staging and set-aside copies.
cleanup() {
  if [ ! -e "$DEST" ] && [ -d "$OLD" ]; then mv "$OLD" "$DEST" 2>/dev/null || true; fi
  rm -rf "$TMP" "$NEW" "$OLD"
}
trap cleanup EXIT

# 1. macOS 13 or later
major="$(sw_vers -productVersion | cut -d. -f1)"
if [ "${major:-0}" -lt 13 ] 2>/dev/null; then
  echo "Personalization Setup needs macOS 13 or later."
  exit 1
fi

# 2. Claude Code
claude_present() { command -v claude >/dev/null 2>&1 || [ -x "$HOME/.local/bin/claude" ]; }
claude_works() { claude --version >/dev/null 2>&1 || "$HOME/.local/bin/claude" --version >/dev/null 2>&1; }
if ! claude_present; then
  want=0
  if [ "$INSTALL_CLAUDE" -eq 1 ]; then
    want=1
  else
    tty_dev="${PCS_TTY:-/dev/tty}"
    if ( : <"$tty_dev" ) 2>/dev/null; then
      printf 'Claude Code is required. Install it now? [y/N] '
      ans=""
      read -r ans <"$tty_dev" 2>/dev/null || ans=""
      case "$ans" in y|Y) want=1 ;; esac
    fi
  fi
  if [ "$want" -eq 0 ]; then
    echo "Claude Code is required. Install it (https://code.claude.com/docs/en/setup), then re-run ./install.sh."
    exit 1
  fi
  curl -fsSL https://claude.ai/install.sh | bash || true
  if ! claude_works; then
    echo "Claude Code didn't install correctly. See https://code.claude.com/docs/en/setup"
    exit 1
  fi
fi

# 3. Get the zip
ZIP="$TMP/$ASSET"
got=0
if [ -n "$ZIP_ARG" ]; then
  if [ ! -f "$ZIP_ARG" ]; then
    echo "Couldn't find $ZIP_ARG."
    exit 1
  fi
  if [ ! -f "$ZIP_ARG.sha256" ]; then
    echo "Missing $(basename "$ZIP_ARG").sha256 next to the zip. Download both files from the Releases page."
    exit 1
  fi
  cp "$ZIP_ARG" "$ZIP"
  cp "$ZIP_ARG.sha256" "$ZIP.sha256"
  got=1
else
  if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    if gh release download ${VERSION:+"$VERSION"} --repo "$PCS_REPO" --pattern "$ASSET*" --dir "$TMP" \
       && [ -f "$ZIP" ]; then
      got=1
    fi
  fi
  if [ "$got" -eq 0 ]; then
    if [ -n "$VERSION" ]; then
      base="https://github.com/$PCS_REPO/releases/download/$VERSION"
    else
      base="https://github.com/$PCS_REPO/releases/latest/download"
    fi
    if curl -fsSL -o "$ZIP" "$base/$ASSET" && curl -fsSL -o "$ZIP.sha256" "$base/$ASSET.sha256"; then
      got=1
    fi
  fi
  if [ "$got" -eq 0 ]; then
    echo "Couldn't download the app. The repository may be private: install GitHub CLI (brew install gh), run 'gh auth login', and re-run. Or download Personalization-Setup-macos.zip from the Releases page and run ./install.sh --zip <file>."
    exit 1
  fi
fi

# 4. Verify (a .sha256 is always required). Compare only the hash, so a zip the
# browser renamed ("... (1).zip") still verifies.
want_sum="$(awk 'NR == 1 { print tolower($1) }' "$ZIP.sha256" 2>/dev/null || true)"
have_sum="$(shasum -a 256 "$ZIP" | awk '{ print $1 }')"
if [ -z "$want_sum" ] || [ "$want_sum" != "$have_sum" ]; then
  echo "The download is corrupt (checksum mismatch). Nothing was installed."
  exit 1
fi
# The checksum passed, so drop the browser's quarantine (D41): --noqtn on the
# extract keeps it off every file (a plain ditto copy would carry it over).
mkdir -p "$TMP/x"
if ! ditto --noqtn -x -k "$ZIP" "$TMP/x" 2>/dev/null || [ ! -e "$TMP/x/$APP_NAME.app/Contents/MacOS/pcs" ]; then
  echo "The download doesn't contain the app. Nothing was installed."
  exit 1
fi

# 5. One-time settings backup, only while the settings are still free of the
# app's hooks (otherwise restoring it would bring back hooks that call a
# launcher uninstall has removed).
SETTINGS="$HOME/.claude/settings.json"
if [ -f "$SETTINGS" ] && [ ! -e "$SETTINGS.pcs-original" ]; then
  if grep -qF "/.pcs/bin/pcs-hook" "$SETTINGS" 2>/dev/null || [ -e "$HOME/.pcs" ]; then
    echo "Skipped saving an original settings copy: Personalization Setup's hooks are already in your settings."
  else
    cp -p "$SETTINGS" "$SETTINGS.pcs-original"
    echo "Saved a copy of your current Claude Code settings: ~/.claude/settings.json.pcs-original"
  fi
fi

# 6. Install. Stage the new bundle next to the old one, then swap with two
# renames, so a hook that relaunches the app never sees a half-copied bundle.
quit_failed() { echo "Couldn't quit the running app. Quit it and re-run."; exit 1; }
mkdir -p "$HOME/Applications"
rm -rf "$NEW" "$OLD"
ditto --noqtn "$TMP/x/$APP_NAME.app" "$NEW"
app_quit || quit_failed
[ -e "$DEST" ] && mv "$DEST" "$OLD"
mv "$NEW" "$DEST"
# A hook may have relaunched the old app between the quit and the swap; its
# process shows the installed path (or the set-aside path). Quit it too.
app_quit "$DEST" "$OLD" || quit_failed
rm -rf "$OLD"

# 7. Launch
open "$DEST" || true
timeout="${PCS_LAUNCH_TIMEOUT:-20}"
settle="${PCS_LAUNCH_SETTLE:-2}"
ok=0
i=0
while [ "$i" -lt $((timeout * 5)) ]; do
  if [ -n "$(app_pids)" ] && launcher_points_at_dest; then
    sleep "$settle"
    if [ -n "$(app_pids)" ]; then ok=1; fi
    break
  fi
  sleep 0.2
  i=$((i + 1))
done
if [ "$ok" -eq 1 ]; then
  echo 'Installed. Personalization Setup is open: click "+ Add hook" to start.'
  exit 0
fi
if xattr -p com.apple.quarantine "$DEST" >/dev/null 2>&1; then
  echo 'macOS blocked the app. Open System Settings → Privacy & Security, click "Open Anyway" for Personalization Setup, then run the same ./install.sh command again.'
else
  echo "The app didn't start. Open it from ~/Applications to see the error, and check the log: ~/Library/Logs/Personalization Setup/app.log"
fi
exit 2
