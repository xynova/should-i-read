# pimdir

## Metadata

| Field | Value |
|-------|-------|
| id | pimdir |
| upstream | `https://github.com/pimalaya/pimdir` |
| clone_path | `tmp/pimalaya/pimdir` |

## Classification

| Field | Value |
|-------|-------|
| domain | Store |
| kind | Spec |
| status | draft-01 (ecosystem: store standard) |

## maturity_notes

Normative parts are **draft-01** (`STORAGE.md`, `SYNC.md`, `SEARCH.md`). Schema version 1 is stable in shape; breaking changes fold into migrations after freeze. Search reference implementation is not complete in the ecosystem.

## capabilities

- Local-first PIM store: **SQLite** index + **content-addressed blob** directory for bodies
- Cross-domain: mail, contacts, events, tasks, journals in one store; multi-account
- Soft delete / retention; action queue for non-owner writers; change feed for derived indexes
- SYNC and SEARCH layers specified; readers/producers can be any language with SQLite bindings
- Test vectors and migrations under `migrations/`, `queries/`, `vectors/`

## cli_surface

N/A (spec repo). Operator tooling lives in **io-pimdir** (`pimdir` binary feature).

## machine_io

N/A at spec level.

## config

Store path is chosen by apps (Neverest account store, io-pimdir CLI).

## auth_model

N/A (local store).

## backends_features

N/A.

## depends_on

Consumed by **io-pimdir**, **neverest** (pimdir replica).

## go_host_fit

`read_store_sqlite` (future reader profile against STORAGE.md; no FFI required for read-only projections).

## product_role

`spec` (canonical local mail/PIM database for should-i-read architecture).

## gaps_risks

Draft status: recreate stores across draft bumps until freeze. SEARCH layer not fully implemented upstream.

## evidence

`tmp/pimalaya/pimdir/README.md`, `OVERVIEW.md`, `STORAGE.md`
