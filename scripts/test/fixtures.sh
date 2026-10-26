#!/bin/bash
# Test fixtures for install/uninstall tests (sourced by *_test.sh files).

# make_fake_release <dir> [marker] [mode]
# Builds <dir>/Personalization-Setup-macos.zip (+ .sha256) containing a fake
# "Personalization Setup.app". mode: "run" (default; writes the launcher then
# sleeps) or "exit" (exits at once). marker is stored in Contents/Resources/marker.
# In "run" mode each start appends "<pid> <marker>" to $HOME/fake-app.log.
# The fake binary re-execs itself with argv[0] set to its own path so that `ps`
# shows the real path (a plain bash script would show "/bin/bash <path>").
make_fake_release() {
  local dir="$1" marker="${2:-v1}" mode="${3:-run}"
  local app="$dir/stage/Personalization Setup.app"
  rm -rf "$dir/stage"
  mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
  printf '%s\n' "$marker" >"$app/Contents/Resources/marker"
  printf '#!/bin/sh\nexit 0\n' >"$app/Contents/MacOS/pcs"
  chmod +x "$app/Contents/MacOS/pcs"
  cat >"$app/Contents/MacOS/personalization-setup" <<SCRIPT
#!/bin/bash
if [ -z "\${PCS_FAKE_REEXEC:-}" ]; then
  PCS_FAKE_REEXEC=1 exec -a "\$0" /bin/bash "\$0" fake
fi
[ "$mode" = "exit" ] && exit 0
echo "\$\$ $marker" >>"\$HOME/fake-app.log"
mkdir -p "\$HOME/.pcs/bin"
printf '#!/bin/sh\nBIN="%s"\n' "\$HOME/Applications/Personalization Setup.app/Contents/MacOS/pcs" >"\$HOME/.pcs/bin/pcs-hook"
while :; do sleep 1; done
SCRIPT
  chmod +x "$app/Contents/MacOS/personalization-setup"
  (cd "$dir/stage" && ditto -c -k --keepParent "Personalization Setup.app" "$dir/Personalization-Setup-macos.zip")
  (cd "$dir" && shasum -a 256 Personalization-Setup-macos.zip >Personalization-Setup-macos.zip.sha256)
  rm -rf "$dir/stage"
}

# stub_open_runs_app: `open <app>` starts the app's binary detached in the background.
stub_open_runs_app() {
  stub open 'nohup "$1/Contents/MacOS/personalization-setup" >/dev/null 2>&1 </dev/null &'
}

# fake_marker_of <pid>: the marker the fake app with that PID started with.
fake_marker_of() { awk -v p="$1" '$1 == p { m = $2 } END { print m }' "$HOME/fake-app.log"; }

# fake_start_and_wait <bundle>: start the bundle's binary detached and wait
# (up to 5 s) until it shows in `ps` under its own path. Prints its PID.
fake_start_and_wait() {
  local bin="$1/Contents/MacOS/personalization-setup" i=0 pid=""
  nohup "$bin" >/dev/null 2>&1 </dev/null &
  while [ "$i" -lt 50 ]; do
    pid="$(ps -axo pid=,command= | awk -v b="$bin " '{ p = $1; $1 = ""; sub(/^ /, ""); if (index($0, b) == 1) print p }' | tail -1)"
    [ -n "$pid" ] && break
    sleep 0.1; i=$((i + 1))
  done
  echo "$pid"
}

# fake_kill_all: kill every fake app process run from anywhere under
# $HOME/Applications (the installed path, or a staging/old path), then wait.
fake_kill_all() {
  local pids p i=0
  pids="$(ps -axo pid=,command= | awk -v h="$HOME/Applications/" '{ p = $1; $1 = ""; sub(/^ /, ""); if (index($0, h) == 1 && index($0, "/Contents/MacOS/personalization-setup ")) print p }')"
  for p in $pids; do kill "$p" 2>/dev/null || true; done
  for p in $pids; do
    while kill -0 "$p" 2>/dev/null && [ "$i" -lt 50 ]; do sleep 0.1; i=$((i + 1)); done
  done
}
