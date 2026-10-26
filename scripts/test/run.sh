#!/bin/bash
# Test runner: sources every scripts/test/*_test.sh and runs each t_* function
# in a subshell with a fresh temp HOME. Helpers below are shared by all tests.
set -euo pipefail

TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$TEST_DIR/../.." && pwd)"
export REPO_ROOT

fail() { printf '%s' "$*" >"$REASON_FILE"; exit 1; }

assert_eq() { # expected actual [label]
  [ "$1" = "$2" ] || fail "${3:+$3: }expected [$1] got [$2]"
}
assert_contains() { # haystack needle [label]
  case "$1" in *"$2"*) ;; *) fail "${3:+$3: }expected output to contain [$2], got [$1]" ;; esac
}
assert_file() { [ -f "$1" ] || fail "expected file $1"; }
assert_no_file() { [ ! -e "$1" ] || fail "expected no file $1"; }

# stub <cmd> <script-body>: executable in a temp bin dir placed first on PATH.
stub() {
  mkdir -p "$STUB_BIN"
  printf '#!/bin/bash\n%s\n' "$2" >"$STUB_BIN/$1"
  chmod +x "$STUB_BIN/$1"
}

run_test() { # fn
  local fn="$1" tmp rc=0
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/pcs-test.XXXXXX")"
  mkdir -p "$tmp/home"
  REASON_FILE="$tmp/reason"
  (
    export HOME="$tmp/home" TEST_TMP="$tmp" REASON_FILE
    STUB_BIN="$tmp/bin"; export STUB_BIN
    PATH="$STUB_BIN:$PATH"
    cd "$tmp"
    "$fn" >"$tmp/out" 2>&1
  ) || rc=$?
  if [ "$rc" -eq 0 ]; then
    echo "ok $fn"; PASSED=$((PASSED + 1))
  else
    local reason="exit $rc"
    [ -s "$tmp/reason" ] && reason="$(cat "$tmp/reason")"
    echo "FAIL $fn: $reason"
    [ -s "$tmp/out" ] && sed 's/^/    | /' "$tmp/out"
    FAILED=$((FAILED + 1))
  fi
  rm -rf "$tmp"
}

PASSED=0; FAILED=0
for f in "$TEST_DIR"/*_test.sh; do
  [ -e "$f" ] || continue
  # Load the file in a subshell to discover its t_* functions, then run them.
  fns="$( (
    # shellcheck disable=SC1090
    . "$f"
    declare -F | awk '{print $3}' | grep '^t_' || true
  ) )"
  # shellcheck disable=SC1090
  . "$f"
  for fn in $fns; do run_test "$fn"; done
done

echo "$PASSED passed, $FAILED failed"
[ "$FAILED" -eq 0 ]
