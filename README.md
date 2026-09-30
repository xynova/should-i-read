# should-i-read

Host Go CLI for local inbox triage: **Neverest + pimdir** (default mail lane), **Polypus** as the only AI gateway, and report-only unwanted-mail evaluation (Jev path later).

## Layout

| Path | Role |
|------|------|
| [`cmd/should-i-read`](cmd/should-i-read) | Host Go CLI |
| [`docs/pimalaya-setup.md`](docs/pimalaya-setup.md) | Default operator path (Neverest, tokens, pim) |
| [`providers/emailops`](providers/emailops) | Optional legacy submodule ([emailops/emailops](https://github.com/emailops/emailops)) |
| [`docs/`](docs/) | Polypus seam, report-only eval, ecosystem index |
| [`config/`](config/) | Example YAML / env (no secrets) |
| [`AGENTS.md`](AGENTS.md) | Agent entry: load order into ai-copilots |
| [`ai-copilots/`](ai-copilots/) | Canonical operator skill + BOOTSTRAP |

Agents start at [`AGENTS.md`](AGENTS.md). Operator settings: `~/.config/should-i-read/config.yaml` (`make init`). Polypus: `POLYPUS_BASE_URL` / `polypus.base_url` in config (see `make config bump` / [`docs/ai-provider-seam.md`](docs/ai-provider-seam.md)).

## Quick start (Pimalaya default)

```bash
git submodule update --init --recursive   # optional: only for EmailOps legacy
make wire-ai-copilots
make build
make init
make setup                    # OAuth *client* ids (Gmail/Outlook apps) into keyring
./bin/should-i-read config bump   # Polypus placeholder when YAML still has localhost

# Neverest: install on PATH, neverest init, wire token.command (see docs/pimalaya-setup.md)
./bin/should-i-read token gmail login
make polypus-check

make doctor                   # pim doctor (Neverest check)
make sync                     # pim sync
make export                   # pim snapshot → tmp/pim-snapshot-*.json
```

Set `pimalaya.pimdir_path` in config to the directory that contains `pimdir.db`. Multi-account: `token gmail --account <label>` and matching Neverest `token.command_args`.

Do not use EmailOps `classify` / `chat` / `embed` for product AI on this host.

## EmailOps legacy (optional)

Custody via EmailOps SQLite + desktop UI is still available but not the default path.

```bash
make emailops-install && make emailops-cli
make emailops-ui              # add mailboxes in the app
make emailops-doctor
make emailops-sync
make emailops-export
```

See [`docs/emailops-setup.md`](docs/emailops-setup.md).

## Architecture

Always-on rule: [`.cursor/rules/architecture.mdc`](.cursor/rules/architecture.mdc). Mail store direction: [`docs/pimalaya-ecosystem/INDEX.md`](docs/pimalaya-ecosystem/INDEX.md). Operator skill: `should-i-read-operator`.

- Host product code is Go (`should-i-read` CLI).
- Do not patch `providers/emailops` for host needs.
- All AI goes through Polypus.
- Unwanted-mail automation stays report-only until eval exit criteria pass ([docs/report-only-eval.md](docs/report-only-eval.md)).

## Milestone boundary

Report-only pipeline: pim snapshot (or legacy export) plus Polypus; Jev clustering and TLDR are the next slice after `polypus-check` passes.
