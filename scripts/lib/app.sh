#!/bin/bash
# Shared helpers for install.sh, uninstall.sh and app/scripts/dev-install.sh.
# Sourced, not executed. Uses $HOME at call time so tests can point it elsewhere.

APP_NAME="Personalization Setup"

app_dest() { printf '%s\n' "$HOME/Applications/$APP_NAME.app"; }
# Staging and set-aside paths used while an upgrade swaps bundles (same volume,
# so both moves are renames).
app_new_dest() { printf '%s\n' "$HOME/Applications/.$APP_NAME.app.new"; }
app_old_dest() { printf '%s\n' "$HOME/Applications/.$APP_NAME.app.old"; }

# app_pids [bundle...]: print PIDs whose command is exactly a bundle's binary
# (with or without args). Default bundle: the installed one.
app_pids() {
  local b bin pid cmd
  [ $# -gt 0 ] || set -- "$(app_dest)"
  ps -axo pid=,command= | while read -r pid cmd; do
    for b in "$@"; do
      bin="$b/Contents/MacOS/personalization-setup"
      if [ "$cmd" = "$bin" ] || [ "${cmd#"$bin "}" != "$cmd" ]; then
        echo "$pid"; break
      fi
    done
  done
}

# app_quit [bundle...]: ask the app to quit, wait up to PCS_QUIT_TIMEOUT
# seconds, then kill the exact PIDs of those bundles' binaries (default: the
# installed one). Returns 1 if any is still running afterwards.
app_quit() {
  local timeout="${PCS_QUIT_TIMEOUT:-10}" i pid
  [ $# -gt 0 ] || set -- "$(app_dest)"
  osascript -e "quit app \"$APP_NAME\"" >/dev/null 2>&1 || true
  i=0
  while [ "$i" -lt $((timeout * 5)) ]; do
    [ -z "$(app_pids "$@")" ] && return 0
    sleep 0.2
    i=$((i + 1))
  done
  [ -z "$(app_pids "$@")" ] && return 0
  echo "    still running; killing PID(s) $(app_pids "$@" | tr '\n' ' ')" >&2
  for pid in $(app_pids "$@"); do kill "$pid" 2>/dev/null || true; done
  i=0
  while [ "$i" -lt 5 ]; do
    [ -z "$(app_pids "$@")" ] && return 0
    sleep 0.2
    i=$((i + 1))
  done
  [ -z "$(app_pids "$@")" ]
}

# Succeeds when the launcher the app writes points at this app's bundled pcs.
launcher_points_at_dest() {
  local f="$HOME/.pcs/bin/pcs-hook"
  [ -f "$f" ] && grep -qxF "BIN=\"$(app_dest)/Contents/MacOS/pcs\"" "$f"
}
