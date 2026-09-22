---
name: should-i-read-operator
description: >-
  Operates the should-i-read host CLI against EmailOps (black box) and Polypus:
  build, doctor, sync, export, polypus check. Use when managing inbox sync from
  this repo, wiring Make targets, or agents would otherwise edit providers/emailops.
---

# should-i-read operator

**Moral:** Drive EmailOps from the host Go CLI. Do not patch the submodule. AI classification is not EmailOps `classify`/`chat`/`embed`.

Architecture: `.cursor/rules/architecture.mdc`. Setup: `docs/emailops-setup.md`. Seam: `docs/ai-provider-seam.md`.

## When to load

- Sync, doctor, list, show, or export mail from this host
- Polypus health before AI work
- Agent proposes editing `providers/emailops` AI providers

## Core constraints

**CONSTRAINT:** Product EmailOps operations MUST go through `make` / `bin/should-i-read`, not ad-hoc cargo in the submodule for host workflows.

- MUST: `make build` then `./bin/should-i-read <cmd>` (or Make aliases `doctor`, `sync`, `export`, `polypus-check`)
- MUST: set `EMAILOPS_DATA_DIR` or `--data-dir` explicitly when not using the platform default
- MUST NOT: edit files under `providers/emailops` for host needs
- MUST NOT: call EmailOps `classify`, `chat`, `embed`, or `compose --send` for product AI / mutation
- NEVER: point EmailOps OpenRouter/Ollama at Polypus as a workaround

Enforcement: planned commands are host CLI/Make only; `git -C providers/emailops status` clean
Violation: STOP, rewrite as host CLI usage

CORRECT:
```bash
make build
make polypus-check
EMAILOPS_DATA_DIR="$HOME/Library/Application Support/com.emailops.app" ./bin/should-i-read doctor
./bin/should-i-read export --limit 50 --mailbox inbox
```

PROHIBITED:
```bash
# edit providers/emailops/.../openrouter.rs
make -C providers/emailops cli-fast ARGS="classify --all --json"
```

**CONSTRAINT:** Before any host AI work, MUST fail closed on Polypus.

- MUST: `make polypus-check` or `./bin/should-i-read polypus check`
- MUST NOT: proceed with Jev/report when Polypus is down

Enforcement: probe exits 0 before AI steps
Violation: STOP, start Polypus (`make serve` in Polypus repo), re-check

## Operator recipe

1. `git submodule update --init --recursive providers/emailops`
2. `make emailops-install` (once per machine / after submodule bump)
3. `make emailops-cli` (or let `should-i-read` cargo-build on first doctor)
4. Connect an account once via EmailOps GUI or `emailops-cli accounts add` (outside host v1)
5. `make build && ./bin/should-i-read doctor`
6. `./bin/should-i-read sync` (desktop app closed)
7. `./bin/should-i-read export --limit 50`
8. `make polypus-check` before any classify/TLDR follow-up

## Pre-completion checklist

- [ ] **Provider untouched:** No writes under `providers/`
      Method: `git -C providers/emailops status`
      Pass: clean for this task
      Fail: STOP, revert nested edits
- [ ] **Host CLI used:** Commands go through `bin/should-i-read` or Make aliases
      Method: Review command history in the turn
      Pass: no product path via EmailOps AI subcommands
      Fail: STOP, switch to host CLI
