# io-imap

## Metadata

| id | io-imap |
| upstream | `https://github.com/pimalaya/io-imap` |
| clone_path | `tmp/pimalaya/io-imap` |

## Classification

| domain | Email |
| kind | Library |
| status | stable |

## maturity_notes

I/O-free IMAP client; foundation for Himalaya, Neverest, Carillon, Sirup.

## capabilities

Full IMAP protocol as coroutines; host brings sockets via stream.

## cli_surface

N/A (use **himalaya** / **neverest**)

## machine_io

none

## config

N/A

## auth_model

SASL mechanisms at app config.

## backends_features

N/A

## depends_on

**stream**, **io-sasl**, **io-starttls**

## go_host_fit

`skip`

## product_role

`out_of_scope` (orchestrate CLIs)

## gaps_risks

N/A

## evidence

ecosystem page, `tmp/pimalaya/io-imap/README.md`
