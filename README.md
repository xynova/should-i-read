# should-i-read

Host Go CLI for local inbox triage: **mail sync + pimdir**, **Polypus** as the only AI gateway, and report-only unwanted-mail evaluation (Jev path later).

## Layout

| Path | Role |
|------|------|
| [`cmd/should-i-read`](cmd/should-i-read) | Host Go CLI |
| [`docs/pimalaya-setup.md`](docs/pimalaya-setup.md) | Operator path (configure hub, tokens, day-2 sync) |
| [`docs/`](docs/) | Polypus seam, report-only eval, ecosystem index |
| [`config/`](config/) | Example YAML / env (no secrets) |
| [`AGENTS.md`](AGENTS.md) | Agent entry: load order into ai-copilots |
| [`ai-copilots/`](ai-copilots/) | Canonical operator skill + BOOTSTRAP |

Agents start at [`AGENTS.md`](AGENTS.md). Operator settings: `~/.config/should-i-read/config.yaml` (`make init`). Polypus: `POLYPUS_BASE_URL` / `polypus.base_url` in config (see `make config bump` / [`docs/ai-provider-seam.md`](docs/ai-provider-seam.md)).

## Quick start

```bash
make wire-ai-copilots
make build
make init
make configure                # TTY hub, or ARGS='--apply --provider gmail --email you@example.com'
./bin/should-i-read config bump   # Polypus placeholder when YAML still has localhost
./bin/should-i-read configure --json   # readiness checklist
make polypus-check

make readiness                # mail readiness (make doctor alias)
make sync                     # mail sync
make export                   # mail export → tmp/mail-export-*.json
```

Set `pimalaya.pimdir_path` in config to the directory that contains `pimdir.db`. Multi-account: `token gmail --account <label>` and matching sync-config `token.command_args`.

## Architecture

Always-on rule: [`.cursor/rules/architecture.mdc`](.cursor/rules/architecture.mdc). Mail store direction: [`docs/pimalaya-ecosystem/INDEX.md`](docs/pimalaya-ecosystem/INDEX.md). Operator skill: `should-i-read-operator`.

- Host product code is Go (`should-i-read` CLI).
- All AI goes through Polypus.
- Unwanted-mail automation stays report-only until eval exit criteria pass ([docs/report-only-eval.md](docs/report-only-eval.md)).

## Milestone boundary

Report-only pipeline: mail export plus Polypus; Jev clustering and TLDR are the next slice after `polypus-check` passes.
