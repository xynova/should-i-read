#!/usr/bin/env bash
# Install the host mail-sync dependency (Neverest) when missing.
# Operators: go tool task ensure | should-i-read pim ensure
# Does not replace an existing resolved binary unless NEVEREST_FORCE_INSTALL=1.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECK="$ROOT/scripts/check-neverest.sh"
REPO_URL="${NEVEREST_GIT_URL:-https://github.com/pimalaya/neverest}"

if [[ "${NEVEREST_FORCE_INSTALL:-}" != "1" ]] && "$CHECK" 2>/dev/null; then
  echo "Mail sync dependency already available; skip install (set NEVEREST_FORCE_INSTALL=1 to reinstall)."
  exit 0
fi

if ! command -v cargo >/dev/null 2>&1; then
  echo "FAIL: cargo not on PATH; cannot install the mail sync dependency from git." >&2
  echo "Install Rust (https://rustup.rs) or place a Neverest binary and set pimalaya.neverest_bin." >&2
  echo "Releases: https://github.com/pimalaya/neverest/releases" >&2
  echo "Then re-run: go tool task ensure" >&2
  exit 1
fi

echo "Installing mail sync dependency from $REPO_URL (cargo install --git)..."
cargo install --git "$REPO_URL"

# cargo install lands under $CARGO_HOME/bin; host ResolveBin also checks that path.
cargo_bin="${CARGO_HOME:-$HOME/.cargo}/bin"
if [[ -x "$cargo_bin/neverest" ]]; then
  export PATH="$cargo_bin:$PATH"
fi

echo "Install finished; verifying..."
exec "$CHECK"
