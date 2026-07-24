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
grep -F 'bin.install "patchlog"' "${dist_dir}/patchlog.rb"
ruby -c "${dist_dir}/patchlog.rb"
python3 -m json.tool "${dist_dir}/patchlog.json" >/dev/null
grep -F '"version": "9.8.7"' "${dist_dir}/patchlog.json"

RELEASE_TAG="${version}" \
  SOURCE_DIGEST="0123456789abcdef0123456789abcdef01234567" \
  PLAN_FINGERPRINT="sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" \
  REPOSITORY="example/patchlog" \
  DIST_DIR="${dist_dir}" \
  bash scripts/render-release-receipt.sh
python3 -m json.tool "${dist_dir}/patchlog-release-receipt.json" >/dev/null
grep -F '"schema": "https://patchlog.dev/schemas/release-receipt/v1"' "${dist_dir}/patchlog-release-receipt.json"
grep -F '"plan_fingerprint": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"' "${dist_dir}/patchlog-release-receipt.json"
python3 -m json.tool docs/schemas/release-plan-v1.schema.json >/dev/null
python3 -m json.tool docs/schemas/release-receipt-v1.schema.json >/dev/null

python3 scripts/product-metrics.py > "${test_root}/metrics.json"
cmp "${test_root}/metrics.json" docs/evidence/metrics.json

readonly_install_root="${test_root}/readonly-module-cache"
mkdir -p "${readonly_install_root}/module"
printf 'module cache\n' > "${readonly_install_root}/module/file.go"
chmod -R a-w "${readonly_install_root}/module"
RELEASE_TAG="v9.8.7" \
  REPOSITORY="example/patchlog" \
  GH_TOKEN="test-token" \
  SOURCE_DIGEST="0123456789abcdef0123456789abcdef01234567" \
  source scripts/verify-release.sh
cleanup_install_root "${readonly_install_root}"
if [ -e "${readonly_install_root}" ]; then
  echo "read-only Go module cache cleanup failed" >&2
  exit 1
fi

bash -n scripts/render-package-manifests.sh
bash -n scripts/render-release-receipt.sh
bash -n scripts/verify-release.sh
