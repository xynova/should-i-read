# carillon

## Metadata

| Field | Value |
|-------|-------|
| id | carillon |
| upstream | `https://github.com/pimalaya/carillon` |
| clone_path | `tmp/pimalaya/carillon` |

## Classification

| Field | Value |
|-------|-------|
| domain | Watch |
| kind | CLI |
| status | early |

## maturity_notes

`v0.x` breaking changes. Replaces retiring **mirador** watch story. Pre-release binaries via CI artifacts.

## capabilities

- Watch mail (IMAP IDLE, JMAP push, Maildir poll) and DAV collections
- Hooks: shell command and/or desktop notification on change
- Multi-account; discovery wizard `carillon configure`

## cli_surface

Watch/run/configure commands (see README Usage).

## machine_io

Check clone README for `--json` on scriptable commands.

## config

Carillon TOML (see clone README Configuration).

## auth_model

IMAP SASL; HTTP basic/bearer for JMAP/DAV.

## backends_features

Cargo features gate backends (IMAP, JMAP, maildir, dav).

## depends_on

**io-imap**, **io-jmap**, **io-http**, discovery crates.

## go_host_fit

`cli_json` or `cli_text` (host triggers sync on hook later)

## product_role

`watch`

## gaps_risks

Not stable release; host should not hard-depend until watch + neverest sync story is designed.

## evidence

`tmp/pimalaya/carillon/README.md`
