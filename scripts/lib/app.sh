#!/bin/bash
# Shared helpers for install.sh, uninstall.sh and app/scripts/dev-install.sh.
# Sourced, not executed. Uses $HOME at call time so tests can point it elsewhere.

APP_NAME="Personalization Setup"

app_dest() { printf '%s\n' "$HOME/Applications/$APP_NAME.app"; }

# Print PIDs whose command is exactly the installed binary (with or without args).
app_pids() {
  local bin pid cmd
  bin="$(app_dest)/Contents/MacOS/personalization-setup"
  ps -axo pid=,command= | while read -r pid cmd; do
    if [ "$cmd" = "$bin" ] || [ "${cmd#"$bin "}" != "$cmd" ]; then
      echo "$pid"
    fi
  done
}

# Ask the app to quit, wait up to PCS_QUIT_TIMEOUT seconds, then kill the exact
# PIDs. Returns 1 if the app is still running afterwards.
app_quit() {
  local timeout="${PCS_QUIT_TIMEOUT:-10}" i pid
  osascript -e "quit app \"$APP_NAME\"" >/dev/null 2>&1 || true
  i=0
  while [ "$i" -lt $((timeout * 5)) ]; do
    [ -z "$(app_pids)" ] && return 0
    sleep 0.2
    i=$((i + 1))
  done
  [ -z "$(app_pids)" ] && return 0
  echo "    still running; killing $(app_dest)/Contents/MacOS/personalization-setup" >&2
  for pid in $(app_pids); do kill "$pid" 2>/dev/null || true; done
  i=0
  while [ "$i" -lt 5 ]; do
    [ -z "$(app_pids)" ] && return 0
    sleep 0.2
    i=$((i + 1))
  done
  [ -z "$(app_pids)" ]
}

# Succeeds when the launcher the app writes points at this app's bundled pcs.
launcher_points_at_dest() {
  local f="$HOME/.pcs/bin/pcs-hook"
  [ -f "$f" ] && grep -qxF "BIN=\"$(app_dest)/Contents/MacOS/pcs\"" "$f"
}
