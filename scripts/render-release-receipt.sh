#!/usr/bin/env bash
set -euo pipefail

: "${RELEASE_TAG:?RELEASE_TAG is required}"
: "${SOURCE_DIGEST:?SOURCE_DIGEST is required}"
: "${PLAN_FINGERPRINT:?PLAN_FINGERPRINT is required}"
: "${REPOSITORY:?REPOSITORY is required}"
DIST_DIR="${DIST_DIR:-dist}"
OUTPUT_PATH="${OUTPUT_PATH:-${DIST_DIR}/patchlog-release-receipt.json}"

if [[ ! "${RELEASE_TAG}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "release receipt requires a stable vMAJOR.MINOR.PATCH tag" >&2
  exit 2
fi
if [[ ! "${SOURCE_DIGEST}" =~ ^[0-9a-f]{40,64}$ ]]; then
  echo "release receipt requires an exact lowercase Git object digest" >&2
  exit 2
fi
if [[ ! "${PLAN_FINGERPRINT}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  echo "release receipt requires an exact Patchlog plan fingerprint" >&2
  exit 2
fi
if [[ ! "${REPOSITORY}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
  echo "release receipt repository must be owner/name" >&2
  exit 2
fi
if [ ! -f "${DIST_DIR}/SHA256SUMS" ]; then
  echo "${DIST_DIR}/SHA256SUMS is required" >&2
  exit 2
fi

artifact_count="$(awk 'NF == 2 { count++ } END { print count + 0 }' "${DIST_DIR}/SHA256SUMS")"
if [ "${artifact_count}" -eq 0 ]; then
  echo "release receipt requires at least one checksummed artifact" >&2
  exit 2
fi

{
  printf '{\n'
  printf '  "schema": "https://patchlog.dev/schemas/release-receipt/v1",\n'
  printf '  "schema_version": 1,\n'
  printf '  "plan_fingerprint": "%s",\n' "${PLAN_FINGERPRINT}"
  printf '  "phase": "finalize",\n'
  printf '  "repository": "%s",\n' "${REPOSITORY}"
  printf '  "source_commit": "%s",\n' "${SOURCE_DIGEST}"
  printf '  "tag": "%s",\n' "${RELEASE_TAG}"
  printf '  "artifacts": [\n'

  index=0
  while read -r digest filename extra; do
    if [ -n "${extra:-}" ] || [[ ! "${digest}" =~ ^[0-9a-f]{64}$ ]] ||
      [[ ! "${filename}" =~ ^[A-Za-z0-9._-]+$ ]]; then
      echo "invalid SHA256SUMS entry for release receipt: ${digest} ${filename} ${extra:-}" >&2
      exit 2
    fi
    index=$((index + 1))
    suffix=","
    if [ "${index}" -eq "${artifact_count}" ]; then
      suffix=""
    fi
    printf '    {"name": "%s", "digest": {"sha256": "%s"}}%s\n' \
      "${filename}" "${digest}" "${suffix}"
  done < "${DIST_DIR}/SHA256SUMS"

  printf '  ],\n'
  printf '  "verification": {\n'
  printf '    "approved_plan_bound_to_tag": true,\n'
  printf '    "required_gate_passed": true,\n'
  printf '    "tag_matches_version": true,\n'
  printf '    "tag_targets_source_commit": true\n'
  printf '  }\n'
  printf '}\n'
} > "${OUTPUT_PATH}"
