#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUTPUT_DIR=$(mktemp -d "${TMPDIR:-/tmp}/patchlog-cross-build.XXXXXX")
trap 'rm -rf "$OUTPUT_DIR"' EXIT

cd "$ROOT_DIR"

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os="${target%/*}"
  arch="${target#*/}"
  suffix=""
  if [ "$os" = windows ]; then
    suffix=".exe"
  fi
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build \
    -trimpath -ldflags "-s -w -X main.version=cross-build" \
    -o "$OUTPUT_DIR/patchlog_${os}_${arch}${suffix}" ./cmd/patchlog
done
