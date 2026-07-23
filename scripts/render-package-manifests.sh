#!/usr/bin/env bash
set -euo pipefail

: "${VERSION:?VERSION is required (for example v0.2.0)}"
DIST_DIR="${DIST_DIR:-dist}"
REPOSITORY="${REPOSITORY:-fxdv/patchlog}"

if [[ ! "${VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "package manifests require a stable vMAJOR.MINOR.PATCH VERSION, got ${VERSION}" >&2
  exit 2
fi
if [[ ! "${REPOSITORY}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo "REPOSITORY must be an owner/repository slug, got ${REPOSITORY}" >&2
  exit 2
fi
if [ ! -f "${DIST_DIR}/SHA256SUMS" ]; then
  echo "${DIST_DIR}/SHA256SUMS is required" >&2
  exit 2
fi

checksum() {
  local filename="$1"
  local digest
  digest="$(awk -v file="${filename}" '$2 == file { print $1 }' "${DIST_DIR}/SHA256SUMS")"
  if [ -z "${digest}" ]; then
    echo "checksum for ${filename} is missing" >&2
    exit 2
  fi
  printf '%s' "${digest}"
}

plain_version="${VERSION#v}"
base_url="https://github.com/${REPOSITORY}/releases/download/${VERSION}"
linux_amd64="patchlog_${VERSION}_linux_amd64.tar.gz"
linux_arm64="patchlog_${VERSION}_linux_arm64.tar.gz"
darwin_amd64="patchlog_${VERSION}_darwin_amd64.tar.gz"
darwin_arm64="patchlog_${VERSION}_darwin_arm64.tar.gz"
windows_amd64="patchlog_${VERSION}_windows_amd64.tar.gz"

linux_amd64_sha="$(checksum "${linux_amd64}")"
linux_arm64_sha="$(checksum "${linux_arm64}")"
darwin_amd64_sha="$(checksum "${darwin_amd64}")"
darwin_arm64_sha="$(checksum "${darwin_arm64}")"
windows_amd64_sha="$(checksum "${windows_amd64}")"

sed \
  -e "s|@@VERSION@@|${plain_version}|g" \
  -e "s|@@BASE_URL@@|${base_url}|g" \
  -e "s|@@LINUX_AMD64_ARCHIVE@@|${linux_amd64}|g" \
  -e "s|@@LINUX_AMD64_SHA@@|${linux_amd64_sha}|g" \
  -e "s|@@LINUX_ARM64_ARCHIVE@@|${linux_arm64}|g" \
  -e "s|@@LINUX_ARM64_SHA@@|${linux_arm64_sha}|g" \
  -e "s|@@DARWIN_AMD64_ARCHIVE@@|${darwin_amd64}|g" \
  -e "s|@@DARWIN_AMD64_SHA@@|${darwin_amd64_sha}|g" \
  -e "s|@@DARWIN_ARM64_ARCHIVE@@|${darwin_arm64}|g" \
  -e "s|@@DARWIN_ARM64_SHA@@|${darwin_arm64_sha}|g" \
  packaging/homebrew/patchlog.rb.tmpl > "${DIST_DIR}/patchlog.rb"

sed \
  -e "s|@@VERSION@@|${plain_version}|g" \
  -e "s|@@BASE_URL@@|${base_url}|g" \
  -e "s|@@WINDOWS_AMD64_ARCHIVE@@|${windows_amd64}|g" \
  -e "s|@@WINDOWS_AMD64_SHA@@|${windows_amd64_sha}|g" \
  packaging/scoop/patchlog.json.tmpl > "${DIST_DIR}/patchlog.json"
