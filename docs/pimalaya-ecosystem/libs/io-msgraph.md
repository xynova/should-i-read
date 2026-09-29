# io-msgraph

## Metadata

| id | io-msgraph |
| upstream | `https://github.com/pimalaya/io-msgraph` |
| clone_path | `tmp/pimalaya/io-msgraph` |

## Classification

| domain | Email + Contacts |
| kind | Library |
| status | early |

## maturity_notes

Microsoft Graph mail and contacts; Neverest `msgraph` feature not default.

## capabilities

Graph API coroutines for mail folders and messages.

## cli_surface

N/A (use **himalaya msgraph**)

## machine_io

none

## config

OAuth bearer in app TOML.

## auth_model

OAuth 2.0 bearer.

## backends_features

Neverest: `msgraph` feature (mail-only while incomplete).

## depends_on

**io-http**

## go_host_fit

`skip`

## product_role

`query`

## gaps_risks

Incomplete sync story on Neverest; verify before architecture commits.

## evidence

ecosystem page, `tmp/pimalaya/io-msgraph/README.md`, neverest README tip
