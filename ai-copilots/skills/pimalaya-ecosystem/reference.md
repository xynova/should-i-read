# Pimalaya ecosystem reference

## Path map

| Path | Purpose |
|------|---------|
| `docs/pimalaya-ecosystem/` | Committed AI index (cards, architecture, integration) |
| `docs/pimalaya-ecosystem/clone-manifest.yaml` | Tiered repo list |
| `tmp/pimalaya/<repo>/` | Shallow upstream clones (gitignored) |

## Clone script (A_must + B_core)

```bash
ROOT="$(git rev-parse --show-toplevel)"
MANIFEST="$ROOT/docs/pimalaya-ecosystem/clone-manifest.yaml"
mkdir -p "$ROOT/tmp/pimalaya"

# Example: clone one repo
git clone --depth 1 "https://github.com/pimalaya/neverest.git" "$ROOT/tmp/pimalaya/neverest"

# Refresh existing
git -C "$ROOT/tmp/pimalaya/neverest" pull --ff-only
```

Parse tier lists from `clone-manifest.yaml` (`A_must`, `B_core`). Default slice: both tiers (23 repos). Tier `C_adjacent` only when filling contacts/calendar cards.

## Card authoring

1. Copy sections from [CARD-TEMPLATE.md](../../docs/pimalaya-ecosystem/CARD-TEMPLATE.md).
2. Fill `evidence` from `tmp/pimalaya/<repo>/README.md`.
3. Update [SOURCES.md](../../docs/pimalaya-ecosystem/SOURCES.md) snapshot date if statuses changed.

## Wire Cursor skill

From repo root, run [BOOTSTRAP.md](../../BOOTSTRAP.md) Phase 2 (includes `pimalaya-ecosystem` symlink).
