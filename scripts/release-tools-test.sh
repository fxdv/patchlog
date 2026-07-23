#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
test_root="$(mktemp -d)"
trap 'rm -rf "${test_root}"' EXIT
dist_dir="${test_root}/dist"
mkdir -p "${dist_dir}"

version="v9.8.7"
archives=(
  "patchlog_${version}_linux_amd64.tar.gz"
  "patchlog_${version}_linux_arm64.tar.gz"
  "patchlog_${version}_darwin_amd64.tar.gz"
  "patchlog_${version}_darwin_arm64.tar.gz"
  "patchlog_${version}_windows_amd64.tar.gz"
)
for archive in "${archives[@]}"; do
  printf '%s\n' "${archive}" > "${dist_dir}/${archive}"
done
(
  cd "${dist_dir}"
  for archive in "${archives[@]}"; do
    sha256sum "${archive}"
  done > SHA256SUMS
)

cd "${repo_root}"
VERSION="${version}" \
  DIST_DIR="${dist_dir}" \
  REPOSITORY="example/patchlog" \
  bash scripts/render-package-manifests.sh

if grep -R '@@' "${dist_dir}/patchlog.rb" "${dist_dir}/patchlog.json"; then
  echo "package manifest contains an unresolved template placeholder" >&2
  exit 1
fi
grep -F 'version "9.8.7"' "${dist_dir}/patchlog.rb"
grep -F 'https://github.com/example/patchlog/releases/download/v9.8.7/' "${dist_dir}/patchlog.rb"
python3 -m json.tool "${dist_dir}/patchlog.json" >/dev/null
grep -F '"version": "9.8.7"' "${dist_dir}/patchlog.json"

bash -n scripts/render-package-manifests.sh
bash -n scripts/verify-release.sh
