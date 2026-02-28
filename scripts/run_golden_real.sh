#!/usr/bin/env bash
set -u -o pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="${ROOT_DIR}/tests/golden_real/manifest.json"
BUNDLE="${ROOT_DIR}/policy/bundles/ops-v0.1"
TMP_WORK="${ROOT_DIR}/.tmp/golden-real"

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq is required" >&2
  exit 1
fi

if [[ ! -f "${MANIFEST}" ]]; then
  echo "error: manifest not found: ${MANIFEST}" >&2
  exit 1
fi

if [[ ! -d "${BUNDLE}" ]]; then
  echo "error: bundle not found: ${BUNDLE}" >&2
  exit 1
fi

mkdir -p "${TMP_WORK}/gocache" "${TMP_WORK}/gomodcache" "${TMP_WORK}/evidence"
export GOCACHE="${TMP_WORK}/gocache"
export GOMODCACHE="${TMP_WORK}/gomodcache"
export EVIDRA_EVIDENCE_DIR="${TMP_WORK}/evidence"

BIN_PATH=""
BIN_OWNED="0"

if [[ -n "${EVIDRA_BIN:-}" ]]; then
  BIN_PATH="${EVIDRA_BIN}"
elif [[ -x "${ROOT_DIR}/evidra" ]]; then
  BIN_PATH="${ROOT_DIR}/evidra"
else
  BIN_PATH="$(mktemp "${TMPDIR:-/tmp}/evidra-golden-real.XXXXXX")"
  BIN_OWNED="1"
  echo "Building evidra CLI..."
  if ! (cd "${ROOT_DIR}" && go build -o "${BIN_PATH}" ./cmd/evidra); then
    echo "error: failed to build evidra CLI (set EVIDRA_BIN or ensure prebuilt ./evidra exists)" >&2
    exit 1
  fi
fi

cleanup() {
  if [[ "${BIN_OWNED}" == "1" ]]; then
    rm -f "${BIN_PATH}"
  fi
}
trap cleanup EXIT

case_count="$(jq '.cases | length' "${MANIFEST}")"
if [[ "${case_count}" -eq 0 ]]; then
  echo "error: manifest has no cases" >&2
  exit 1
fi

echo "Running ${case_count} golden real-world cases..."

idx=0
while [[ "${idx}" -lt "${case_count}" ]]; do
  id="$(jq -r ".cases[${idx}].id" "${MANIFEST}")"
  env="$(jq -r ".cases[${idx}].environment // \"\"" "${MANIFEST}")"
  expected_decision="$(jq -r ".cases[${idx}].expected.decision" "${MANIFEST}")"
  expected_rule_ids="$(jq -c ".cases[${idx}].expected.rule_ids | sort" "${MANIFEST}")"

  fixture="${ROOT_DIR}/tests/golden_real/${id}.json"
  if [[ ! -f "${fixture}" ]]; then
    echo "FAIL ${id}: fixture missing: ${fixture}" >&2
    exit 1
  fi

  cmd=("${BIN_PATH}" validate --offline --bundle "${BUNDLE}" --json)
  if [[ -n "${env}" ]]; then
    cmd+=(--environment "${env}")
  fi
  cmd+=("${fixture}")

  set +e
  output="$(${cmd[@]} 2>&1)"
  rc=$?
  set -e

  if [[ "${rc}" -ne 0 && "${rc}" -ne 2 ]]; then
    echo "FAIL ${id}: validator exited with rc=${rc}" >&2
    echo "${output}" >&2
    exit 1
  fi

  if ! echo "${output}" | jq -e . >/dev/null 2>&1; then
    echo "FAIL ${id}: output is not valid JSON" >&2
    echo "${output}" >&2
    exit 1
  fi

  status="$(echo "${output}" | jq -r '.status')"
  case "${status}" in
    PASS) actual_decision="allow" ;;
    FAIL) actual_decision="deny" ;;
    *)
      echo "FAIL ${id}: unexpected status=${status}" >&2
      echo "${output}" >&2
      exit 1
      ;;
  esac

  actual_rule_ids="$(echo "${output}" | jq -c '(.rule_ids // []) | sort')"

  if [[ "${actual_decision}" != "${expected_decision}" ]]; then
    echo "FAIL ${id}: decision mismatch (expected=${expected_decision}, actual=${actual_decision})" >&2
    echo "${output}" >&2
    exit 1
  fi

  if [[ "${actual_rule_ids}" != "${expected_rule_ids}" ]]; then
    echo "FAIL ${id}: rule_ids mismatch" >&2
    echo "  expected: ${expected_rule_ids}" >&2
    echo "  actual:   ${actual_rule_ids}" >&2
    echo "${output}" >&2
    exit 1
  fi

  echo "OK   ${id}"
  idx=$((idx + 1))
done

echo "All ${case_count} golden real-world cases passed."
