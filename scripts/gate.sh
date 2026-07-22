#!/usr/bin/env bash
set -euo pipefail

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
BOLD=$'\033[1m'
RESET=$'\033[0m'

pass=0
fail=0
errors=""

export CGO_ENABLED="${CGO_ENABLED:-1}"

detect_go() {
  local candidates=(
    "$HOME/.local/go/bin/go"
    "/usr/local/go/bin/go"
    "/opt/homebrew/bin/go"
    "$(command -v go 2>/dev/null || true)"
  )
  for g in "${candidates[@]}"; do
    if [ -x "$g" ]; then
      echo "$g"
      return
    fi
  done
  return 1
}

GO_BIN=$(detect_go) || { echo "  go not found — quality gate cannot run" >&2; exit 1; }
export PATH="$(dirname "$GO_BIN"):$PATH"

run_check() {
  local name="$1"
  shift
  printf "  %-30s" "$name"
  local out
  if out=$("$@" 2>&1); then
    printf "${GREEN}PASS${RESET}\n"
    pass=$((pass + 1))
  else
    printf "${RED}FAIL${RESET}\n"
    fail=$((fail + 1))
    errors="${errors}\n  --- ${name} ---\n${out}\n"
  fi
}

echo ""
printf "  ${BOLD}patchlog quality gate${RESET}\n"
echo "  ──────────────────────────────────────────────────"
echo ""

run_check "go vet"            go vet ./...
run_check "gofmt"             bash -c 'test -z "$(gofmt -l . 2>/dev/null | grep -v vendor)"'
run_check "build"             go build -ldflags "-X main.version=dev" -o /dev/null ./cmd/patchlog/
run_check "release target builds" bash scripts/cross-build.sh
run_check "unit tests"        go test ./pkg/... -count=1 -race
run_check "integration tests" go test ./tests/integration/... -count=1
run_check "e2e tests"         go test ./tests/e2e/... -count=1

echo "  ──────────────────────────────────────────────────"

if [ "$fail" -gt 0 ]; then
  printf "  ${RED}${BOLD}GATE FAILED${RESET}  %d passed, %d failed\n\n" "$pass" "$fail"
  if [ -n "$errors" ]; then
    printf "  Details:\n"
    printf "%b\n" "$errors"
  fi
  exit 1
fi

printf "  ${GREEN}${BOLD}GATE PASSED${RESET}  %d checks ok\n\n" "$pass"
exit 0
