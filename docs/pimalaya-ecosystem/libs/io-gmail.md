# io-gmail

## Metadata

| id | io-gmail |
| upstream | `https://github.com/pimalaya/io-gmail` |
| clone_path | `tmp/pimalaya/io-gmail` |

## Classification

| domain | Email |
| kind | Library |
| status | early |

## maturity_notes

Gmail REST API client; Himalaya `gmail` subgroup.

## capabilities

Native Gmail API coroutines (alternative to Gmail IMAP).

## cli_surface

N/A (use **himalaya gmail**)

## machine_io

none

## config

OAuth bearer via ortie in himalaya config.

## auth_model

OAuth 2.0 bearer.

## backends_features

N/A

## depends_on

**io-http**, **io-oauth**

## go_host_fit

`skip`

## product_role

`query`

## gaps_risks

Neverest Gmail backend not implemented; prefer IMAP or Himalaya until stable.

## evidence

ecosystem page, clone README
