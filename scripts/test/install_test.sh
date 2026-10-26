#!/bin/bash
# Tests for install.sh (sourced by run.sh). Never touches the real ~.

. "$(dirname "${BASH_SOURCE[0]}")/fixtures.sh"
. "$REPO_ROOT/scripts/lib/app.sh"

# Common setup: system-only PATH (so a real claude/gh can't leak in), stubs, fast timeouts.
inst_setup() {
  PATH="$STUB_BIN:/usr/bin:/bin:/usr/sbin:/sbin"
  export PCS_LAUNCH_TIMEOUT=10 PCS_QUIT_TIMEOUT=1 PCS_LAUNCH_SETTLE=1
  export PCS_TTY=/nonexistent/tty
  stub_open_runs_app
  stub osascript 'exit 0'
  trap inst_cleanup EXIT
  mkdir -p "$TEST_TMP/rel"
}
inst_with_claude() { stub claude 'echo "1.0.0 (Claude Code)"'; }
inst_cleanup() {
  local p
  for p in $(app_pids); do kill "$p" 2>/dev/null || true; done
}
# inst_run <args...>: runs install.sh, sets INST_OUT and INST_RC.
inst_run() {
  INST_RC=0
  INST_OUT="$("$REPO_ROOT/install.sh" "$@" 2>&1 </dev/null)" || INST_RC=$?
}
inst_marker() { cat "$(app_dest)/Contents/Resources/marker"; }

t_install_from_zip_happy_path() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "exit: $INST_OUT"
  assert_file "$(app_dest)/Contents/MacOS/pcs"
  launcher_points_at_dest || fail "launcher not pointing at dest"
  assert_contains "$INST_OUT" 'Installed. Personalization Setup is open: click "+ Add hook" to start.'
  assert_file "$HOME/Library/Logs/Personalization Setup/install.log"
  assert_contains "$(cat "$HOME/Library/Logs/Personalization Setup/install.log")" "Installed. Personalization Setup is open"
}

t_install_no_tty_without_claude_exits_1() {
  inst_setup
  make_fake_release "$TEST_TMP/rel" v1
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 1 "$INST_RC" "exit"
  assert_contains "$INST_OUT" "Claude Code is required. Install it (https://code.claude.com/docs/en/setup), then re-run ./install.sh."
  assert_no_file "$HOME/Applications"
}

t_install_claude_flag_runs_official_installer() {
  inst_setup
  make_fake_release "$TEST_TMP/rel" v1
  stub curl 'echo "$*" >>"$HOME/curl.args"
cat <<EOS
mkdir -p "\$HOME/.local/bin"
printf "#!/bin/sh\necho 1.0.0\n" >"\$HOME/.local/bin/claude"
chmod +x "\$HOME/.local/bin/claude"
EOS'
  inst_run --install-claude --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "exit: $INST_OUT"
  assert_contains "$(cat "$HOME/curl.args")" "https://claude.ai/install.sh"
  [ -x "$HOME/.local/bin/claude" ] || fail "claude not created"
}

t_install_claude_installer_failure_exits_1() {
  inst_setup
  make_fake_release "$TEST_TMP/rel" v1
  stub curl 'echo "true"'
  inst_run --install-claude --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 1 "$INST_RC" "exit"
  assert_contains "$INST_OUT" "Claude Code didn't install correctly. See https://code.claude.com/docs/en/setup"
  assert_no_file "$HOME/Applications"
}

t_install_old_macos_exits_1() {
  inst_setup; inst_with_claude
  stub sw_vers 'echo 12.6'
  make_fake_release "$TEST_TMP/rel" v1
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 1 "$INST_RC" "exit"
  assert_contains "$INST_OUT" "Personalization Setup needs macOS 13 or later."
}

