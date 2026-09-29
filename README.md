# should-i-read

Host project for local inbox triage: EmailOps for mail (black box), Polypus as the only AI gateway, and a later Jev decision path for unwanted-mail filtering.

## Layout

| Path | Role |
|------|------|
| [`cmd/should-i-read`](cmd/should-i-read) | Host Go CLI |
| [`providers/emailops`](providers/emailops) | Git submodule: [emailops/emailops](https://github.com/emailops/emailops) |
| [`docs/`](docs/) | Setup, provider seam, report-only evaluation |
| [`config/`](config/) | Example YAML / env (no secrets) |
| [`AGENTS.md`](AGENTS.md) | Agent entry: load order into ai-copilots |
| [`ai-copilots/`](ai-copilots/) | Canonical operator skill + BOOTSTRAP |

Agents start at [`AGENTS.md`](AGENTS.md). Operator settings live in `~/.config/should-i-read/config.yaml` (see `make init`). Polypus lives outside this tree (typical checkout: `~/Xynova/ai/polypus`). Clients call only `http://127.0.0.1:1320`.

## Quick start

```bash
git submodule update --init --recursive
make emailops-install    # once: Node deps inside submodule
make emailops-cli        # build emailops-cli without llama.cpp
make build
make init                # ~/.config/should-i-read + default data dir
make setup               # product OAuth if present; else BYO / guided DIY
make polypus-check       # Polypus must be up (make serve in Polypus repo)

./bin/should-i-read doctor
./bin/should-i-read sync
./bin/should-i-read export --limit 50
# optional: make ui  (EmailOps desktop with config-injected env)
```

Connect a mailbox once via `make ui` or upstream `emailops-cli accounts add`. Prefer `make setup` for OAuth *client* credentials (product-owned default, BYO override). Do not put secrets in YAML. Do not use EmailOps `classify` / `chat` / `embed` for product AI on this host.

## Architecture

Always-on rule: [`.cursor/rules/architecture.mdc`](.cursor/rules/architecture.mdc). Agents: [`AGENTS.md`](AGENTS.md). Operator skill: `should-i-read-operator`. Mail-store direction (Pimalaya / pimdir): [`docs/pimalaya-ecosystem/INDEX.md`](docs/pimalaya-ecosystem/INDEX.md). Operator setup: [`docs/pimalaya-setup.md`](docs/pimalaya-setup.md).

- Use EmailOps as a black box; do not patch `providers/emailops`.
- Host product code is Go (`should-i-read` CLI).
- All AI goes through Polypus.
- Unwanted-mail automation stays report-only until eval exit criteria pass ([docs/report-only-eval.md](docs/report-only-eval.md)).

## Milestone boundary

Mail sync and JSON export run from this host. Jev clustering and TLDR are the next slice after export + Polypus check.
