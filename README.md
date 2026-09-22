# should-i-read

Host project for local inbox triage: EmailOps for mail (black box), Polypus as the only AI gateway, and a later Jev decision path for unwanted-mail filtering.

## Layout

| Path | Role |
|------|------|
| [`cmd/should-i-read`](cmd/should-i-read) | Host Go CLI |
| [`providers/emailops`](providers/emailops) | Git submodule: [emailops/emailops](https://github.com/emailops/emailops) |
| [`docs/`](docs/) | Setup, provider seam, report-only evaluation |
| [`config/`](config/) | Example env (no secrets) |
| [`.cursor/skills/should-i-read-operator`](.cursor/skills/should-i-read-operator) | Operator skill for agents |

Polypus lives outside this tree (typical checkout: `~/Xynova/ai/polypus`). Clients call only `http://127.0.0.1:1320`.

## Quick start

```bash
git submodule update --init --recursive
make emailops-install    # once: Node deps inside submodule
make emailops-cli        # build emailops-cli without llama.cpp
make build
make polypus-check       # Polypus must be up (make serve in Polypus repo)

# Platform default data dir, or set EMAILOPS_DATA_DIR / --data-dir
./bin/should-i-read doctor
./bin/should-i-read sync
./bin/should-i-read export --limit 50
```

Connect a mailbox once via the EmailOps desktop app or upstream `emailops-cli accounts add`. Do not use EmailOps `classify` / `chat` / `embed` for product AI on this host.

## Architecture

Always-on rule: [`.cursor/rules/architecture.mdc`](.cursor/rules/architecture.mdc). Operator skill: `should-i-read-operator`.

- Use EmailOps as a black box; do not patch `providers/emailops`.
- Host product code is Go (`should-i-read` CLI).
- All AI goes through Polypus.
- Unwanted-mail automation stays report-only until eval exit criteria pass ([docs/report-only-eval.md](docs/report-only-eval.md)).

## Milestone boundary

Mail sync and JSON export run from this host. Jev clustering and TLDR are the next slice after export + Polypus check.