t_install_bad_checksum_changes_nothing() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  mkdir -p "$(app_dest)/Contents"
  echo old >"$(app_dest)/Contents/old-file"
  echo tampered >>"$TEST_TMP/rel/Personalization-Setup-macos.zip"
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 1 "$INST_RC" "exit"
  assert_contains "$INST_OUT" "The download is corrupt (checksum mismatch). Nothing was installed."
  assert_eq old "$(cat "$(app_dest)/Contents/old-file")"
  [ -z "$(app_pids)" ] || fail "app should not be running"
}

t_install_zip_without_app_exits_1() {
  inst_setup; inst_with_claude
  mkdir -p "$TEST_TMP/empty/Other.app"
  (cd "$TEST_TMP/empty" && ditto -c -k --keepParent Other.app "$TEST_TMP/rel/z.zip")
  (cd "$TEST_TMP/rel" && shasum -a 256 z.zip >z.zip.sha256)
  inst_run --zip "$TEST_TMP/rel/z.zip"
  assert_eq 1 "$INST_RC" "exit"
  assert_contains "$INST_OUT" "The download doesn't contain the app. Nothing was installed."
  assert_no_file "$HOME/Applications"
}

t_install_upgrades_running_app() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "first install: $INST_OUT"
  local old; old="$(app_pids)"
  [ -n "$old" ] || fail "app should be running after first install"
  make_fake_release "$TEST_TMP/rel" v2
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "upgrade: $INST_OUT"
  local now; now="$(app_pids)"
  [ -n "$now" ] || fail "no app running after upgrade"
  assert_eq 1 "$(echo "$now" | wc -l | tr -d ' ')" "number of running copies"
  [ "$now" != "$old" ] || fail "old PID still running"
  assert_eq v2 "$(inst_marker)"
}

t_install_original_backup_once() {
  inst_setup; inst_with_claude
  mkdir -p "$HOME/.claude"
  echo '{"first":1}' >"$HOME/.claude/settings.json"
  make_fake_release "$TEST_TMP/rel" v1
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "exit: $INST_OUT"
  assert_contains "$INST_OUT" "Saved a copy of your current Claude Code settings: ~/.claude/settings.json.pcs-original"
  assert_eq '{"first":1}' "$(cat "$HOME/.claude/settings.json.pcs-original")"
  echo '{"second":2}' >"$HOME/.claude/settings.json"
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "second: $INST_OUT"
  assert_eq '{"first":1}' "$(cat "$HOME/.claude/settings.json.pcs-original")"
  case "$INST_OUT" in *"Saved a copy"*) fail "second run should not save again" ;; esac
}

t_install_launch_failure_quarantine_message() {
  inst_setup; inst_with_claude
  export PCS_LAUNCH_TIMEOUT=2
  stub xattr 'exit 0'
  make_fake_release "$TEST_TMP/rel" v1 exit
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 2 "$INST_RC" "exit"
  assert_contains "$INST_OUT" 'macOS blocked the app. Open System Settings → Privacy & Security, click "Open Anyway" for Personalization Setup, then run the same ./install.sh command again.'
}

t_install_launch_failure_without_quarantine() {
  inst_setup; inst_with_claude
  export PCS_LAUNCH_TIMEOUT=2
  stub xattr 'exit 1'
  make_fake_release "$TEST_TMP/rel" v1 exit
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 2 "$INST_RC" "exit"
  assert_contains "$INST_OUT" "The app didn't start. Open it from ~/Applications to see the error, and check the log: ~/Library/Logs/Personalization Setup/app.log"
}

t_install_download_routes() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  mkdir -p "$TEST_TMP/src"
  cp "$TEST_TMP/rel/"Personalization-Setup-macos.zip* "$TEST_TMP/src/"
  # Route 1: gh authenticated.
  stub gh 'echo "gh $*" >>"$HOME/calls"
