#!/bin/bash
# Self-tests for run.sh itself.

run_nested() { # dir -> output (never fails)
  PCS_TEST_DIR="$1" bash "$REPO_ROOT/scripts/test/run.sh" 2>&1 || true
}

t_runner_fails_on_false_in_test() {
  local d="$TEST_TMP/nested" out
  mkdir -p "$d"
  printf 't_a1() {\n  false\n  echo reached\n}\n' >"$d/x_test.sh"
  out="$(run_nested "$d")"
  assert_contains "$out" "FAIL t_a1"
}

t_runner_runs_each_test_once() {
  local d="$TEST_TMP/nested" out
  mkdir -p "$d"
  printf 't_one() { :; }\n' >"$d/a_test.sh"
  printf 't_two() { :; }\n' >"$d/b_test.sh"
  out="$(run_nested "$d")"
  assert_contains "$out" "2 passed, 0 failed"
  assert_eq 1 "$(printf '%s\n' "$out" | grep -c '^ok t_one')" "t_one runs"
}
