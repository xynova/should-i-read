# neverest

## Metadata

| Field | Value |
|-------|-------|
| id | neverest |
| upstream | `https://github.com/pimalaya/neverest` |
| clone_path | `tmp/pimalaya/neverest` |

## Classification

| Field | Value |
|-------|-------|
| domain | Email (+ PIM collections via features) |
| kind | CLI |
| status | beta |

## maturity_notes

`v0.x`: breaking changes until stabilisation. v1 installer not released yet; use CI artifacts or `cargo install --git`.

## capabilities

- Sync PIM collections (mail via IMAP default; contacts/calendar need `dav` feature)
- Local **pimdir** store as offline replica per account
- Retention, relay mode, queued submission, conflict merge and resolve
- Discovery wizard; multi-account TOML config
- Backup pattern: read-only IMAP + no purge retains bodies after remote expunge

## cli_surface

`init`, `sync`, `check`, `configure`, `conflict list|show|resolve`, `json-schema`; flags `--json`, `--dry-run`, `--reset`, collection filters.

## machine_io

`--json` on supported commands; `neverest json-schema` describes payloads. Exit **2** when human decision needed (conflicts, duplicates, blocked writes).

## config

`$XDG_CONFIG_HOME/neverest/config.toml`, `~/.config/neverest/config.toml`, `~/.neverestrc`; override `-c` / `NEVEREST_CONFIG` (colon-merge for multiple files).

## auth_model

IMAP/SMTP/DAV: anonymous, login, plain, oauthbearer, xoauth2, scram-sha-256; basic/bearer for DAV.

## backends_features

Default features in Cargo.toml; `dav` for CardDAV/CalDAV; `msgraph` not default (mail-only while incomplete). **JMAP and Gmail sources configure but have no backend yet** (README tip).

## depends_on

**pimdir** / **io-pimdir**, **io-replica** engine (org); discovery via io-pim-discovery patterns.

## go_host_fit

`cli_json`

## product_role

`ingest` (primary sync into local store).

## gaps_risks

No Maildir-as-sync-source; migrate via server resync or io-pimdir conversion. Gmail/JMAP backends missing. Not v1 release.

## evidence

`tmp/pimalaya/neverest/README.md`, `config.sample.toml`