[ "$1 $2" = "auth status" ] && exit 0
dir=""; while [ $# -gt 0 ]; do [ "$1" = "--dir" ] && dir="$2"; shift; done
cp "'"$TEST_TMP"'/src/"Personalization-Setup-macos.zip* "$dir/"'
  stub curl 'echo "curl $*" >>"$HOME/calls"; exit 22'
  inst_run --version v1.2.3
  assert_eq 0 "$INST_RC" "gh route: $INST_OUT"
  assert_contains "$(cat "$HOME/calls")" "gh release download v1.2.3 --repo kushanparikh/personalizationSetup_ClaudeCode"
  case "$(cat "$HOME/calls")" in *curl*) fail "curl should not be used when gh works" ;; esac
  # Route 2: no gh, curl latest.
  rm -f "$STUB_BIN/gh" "$HOME/calls"
  stub curl 'echo "curl $*" >>"$HOME/calls"
out=""; while [ $# -gt 0 ]; do [ "$1" = "-o" ] && out="$2"; last="$1"; shift; done
cp "'"$TEST_TMP"'/src/$(basename "$last")" "$out"'
  inst_run
  assert_eq 0 "$INST_RC" "curl route: $INST_OUT"
  assert_contains "$(cat "$HOME/calls")" "https://github.com/kushanparikh/personalizationSetup_ClaudeCode/releases/latest/download/Personalization-Setup-macos.zip"
  assert_contains "$(cat "$HOME/calls")" "https://github.com/kushanparikh/personalizationSetup_ClaudeCode/releases/latest/download/Personalization-Setup-macos.zip.sha256"
  # Route 3: everything fails.
  stub curl 'exit 22'
  inst_run
  assert_eq 1 "$INST_RC" "failure exit"
  assert_contains "$INST_OUT" "Couldn't download the app. The repository may be private: install GitHub CLI (brew install gh), run 'gh auth login', and re-run. Or download Personalization-Setup-macos.zip from the Releases page and run ./install.sh --zip <file>."
}

t_install_download_requires_checksum() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  stub curl 'out=""; while [ $# -gt 0 ]; do [ "$1" = "-o" ] && out="$2"; last="$1"; shift; done
case "$last" in *.sha256) exit 22 ;; esac
cp "'"$TEST_TMP"'/rel/Personalization-Setup-macos.zip" "$out"'
  inst_run
  assert_eq 1 "$INST_RC" "exit"
  assert_no_file "$HOME/Applications"
}

# A hook fires right after the first quit and relaunches the old bundle: the
# stub `mv`/`rm` start the old app the first time they touch the installed path
# after a quit. osascript "quit" kills every copy, like quitting by app name.
t_install_hook_relaunch_during_upgrade() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "first install: $INST_OUT"
  make_fake_release "$TEST_TMP/rel" v2
  stub osascript '. "$REPO_ROOT/scripts/test/fixtures.sh"; fake_kill_all
[ -e "$HOME/relaunch.fired" ] || touch "$HOME/relaunch.armed"'
  local relaunch='. "$REPO_ROOT/scripts/test/fixtures.sh"
if [ -e "$HOME/relaunch.armed" ] && [ ! -e "$HOME/relaunch.fired" ]; then
  for a in "$@"; do
    if [ "$a" = "$HOME/Applications/Personalization Setup.app" ] && [ -d "$a" ]; then
      touch "$HOME/relaunch.fired"; fake_start_and_wait "$a" >"$HOME/relaunch.pid"; break
    fi
  done
fi'
  stub mv "$relaunch"'
exec /bin/mv "$@"'
  stub rm "$relaunch"'
exec /bin/rm "$@"'
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "upgrade: $INST_OUT"
  assert_file "$HOME/relaunch.fired"
  local rp; rp="$(cat "$HOME/relaunch.pid")"
  [ -n "$rp" ] || fail "relaunch did not start"
  assert_eq v1 "$(fake_marker_of "$rp")" "relaunched copy"
  ! kill -0 "$rp" 2>/dev/null || fail "relaunched old copy still running"
  local now; now="$(app_pids)"
  assert_eq 1 "$(echo "$now" | grep -c .)" "number of running copies"
  assert_eq v2 "$(fake_marker_of "$now")" "running copy"
  assert_eq v2 "$(inst_marker)"
  assert_no_file "$HOME/Applications/.Personalization Setup.app.old"
  assert_no_file "$HOME/Applications/.Personalization Setup.app.new"
}

