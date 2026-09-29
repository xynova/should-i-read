# ortie

## Metadata

| Field | Value |
|-------|-------|
| id | ortie |
| upstream | `https://github.com/pimalaya/ortie` |
| clone_path | `tmp/pimalaya/ortie` |

## Classification

| Field | Value |
|-------|-------|
| domain | Auth |
| kind | CLI |
| status | stable |

## maturity_notes

Stable plumbing for OAuth 2.0 across Pimalaya CLIs.

## capabilities

- Provider wizard; dynamic client registration
- Authorization code, device, client credentials grants; PKCE; token refresh
- Token storage via user shell commands; persistent unlock session
- Hooks on issuance/refresh

## cli_surface

Account/token subcommands (see `ortie --help`); data commands support `--json`.

## machine_io

`--json` on every data command; `ortie json-schema`.

## config

XDG ortie config (see clone README Configuration).

## auth_model

OAuth 2.0; tokens exposed to Himalaya/Neverest via `*.command` or bearer config.

## backends_features

TLS via rustls (ring/aws) or native-tls features.

## depends_on

**io-oauth** library.

## go_host_fit

`cli_json`

## product_role

`auth`

## gaps_risks

Host must not commit tokens; use operator-config / keyring and command hooks.

## evidence

`tmp/pimalaya/ortie/README.md`
