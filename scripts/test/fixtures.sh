#!/bin/bash
# Test fixtures for install/uninstall tests (sourced by *_test.sh files).

# make_fake_release <dir> [marker] [mode]
# Builds <dir>/Personalization-Setup-macos.zip (+ .sha256) containing a fake
# "Personalization Setup.app". mode: "run" (default; writes the launcher then
# sleeps) or "exit" (exits at once). marker is stored in Contents/Resources/marker.
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
