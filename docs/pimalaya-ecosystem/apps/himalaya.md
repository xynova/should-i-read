# himalaya

## Metadata

| Field | Value |
|-------|-------|
| id | himalaya |
| upstream | `https://github.com/pimalaya/himalaya` |
| clone_path | `tmp/pimalaya/himalaya` |

## Classification

| Field | Value |
|-------|-------|
| domain | Email |
| kind | CLI |
| status | stable |

## maturity_notes

Himalaya v2 is the mature mail CLI in the ecosystem. Cargo features gate backends.

## capabilities

- Backend-agnostic mail ops: list/read/search/move/flags/attachments
- Protocol-specific sub-APIs: IMAP, JMAP, Gmail REST, Microsoft Graph, SMTP, ManageSieve
- Discovery wizard (PACC, Thunderbird autoconfig, SRV, JMAP)
- MML compose integration; multi-account TOML

## cli_surface

Shared: `mailbox`, `envelope list|search`, `flag`, `message read|copy|…`, `attachment`. Native: `imap`, `jmap`, `gmail`, `msgraph`, `smtp`, `sieve` subgroups. Account: `account list`, `account check`.

## machine_io

`--json` documented in README Features.

## config

`$XDG_CONFIG_HOME/himalaya/config.toml`, `~/.config/himalaya/config.toml`, `~/.himalaya.toml`; shared with himalaya-tui.

## auth_model

Password commands, OAuth bearer via `ortie` token commands in TOML for Gmail/Graph.

## backends_features

IMAP, SMTP, JMAP, Gmail, MS Graph, ManageSieve per Cargo.toml default set.

## depends_on

**mml**, **io-*** crates, **sirup** (optional session reuse), **ortie** (OAuth).

## go_host_fit

`cli_json`

## product_role

`query` (live remote or backend ops when store index is insufficient).

## gaps_risks

Each invocation opens fresh TCP+TLS+SASL unless **sirup** used. Not the canonical local store (use neverest + pimdir for replica).

## evidence

`tmp/pimalaya/himalaya/README.md`, `config.sample.toml`
