#!/usr/bin/env bash
# Probe Polypus before EmailOps AI work. Fail closed on any error.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${SHOULD_I_READ_BIN:-$ROOT/bin/should-i-read}"
ENV_FILE="${EMAILOPS_POLYPUS_ENV:-$ROOT/config/emailops-polypus.env}"
EXAMPLE="$ROOT/config/emailops-polypus.example.env"

if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
elif [[ -f "$EXAMPLE" ]]; then
  # shellcheck disable=SC1090
  set -a
  # shellcheck disable=SC1090
  source "$EXAMPLE"
  set +a
fi

resolve_base_url() {
  if [[ -x "$BIN" ]] && command -v jq >/dev/null 2>&1; then
    local cfg_url
    cfg_url="$( "$BIN" config show 2>/dev/null | jq -r '.polypus.base_url // empty' )"
    if [[ -n "$cfg_url" && "$cfg_url" != "null" ]]; then
      printf '%s' "$cfg_url"
      return 0
    fi
  fi
  if [[ -n "${POLYPUS_BASE_URL:-}" ]]; then
    printf '%s' "$POLYPUS_BASE_URL"
    return 0
  fi
  printf '%s' "http://127.0.0.1:1320"
}

BASE_URL="$(resolve_base_url)"
BASE_URL="${BASE_URL%/}"

echo "Polypus base URL: $BASE_URL"

if ! curl -sf --max-time 5 "$BASE_URL/health" >/tmp/polypus-health.$$.json; then
  echo "FAIL: Polypus /health unreachable at $BASE_URL" >&2
  echo "Set polypus.base_url in ~/.config/should-i-read/config.yaml (see: should-i-read config bump), export POLYPUS_BASE_URL, or start local Polypus (make serve in the Polypus repo)." >&2
  exit 1
fi

echo "OK: /health"
if command -v jq >/dev/null 2>&1; then
  jq . </tmp/polypus-health.$$.json
else
  cat /tmp/polypus-health.$$.json
  echo
fi
rm -f /tmp/polypus-health.$$.json

if ! curl -sf --max-time 10 "$BASE_URL/v1/models" >/tmp/polypus-models.$$.json; then
  echo "FAIL: Polypus /v1/models unreachable" >&2
  exit 1
fi

echo "OK: /v1/models (enabled)"
if command -v jq >/dev/null 2>&1; then
  jq -r '.data[]?.id // empty' </tmp/polypus-models.$$.json
  count="$(jq -r '.data | length' </tmp/polypus-models.$$.json)"
else
  cat /tmp/polypus-models.$$.json
  echo
  count="unknown"
fi
rm -f /tmp/polypus-models.$$.json

if [[ "$count" == "0" ]]; then
  echo "FAIL: no enabled models; check ~/.config/polypus/config.yaml allow-lists" >&2
  exit 1
fi

echo "OK: Polypus ready for EmailOps gateway traffic ($count enabled model(s))"
