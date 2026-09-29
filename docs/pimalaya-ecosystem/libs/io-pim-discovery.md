# io-pim-discovery

## Metadata

| id | io-pim-discovery |
| upstream | `https://github.com/pimalaya/io-pim-discovery` |
| clone_path | `tmp/pimalaya/io-pim-discovery` |

## Classification

| domain | Plumbing |
| kind | Library + CLI (CLI optional feature) |
| status | early |

## maturity_notes

CLI ships as off-by-default cargo feature; wizards embed library.

## capabilities

Discover IMAP, SMTP, CardDAV, CalDAV from email address (PACC, Thunderbird autoconfig, SRV, DAV).

## cli_surface

Optional discovery CLI binary.

## machine_io

Verify in clone if CLI exposes `--json`.

## config

N/A for library.

## auth_model

N/A (discovery only).

## backends_features

CLI feature flag.

## depends_on

DNS/HTTP discovery stack.

## go_host_fit

`skip` (wizards in neverest/himalaya/carillon)

## product_role

`out_of_scope`

## gaps_risks

Host relies on tool wizards, not direct discovery in v1.

## evidence

ecosystem page, `tmp/pimalaya/io-pim-discovery/README.md`
