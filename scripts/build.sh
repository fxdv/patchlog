#!/usr/bin/env bash
set -euo pipefail

GREEN=$'\033[0;32m'
BOLD=$'\033[1m'
RESET=$'\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY="$ROOT_DIR/patchlog"

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
      return 0
    fi
  done
  return 1
}

GO_BIN=$(detect_go 2>/dev/null) || {
  echo "Go 1.22 or newer is required; refusing to install or replace toolchains automatically." >&2
  exit 1
}
export PATH="$(dirname "$GO_BIN"):$PATH"
export CGO_ENABLED=0

VERSION=$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-X main.version=${VERSION}"

printf "  ${BOLD}Building patchlog ${VERSION}...${RESET}\n"
go build -ldflags "$LDFLAGS" -o "$BINARY" "$ROOT_DIR/cmd/patchlog/"

printf "  ${GREEN}Done: ${BINARY}${RESET}\n"
