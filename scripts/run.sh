#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY="$ROOT_DIR/patchlog"

if [ ! -x "$BINARY" ]; then
  echo "Binary not found at $BINARY. Run ./scripts/build.sh first."
  exit 1
fi

CONFIG="$ROOT_DIR/patchlog.yaml"
REPO="${REPO:-.}"
EXTRA_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      CONFIG="$2"; shift 2 ;;
    --repo)
      REPO="$2"; shift 2 ;;
    *)
      EXTRA_ARGS+=("$1"); shift ;;
  esac
done

exec "$BINARY" \
  --repo "$REPO" \
  --config "$CONFIG" \
  "${EXTRA_ARGS[@]}"
