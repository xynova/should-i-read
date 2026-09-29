# m2m

## Metadata

| Field | Value |
|-------|-------|
| id | m2m |
| upstream | `https://github.com/pimalaya/m2m` |
| clone_path | `tmp/pimalaya/m2m` |

## Classification

| Field | Value |
|-------|-------|
| domain | Email |
| kind | CLI |
| status | in_development |

## maturity_notes

`v0.x` active development; breaking changes until stabilization.

## capabilities

- Convert Maildir, Maildir++, m2dir filesystem stores
- Parallel workers, dry-run, skip-existing dedup
- Keyword round-trip between Maildir-family layouts

## cli_surface

`maildir`, `maildir++`, `m2dir` subcommands and conversion flags (see README Usage).

## machine_io

Primarily human-oriented logs; verify `--json` in clone if added.

## config

CLI flags only.

## auth_model

N/A (local filesystem).

## backends_features

Filesystem backends only.

## depends_on

**io-maildir**, **io-m2dir** concepts.

## go_host_fit

`cli_text`

## product_role

`convert` (migration/interop; not canonical product DB)

## gaps_risks

pimdir is canonical store; m2m is for legacy Maildir trees only.

## evidence

`tmp/pimalaya/m2m/README.md`