t_install_quit_failure_cleans_staging() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "first install: $INST_OUT"
  inst_cleanup
  mkdir -p "$HOME/Applications/.Personalization Setup.app.new/stale"
  # A copy that ignores SIGTERM, so app_quit fails.
  (trap '' TERM; exec -a "$(app_dest)/Contents/MacOS/personalization-setup" /bin/sleep 30) &
  local stuck=$!
  trap 'kill -9 '"$stuck"' 2>/dev/null || true; inst_cleanup' EXIT
  local i=0
  while [ -z "$(app_pids)" ] && [ $i -lt 50 ]; do sleep 0.1; i=$((i + 1)); done
  make_fake_release "$TEST_TMP/rel" v2
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 1 "$INST_RC" "exit: $INST_OUT"
  assert_contains "$INST_OUT" "Couldn't quit the running app. Quit it and re-run."
  assert_eq v1 "$(inst_marker)"
  assert_no_file "$HOME/Applications/.Personalization Setup.app.new"
  assert_no_file "$HOME/Applications/.Personalization Setup.app.old"
}

t_install_zip_requires_sha256() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  rm -f "$TEST_TMP/rel/Personalization-Setup-macos.zip.sha256"
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 1 "$INST_RC" "exit"
  assert_contains "$INST_OUT" "Missing Personalization-Setup-macos.zip.sha256 next to the zip. Download both files from the Releases page."
  assert_no_file "$HOME/Applications"
}

# Browsers rename repeat downloads ("... (1).zip"); the checksum still matches.
t_install_zip_renamed_still_verifies() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  mv "$TEST_TMP/rel/Personalization-Setup-macos.zip" "$TEST_TMP/rel/dl (1).zip"
  mv "$TEST_TMP/rel/Personalization-Setup-macos.zip.sha256" "$TEST_TMP/rel/dl (1).zip.sha256"
  inst_run --zip "$TEST_TMP/rel/dl (1).zip"
  assert_eq 0 "$INST_RC" "exit: $INST_OUT"
  echo tampered >>"$TEST_TMP/rel/dl (1).zip"
  inst_run --zip "$TEST_TMP/rel/dl (1).zip"
  assert_eq 1 "$INST_RC" "tampered exit"
  assert_contains "$INST_OUT" "The download is corrupt (checksum mismatch). Nothing was installed."
}

t_install_clears_quarantine_after_checksum() {
  inst_setup; inst_with_claude
  make_fake_release "$TEST_TMP/rel" v1
  xattr -w com.apple.quarantine '0081;00000000;Safari;' "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "exit: $INST_OUT"
  ! xattr -p com.apple.quarantine "$(app_dest)" >/dev/null 2>&1 || fail "bundle still quarantined"
  ! xattr -p com.apple.quarantine "$(app_dest)/Contents/MacOS/personalization-setup" >/dev/null 2>&1 || fail "binary still quarantined"
}

t_install_original_backup_skips_when_app_hooks_present() {
  inst_setup; inst_with_claude
  mkdir -p "$HOME/.claude"
  echo '{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"'"$HOME"'/.pcs/bin/pcs-hook stop"}]}]}}' >"$HOME/.claude/settings.json"
  make_fake_release "$TEST_TMP/rel" v1
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "exit: $INST_OUT"
  assert_contains "$INST_OUT" "Skipped saving an original settings copy: Personalization Setup's hooks are already in your settings."
  assert_no_file "$HOME/.claude/settings.json.pcs-original"
  # ~/.pcs existing (the app ran before) also means the settings aren't original.
  inst_cleanup
  echo '{"plain":1}' >"$HOME/.claude/settings.json"
  inst_run --zip "$TEST_TMP/rel/Personalization-Setup-macos.zip"
  assert_eq 0 "$INST_RC" "second: $INST_OUT"
  assert_contains "$INST_OUT" "Skipped saving an original settings copy"
  assert_no_file "$HOME/.claude/settings.json.pcs-original"
}
