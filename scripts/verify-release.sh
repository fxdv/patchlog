#!/usr/bin/env bash
set -euo pipefail

: "${RELEASE_TAG:?RELEASE_TAG is required}"
: "${REPOSITORY:?REPOSITORY is required (owner/repository)}"
: "${GH_TOKEN:?GH_TOKEN is required}"

VERIFY_SOURCE_IDENTITY="${VERIFY_SOURCE_IDENTITY:-true}"
VERIFY_MODULE_INSTALLS="${VERIFY_MODULE_INSTALLS:-true}"
VERIFY_RELEASE_RECEIPT="${VERIFY_RELEASE_RECEIPT:-true}"
SOURCE_DIGEST="${SOURCE_DIGEST:-}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-12}"
RETRY_DELAY_SECONDS="${RETRY_DELAY_SECONDS:-10}"

if [[ ! "${RELEASE_TAG}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid RELEASE_TAG: ${RELEASE_TAG}" >&2
  exit 2
fi
if [[ ! "${REPOSITORY}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo "REPOSITORY must be an owner/repository slug, got ${REPOSITORY}" >&2
  exit 2
fi
if [[ ! "${MAX_ATTEMPTS}" =~ ^[1-9][0-9]*$ ]] || [ "${MAX_ATTEMPTS}" -gt 30 ]; then
  echo "MAX_ATTEMPTS must be an integer from 1 to 30" >&2
  exit 2
fi
if [[ ! "${RETRY_DELAY_SECONDS}" =~ ^[0-9]+$ ]] || [ "${RETRY_DELAY_SECONDS}" -gt 60 ]; then
  echo "RETRY_DELAY_SECONDS must be an integer from 0 to 60" >&2
  exit 2
fi
case "${VERIFY_SOURCE_IDENTITY}:${VERIFY_MODULE_INSTALLS}:${VERIFY_RELEASE_RECEIPT}" in
  true:true:true | true:true:false | true:false:true | true:false:false | false:true:true | false:true:false | false:false:true | false:false:false) ;;
  *)
    echo "VERIFY_SOURCE_IDENTITY, VERIFY_MODULE_INSTALLS, and VERIFY_RELEASE_RECEIPT must be true or false" >&2
    exit 2
    ;;
esac
if [ "${VERIFY_SOURCE_IDENTITY}" = "true" ] && [ -z "${SOURCE_DIGEST}" ]; then
  echo "SOURCE_DIGEST is required when VERIFY_SOURCE_IDENTITY=true" >&2
  exit 2
fi
if [ "${VERIFY_SOURCE_IDENTITY}" = "true" ] && [[ ! "${SOURCE_DIGEST}" =~ ^[0-9a-f]{40,64}$ ]]; then
  echo "SOURCE_DIGEST must be a lowercase Git object digest" >&2
  exit 2
fi

retry() {
  local description="$1"
  shift
  local attempt
  for ((attempt = 1; attempt <= MAX_ATTEMPTS; attempt++)); do
    if "$@"; then
      return 0
    fi
    if [ "${attempt}" -eq "${MAX_ATTEMPTS}" ]; then
      echo "${description} failed after ${MAX_ATTEMPTS} attempts" >&2
      return 1
    fi
    echo "${description} not ready (attempt ${attempt}/${MAX_ATTEMPTS}); retrying in ${RETRY_DELAY_SECONDS}s" >&2
    sleep "${RETRY_DELAY_SECONDS}"
  done
}

release_is_published() {
  [ "$(gh release view "${RELEASE_TAG}" --repo "${REPOSITORY}" --json isDraft --jq .isDraft 2>/dev/null)" = "false" ]
}

verify_attestation() {
  local archive="$1"
  local arguments=(
    "${archive}"
    --repo "${REPOSITORY}"
    --signer-workflow "${REPOSITORY}/.github/workflows/release.yml"
  )
  if [ "${VERIFY_SOURCE_IDENTITY}" = "true" ]; then
    arguments+=(
      --source-digest "${SOURCE_DIGEST}"
      --source-ref "refs/tags/${RELEASE_TAG}"
    )
  fi
  gh attestation verify "${arguments[@]}"
}

cleanup_install_root() {
  local install_root="$1"
  # Go deliberately stores downloaded modules read-only. Make only this
  # mktemp-owned tree writable so cleanup cannot turn a successful install
  # verification into a false failure.
  chmod -R u+w "${install_root}" 2>/dev/null || true
  rm -rf -- "${install_root}"
}

install_and_verify_module() {
  local selector="$1"
  local expected="$2"
  local install_root
  install_root="$(mktemp -d)"
  if ! GOBIN="${install_root}/bin" \
    GOCACHE="${install_root}/build-cache" \
    GOMODCACHE="${install_root}/module-cache" \
    GOENV="off" \
    GOINSECURE="" \
    GONOSUMDB="" \
    GOPRIVATE="" \
    GOPROXY="https://proxy.golang.org,direct" \
    GOSUMDB="sum.golang.org" \
    go install "github.com/fxdv/patchlog/cmd/patchlog@${selector}"; then
    cleanup_install_root "${install_root}"
    return 1
  fi
  if ! "${install_root}/bin/patchlog" --version | grep -Fx "patchlog ${expected}"; then
    cleanup_install_root "${install_root}"
    return 1
  fi
  if ! go version -m "${install_root}/bin/patchlog" |
    awk -v module="github.com/fxdv/patchlog" -v version="${expected}" \
      '$1 == "mod" && $2 == module && $3 == version { found = 1 } END { exit !found }'; then
    cleanup_install_root "${install_root}"
    return 1
  fi
  cleanup_install_root "${install_root}"
}

run_release_verification() {
retry "published GitHub release" release_is_published

verification_root="$(mktemp -d)"
trap 'rm -rf "${verification_root}"' EXIT
download_args=(
  "${RELEASE_TAG}"
  --repo "${REPOSITORY}"
  --dir "${verification_root}"
  --pattern 'patchlog_*.tar.gz'
  --pattern 'SHA256SUMS'
  --pattern 'patchlog.rb'
  --pattern 'patchlog.json'
)
if [ "${VERIFY_RELEASE_RECEIPT}" = "true" ]; then
  download_args+=(--pattern 'patchlog-release-receipt.json')
fi
gh release download "${download_args[@]}"

(
  cd "${verification_root}"
  sha256sum --check SHA256SUMS
)

archives=(
  "patchlog_${RELEASE_TAG}_linux_amd64.tar.gz"
  "patchlog_${RELEASE_TAG}_linux_arm64.tar.gz"
  "patchlog_${RELEASE_TAG}_darwin_amd64.tar.gz"
  "patchlog_${RELEASE_TAG}_darwin_arm64.tar.gz"
  "patchlog_${RELEASE_TAG}_windows_amd64.tar.gz"
)
for archive in "${archives[@]}"; do
  archive_path="${verification_root}/${archive}"
  if [ ! -f "${archive_path}" ]; then
    echo "missing release archive: ${archive}" >&2
    exit 1
  fi
  retry "provenance for ${archive}" verify_attestation "${archive_path}"
done

if [ "${VERIFY_RELEASE_RECEIPT}" = "true" ]; then
  receipt="${verification_root}/patchlog-release-receipt.json"
  if [ ! -f "${receipt}" ]; then
    echo "missing release receipt" >&2
    exit 1
  fi
  python3 - "${receipt}" "${RELEASE_TAG}" "${REPOSITORY}" "${SOURCE_DIGEST}" <<'PY'
import json
import sys

path, tag, repository, source_digest = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    receipt = json.load(handle)
expected = {
    "schema": "https://patchlog.dev/schemas/release-receipt/v1",
    "schema_version": 1,
    "phase": "finalize",
    "repository": repository,
    "source_commit": source_digest,
    "tag": tag,
}
for field, value in expected.items():
    if receipt.get(field) != value:
        raise SystemExit(f"release receipt {field}={receipt.get(field)!r}, expected {value!r}")
fingerprint = receipt.get("plan_fingerprint", "")
if not fingerprint.startswith("sha256:") or len(fingerprint) != 71:
    raise SystemExit("release receipt has an invalid plan fingerprint")
if not receipt.get("artifacts"):
    raise SystemExit("release receipt has no artifact subjects")
PY
  retry "provenance for release receipt" verify_attestation "${receipt}"
fi

archive_root="${verification_root}/archive"
mkdir -p "${archive_root}"
tar -C "${archive_root}" -xzf "${verification_root}/patchlog_${RELEASE_TAG}_linux_amd64.tar.gz"
"${archive_root}/patchlog_${RELEASE_TAG}_linux_amd64/patchlog" --version |
  grep -Fx "patchlog ${RELEASE_TAG}"

if [ "${VERIFY_MODULE_INSTALLS}" = "true" ]; then
  retry "go install @${RELEASE_TAG}" install_and_verify_module "${RELEASE_TAG}" "${RELEASE_TAG}"
  retry "go install @latest" install_and_verify_module "latest" "${RELEASE_TAG}"
fi

echo "release ${RELEASE_TAG} passed archive, checksum, provenance, and installation verification"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  run_release_verification
fi
