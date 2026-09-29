# mml

## Metadata

| Field | Value |
|-------|-------|
| id | mml |
| upstream | `https://github.com/pimalaya/mml` |
| clone_path | `tmp/pimalaya/mml` |

## Classification

| Field | Value |
|-------|-------|
| domain | Email |
| kind | Library + CLI |
| status | stable |

## maturity_notes

Stable MIME markup (Emacs-style MML).

## capabilities

- Compose MIME messages as human-editable markup
- Library + CLI for conversion to RFC 5322 messages

## cli_surface

MML CLI subcommands (see clone README).

## machine_io

Check clone for `--json` on CLI; primarily text/markup.

## config

N/A for library; CLI flags per README.

## auth_model

N/A (compose only; send via Himalaya SMTP).

## backends_features

N/A.

## depends_on

Used by **himalaya** compose flows.

## go_host_fit

`cli_text` (invoke mml CLI when host generates outbound mail later)

## product_role

`compose`

## gaps_risks

Out of unwanted-mail report-only path until send is in scope.

## evidence

`tmp/pimalaya/mml/README.md`
