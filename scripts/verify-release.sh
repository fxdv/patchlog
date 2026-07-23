#!/usr/bin/env bash
set -euo pipefail

: "${RELEASE_TAG:?RELEASE_TAG is required}"
: "${REPOSITORY:?REPOSITORY is required (owner/repository)}"
: "${GH_TOKEN:?GH_TOKEN is required}"

VERIFY_SOURCE_IDENTITY="${VERIFY_SOURCE_IDENTITY:-true}"
VERIFY_MODULE_INSTALLS="${VERIFY_MODULE_INSTALLS:-true}"
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
case "${VERIFY_SOURCE_IDENTITY}:${VERIFY_MODULE_INSTALLS}" in
  true:true | true:false | false:true | false:false) ;;
  *)
    echo "VERIFY_SOURCE_IDENTITY and VERIFY_MODULE_INSTALLS must be true or false" >&2
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
    rm -rf "${install_root}"
    return 1
  fi
  if ! "${install_root}/bin/patchlog" --version | grep -Fx "patchlog ${expected}"; then
    rm -rf "${install_root}"
    return 1
  fi
  if ! go version -m "${install_root}/bin/patchlog" |
    awk -v module="github.com/fxdv/patchlog" -v version="${expected}" \
      '$1 == "mod" && $2 == module && $3 == version { found = 1 } END { exit !found }'; then
    rm -rf "${install_root}"
    return 1
  fi
  rm -rf "${install_root}"
}

retry "published GitHub release" release_is_published

verification_root="$(mktemp -d)"
trap 'rm -rf "${verification_root}"' EXIT
gh release download "${RELEASE_TAG}" \
  --repo "${REPOSITORY}" \
  --dir "${verification_root}" \
  --pattern 'patchlog_*.tar.gz' \
  --pattern 'SHA256SUMS' \
  --pattern 'patchlog.rb' \
  --pattern 'patchlog.json'

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
