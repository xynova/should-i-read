# stream

## Metadata

| id | stream |
| upstream | `https://github.com/pimalaya/stream` |
| clone_path | `tmp/pimalaya/stream` |

## Classification

| domain | Transport |
| kind | Library |
| status | stable |

## maturity_notes

Standard I/O connectors that drive I/O-free crates.

## capabilities

Socket/stream connectors for sans-I/O coroutines.

## cli_surface

N/A

## machine_io

none

## config

N/A

## auth_model

N/A

## backends_features

N/A

## depends_on

Used by apps wiring **io-imap**, **io-smtp**, etc.

## go_host_fit

`skip` (use CLIs in v1)

## product_role

`out_of_scope` for host v1

## gaps_risks

Embedding requires Rust runtime.

## evidence

`tmp/pimalaya/stream/README.md`, ecosystem page
