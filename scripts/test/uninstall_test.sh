#!/bin/bash
# Tests for uninstall.sh (sourced by run.sh). Never touches the real ~.

. "$(dirname "${BASH_SOURCE[0]}")/fixtures.sh"
. "$REPO_ROOT/scripts/lib/app.sh"

uni_setup() {
  PATH="$STUB_BIN:/usr/bin:/bin:/usr/sbin:/sbin"
  export PCS_QUIT_TIMEOUT=1
  export PCS_TTY=/nonexistent/tty
  stub osascript 'exit 0'
  trap uni_cleanup EXIT
  mkdir -p "$TEST_TMP/rel"
}
uni_cleanup() {
  local p
  for p in $(app_pids); do kill "$p" 2>/dev/null || true; done
}
# uni_install_app <pcs-stub-body>: unpack a fake app into the temp HOME with a custom pcs.
uni_install_app() {
  make_fake_release "$TEST_TMP/rel" v1
  mkdir -p "$HOME/Applications"
  ditto -x -k "$TEST_TMP/rel/Personalization-Setup-macos.zip" "$HOME/Applications"
  printf '#!/bin/bash\n%s\n' "$1" >"$(app_dest)/Contents/MacOS/pcs"
  chmod +x "$(app_dest)/Contents/MacOS/pcs"
  mkdir -p "$HOME/.pcs/bin" "$HOME/.claude"
  printf '#!/bin/sh\n' >"$HOME/.pcs/bin/pcs-hook"
}
# uni_run <args...>: runs uninstall.sh with no stdin, sets UNI_OUT and UNI_RC.
uni_run() {
  UNI_RC=0
  UNI_OUT="$("$REPO_ROOT/uninstall.sh" "$@" 2>&1 </dev/null)" || UNI_RC=$?
}

t_uninstall_runs_pcs_then_removes_app() {
  uni_setup
  uni_install_app 'echo "$*" >"$HOME/pcs.args"; echo "{\"removedHooks\":3,\"backup\":\"/x\"}"'
  printf 'x' >"$HOME/.claude/settings.json.pcs-original"
  uni_run --yes
  assert_eq 0 "$UNI_RC" "exit: $UNI_OUT"
  assert_eq "uninstall --yes" "$(cat "$HOME/pcs.args")" "pcs args"
  assert_contains "$UNI_OUT" "Removed 3 hook(s)."
  assert_contains "$UNI_OUT" "Backup of your settings: /x"
  assert_contains "$UNI_OUT" "Uninstalled. Your original settings copy is still at ~/.claude/settings.json.pcs-original"
  [ ! -e "$(app_dest)" ] || fail "app still present"
}

t_uninstall_empty_backup_and_no_original() {
  uni_setup
  uni_install_app 'echo "{\"removedHooks\":0,\"backup\":\"\"}"'
  uni_run --yes
  assert_eq 0 "$UNI_RC" "exit: $UNI_OUT"
  assert_contains "$UNI_OUT" "Removed 0 hook(s)."
  case "$UNI_OUT" in *"Backup of your settings"*|*"still at"*) fail "unexpected lines: $UNI_OUT" ;; esac
  assert_contains "$UNI_OUT" "Uninstalled."
}

t_uninstall_cancel_keeps_everything() {
  uni_setup
  uni_install_app 'touch "$HOME/pcs.ran"; echo "{\"removedHooks\":1,\"backup\":\"\"}"'
  uni_run
  assert_eq 0 "$UNI_RC" "no tty exit"
  assert_contains "$UNI_OUT" "Cancelled."
  printf 'n\n' >"$TEST_TMP/ans"
  PCS_TTY="$TEST_TMP/ans" uni_run
  assert_eq 0 "$UNI_RC" "n exit"
  assert_contains "$UNI_OUT" "Continue? [y/N] "
  assert_contains "$UNI_OUT" "Cancelled."
  assert_file "$(app_dest)/Contents/MacOS/pcs"
  assert_file "$HOME/.pcs/bin/pcs-hook"
  assert_no_file "$HOME/pcs.ran"
}

t_uninstall_confirm_yes_proceeds() {
  uni_setup
  uni_install_app 'echo "{\"removedHooks\":2,\"backup\":\"\"}"'
  printf 'y\n' >"$TEST_TMP/ans"
  PCS_TTY="$TEST_TMP/ans" uni_run
  assert_eq 0 "$UNI_RC" "exit: $UNI_OUT"
  assert_contains "$UNI_OUT" "Removed 2 hook(s)."
  [ ! -e "$(app_dest)" ] || fail "app still present"
}

t_uninstall_without_app_keeps_launcher() {
  uni_setup
  mkdir -p "$HOME/.pcs/bin"
  printf '#!/bin/sh\n' >"$HOME/.pcs/bin/pcs-hook"
  uni_run --yes
  assert_eq 1 "$UNI_RC" "exit"
  assert_contains "$UNI_OUT" "The app is already gone, so its hooks can't be removed automatically. They do nothing while the app is missing. To finish: reinstall with ./install.sh, then run ./uninstall.sh again."
  assert_file "$HOME/.pcs/bin/pcs-hook"
}

t_uninstall_pcs_error_keeps_app() {
  uni_setup
  uni_install_app 'echo "{\"error\":\"settings.json is not valid JSON\"}"; exit 1'
  uni_run --yes
  assert_eq 1 "$UNI_RC" "exit"
  assert_contains "$UNI_OUT" "settings.json is not valid JSON"
  assert_file "$(app_dest)/Contents/MacOS/pcs"
  assert_file "$HOME/.pcs/bin/pcs-hook"
}

t_uninstall_pcs_garbage_keeps_app() {
  uni_setup
  uni_install_app 'echo "boom"; exit 1'
  uni_run --yes
  assert_eq 1 "$UNI_RC" "exit"
  assert_file "$(app_dest)/Contents/MacOS/pcs"
}

t_uninstall_quits_running_app_first() {
  uni_setup
  uni_install_app 'if [ -f "$HOME/quit.called" ]; then echo "{\"removedHooks\":1,\"backup\":\"\"}"; else echo "{\"error\":\"pcs ran before quit\"}"; exit 1; fi'
  nohup "$(app_dest)/Contents/MacOS/personalization-setup" >/dev/null 2>&1 </dev/null &
  disown
  local i=0
  while [ -z "$(app_pids)" ] && [ $i -lt 50 ]; do sleep 0.1; i=$((i + 1)); done
  [ -n "$(app_pids)" ] || fail "fake app did not start"
  stub osascript 'touch "$HOME/quit.called"; for p in '"$(app_pids | tr '\n' ' ')"'; do kill "$p"; done'
  uni_run --yes
  assert_eq 0 "$UNI_RC" "exit: $UNI_OUT"
  assert_file "$HOME/quit.called"
  [ -z "$(app_pids)" ] || fail "app still running"
  [ ! -e "$(app_dest)" ] || fail "app still present"
}

t_uninstall_writes_log() {
  uni_setup
  uni_install_app 'echo "{\"removedHooks\":1,\"backup\":\"\"}"'
  uni_run --yes
  # pcs normally deletes the log folder; the stub does not, so the log must exist.
  assert_contains "$(cat "$HOME/Library/Logs/Personalization Setup/install.log")" "uninstall.sh"
}

t_uninstall_bad_arg_exits_1() {
  uni_setup
  uni_run --bogus
  assert_eq 1 "$UNI_RC" "exit"
  assert_contains "$UNI_OUT" "Usage: ./uninstall.sh [--yes]"
}
