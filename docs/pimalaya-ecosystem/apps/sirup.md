# sirup

## Metadata

| Field | Value |
|-------|-------|
| id | sirup |
| upstream | `https://github.com/pimalaya/sirup` |
| clone_path | `tmp/pimalaya/sirup` |

## Classification

| Field | Value |
|-------|-------|
| domain | Plumbing |
| kind | CLI |
| status | early |

## maturity_notes

Early but documented for Himalaya session amortisation.

## capabilities

- Pre-authenticated IMAP, SMTP, ManageSieve sessions on Unix sockets
- `sirup start` serves protocols from one process; configure wizard
- REPL for raw protocol testing

## cli_surface

`start`, `configure`, REPL; see README Usage.

## machine_io

`--json` on supported commands (README Features).

## config

Sirup account TOML (see clone README); Himalaya points `imap.server` / `smtp.server` at socket URLs.

## auth_model

Same SASL set as Himalaya (incl. oauthbearer, xoauth2).

## backends_features

Protocol and TLS gated by Cargo features.

## depends_on

**io-imap**, **io-smtp**, **io-managesieve** (via features).

## go_host_fit

`unix_socket` (daemon) + `cli_json` (control plane)

## product_role

`query` (performance plumbing for Himalaya, optional)

## gaps_risks

Extra moving part: host must manage sirup lifecycle if used.

## evidence

`tmp/pimalaya/sirup/README.md`
