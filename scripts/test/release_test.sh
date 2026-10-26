#!/bin/bash
# Tests for scripts/check-version.sh and scripts/release.sh (sourced by run.sh).

# Copy the three version-bearing files into $1 with the repo's layout.
copy_version_files() {
  local dest="$1"
  mkdir -p "$dest/app/src-tauri"
  cp "$REPO_ROOT/app/package.json" "$dest/app/package.json"
  cp "$REPO_ROOT/app/src-tauri/tauri.conf.json" "$dest/app/src-tauri/"
  cp "$REPO_ROOT/app/src-tauri/Cargo.toml" "$dest/app/src-tauri/"
}

cur_version() { sed -n 's/^version = "\(.*\)"/\1/p' "$REPO_ROOT/app/src-tauri/Cargo.toml" | head -n1; }

t_check_version_matches() {
  local out
  out="$("$REPO_ROOT/scripts/check-version.sh" "v$(cur_version)" 2>&1)" || fail "should pass: $out"
}

t_check_version_mismatch_lists_files() {
  local out rc=0
  copy_version_files "$TEST_TMP/copy"
  sed 's/"version": "[^"]*"/"version": "9.9.9"/' "$REPO_ROOT/app/package.json" >"$TEST_TMP/copy/app/package.json"
  out="$(ROOT_DIR="$TEST_TMP/copy" "$REPO_ROOT/scripts/check-version.sh" "v$(cur_version)" 2>&1)" || rc=$?
  assert_eq 1 "$rc" "exit code"
  assert_contains "$out" "app/package.json"
  case "$out" in *tauri.conf.json*) fail "tauri.conf.json should not be listed: $out" ;; esac
}

t_release_refuses_dirty_tree() {
  local out rc=0
  git clone -q "$REPO_ROOT" "$TEST_TMP/clone"
  # The clone only has committed files; make sure release.sh is present there.
  mkdir -p "$TEST_TMP/clone/scripts"
  cp "$REPO_ROOT/scripts/release.sh" "$REPO_ROOT/scripts/check-version.sh" "$TEST_TMP/clone/scripts/"
  # Copying is itself an uncommitted change when the scripts are not yet committed; add another anyway.
  echo dirty >"$TEST_TMP/clone/dirty.txt"
  out="$(cd "$TEST_TMP/clone" && scripts/release.sh "v$(cur_version)" --dry-run 2>&1)" || rc=$?
  assert_eq 1 "$rc" "exit code"
  assert_contains "$out" "Working tree has uncommitted changes."
}

t_release_refuses_bad_tag() {
  local tag out rc
  for tag in 0.1 v0.1; do
    rc=0
    out="$("$REPO_ROOT/scripts/release.sh" "$tag" 2>&1)" || rc=$?
    assert_eq 1 "$rc" "exit code for $tag"
    assert_contains "$out" "Usage: scripts/release.sh vX.Y.Z [--dry-run]" "$tag"
  done
}
