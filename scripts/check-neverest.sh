#!/usr/bin/env bash
# Verify the host mail-sync dependency (Neverest) before mail readiness/sync.
# Resolution: NEVEREST_BIN → pimalaya.neverest_bin → PATH → $CARGO_HOME/bin.
# Operators use: should-i-read pim ensure | go tool task ensure | go tool task mail:readiness
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${SHOULD_I_READ_BIN:-$ROOT/bin/should-i-read}"

cargo_bin_dir() {
  if [[ -n "${CARGO_HOME:-}" ]]; then
    printf '%s/bin' "$CARGO_HOME"
    return 0
  fi
  printf '%s/.cargo/bin' "$HOME"
}

resolve_neverest() {
  if [[ -n "${NEVEREST_BIN:-}" ]]; then
    printf '%s' "$NEVEREST_BIN"
    return 0
  fi
  if [[ -x "$BIN" ]] && command -v jq >/dev/null 2>&1; then
    local cfg_bin
    cfg_bin="$( "$BIN" config show 2>/dev/null | jq -r '.pimalaya.neverest_bin // empty' )"
    if [[ -n "$cfg_bin" && "$cfg_bin" != "null" ]]; then
      printf '%s' "$cfg_bin"
      return 0
    fi
  fi
  if command -v neverest >/dev/null 2>&1; then
    command -v neverest
    return 0
  fi
  local cargo_neverest
  cargo_neverest="$(cargo_bin_dir)/neverest"
  if [[ -x "$cargo_neverest" ]]; then
    printf '%s' "$cargo_neverest"
    return 0
  fi
  return 1
}

if ! neverest_bin="$(resolve_neverest)"; then
  echo "FAIL: mail sync dependency missing (host uses Neverest under the hood)." >&2
  echo "Install with: go tool task ensure   # or: ./bin/should-i-read pim ensure" >&2
  echo "Or set pimalaya.neverest_bin / NEVEREST_BIN to an existing binary." >&2
  echo "See docs/pimalaya-setup.md" >&2
  exit 1
fi

echo "Mail sync binary: $neverest_bin"
if ! "$neverest_bin" --version; then
  echo "FAIL: mail sync binary --version failed for $neverest_bin" >&2
  echo "Install with: go tool task ensure" >&2
  exit 1
fi

echo "OK: mail sync dependency ready for mail readiness/sync"
