# io-managesieve

## Metadata

| id | io-managesieve |
| upstream | `https://github.com/pimalaya/io-managesieve` |
| clone_path | `tmp/pimalaya/io-managesieve` |

## Classification

| domain | Email |
| kind | Library |
| status | stable (protocol family) |

## maturity_notes

ManageSieve client; Himalaya `sieve` subgroup and Sirup sieve sockets.

## capabilities

Sieve script management over ManageSieve (RFC 5804).

## cli_surface

N/A (use **himalaya sieve**)

## machine_io

none

## config

Optional `sieve` block in himalaya/sirup account TOML.

## auth_model

Password or bearer per connection security rules.

## backends_features

N/A

## depends_on

**stream**, TLS crates

## go_host_fit

`skip`

## product_role

`out_of_scope` for should-i-read v1

## gaps_risks

Filter management is not core to unwanted-mail report path.

## evidence

himalaya README sieve section, clone README
