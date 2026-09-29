# io-pimdir

## Metadata

| Field | Value |
|-------|-------|
| id | io-pimdir |
| upstream | `https://github.com/pimalaya/io-pimdir` |
| clone_path | `tmp/pimalaya/io-pimdir` |

## Classification

| Field | Value |
|-------|-------|
| domain | Store |
| kind | Library (+ optional CLI via `cli` feature) |
| status | early (implements draft-01 pimdir) |

## maturity_notes

Three layers: I/O-free core (always), std **client** (reader/producer/owner handles), **CLI** (`pimdir` binary). Tied to pimdir draft-01.

## capabilities

- Reference implementation of pimdir store + sync engine
- Owner/producer/reader profiles; five sync verbs; connector seam for IMAP/JMAP/CardDAV/CalDAV
- Operator CLI: inspect store, trash/restore/purge, queue, consistency checks
- Conformance vectors in test suite

## cli_surface

`pimdir` binary (feature `cli`): store maintenance while sync runs (see clone README Usage).

## machine_io

CLI-oriented; check clone `--help` for JSON on subcommands when wiring host tools.

## config

Store path passed to CLI / client open calls.

## auth_model

Via connectors (remote sources), not store-local.

## backends_features

Cargo features: `client`, `cli`, connector backends per Cargo.toml.

## depends_on

**pimdir** spec; used by **neverest** sync stack.

## go_host_fit

`ffi_later` or invoke **pimdir** CLI if allow-listed; prefer **neverest** for ingest in v1.

## product_role

`spec` / `ingest` (engine behind Neverest).

## gaps_risks

Host should not embed Rust in v1; treat as documentation of store behavior and optional ops CLI.

## evidence

`tmp/pimalaya/io-pimdir/README.md`
