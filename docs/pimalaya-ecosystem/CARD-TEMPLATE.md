# Card template

Every file under `store/`, `libs/`, and `apps/` MUST include these sections (use the headings below).

## Metadata

| Field | Value |
|-------|-------|
| id | |
| upstream | `https://github.com/pimalaya/<name>` |
| clone_path | `tmp/pimalaya/<name>` or `none` |

## Classification

| Field | Value |
|-------|-------|
| domain | Email / Auth / Transport / Contacts / Calendar / Plumbing / Store / Watch / Time |
| kind | Library / Library+CLI / CLI / Spec |
| status | stable / beta / early / in_development / retiring / frozen / deprecated |

## maturity_notes

Short paragraph on breaking changes or draft status.

## capabilities

- Bullet list

## cli_surface

Subcommands worth wrapping, or `N/A` for pure libraries.

## machine_io

Document `--json`, `json-schema`, Unix sockets, or `none` with evidence.

## config

XDG paths, env overrides (`NEVEREST_CONFIG`, etc.).

## auth_model

Password commands, OAuth bearer, Ortie integration, keyring.

## backends_features

Cargo features and protocol backends.

## depends_on

Other Pimalaya crates or CLIs.

## go_host_fit

One of: `cli_json`, `cli_text`, `unix_socket`, `read_store_sqlite`, `ffi_later`, `skip`.

## product_role

One of: `ingest`, `query`, `auth`, `watch`, `compose`, `convert`, `spec`, `out_of_scope`.

## gaps_risks

Honest limits for architecture planning.

## evidence

Paths in `tmp/pimalaya/<name>/` used to write this card (for example `README.md`).
