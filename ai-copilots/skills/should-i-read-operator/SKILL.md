---
name: should-i-read-operator
description: >-
  Operates the should-i-read host CLI: Pimalaya default (Neverest/pimdir, pim
  doctor/sync/snapshot, token gmail|outlook), Polypus check, init/setup/config.
  EmailOps legacy via make emailops-* only when needed.
  Use when managing inbox sync from this repo, wiring Make targets, guiding
  first-time setup or multi-account Neverest labels, or agents would otherwise
  edit providers/emailops.
---

# should-i-read operator

**Moral:** Drive the host Go CLI and XDG config. Default mail lane is **Pimalaya** (Neverest + pimdir + token brokers). EmailOps is optional legacy (`make emailops-*`). Do not patch the submodule. Product AI is not EmailOps `classify`/`chat`/`embed`.

Architecture: `.cursor/rules/architecture.mdc`. **Default setup:** [`docs/pimalaya-setup.md`](../../docs/pimalaya-setup.md). Legacy EmailOps: [`docs/emailops-setup.md`](../../docs/emailops-setup.md). Polypus: [`docs/ai-provider-seam.md`](../../docs/ai-provider-seam.md). Operator config: skill `operator-config`. Entry: [`AGENTS.md`](../../AGENTS.md).

## When to load

- First-time operator setup (init, setup, token login, Neverest, pimdir path)
- Polypus health before AI work
- Pimalaya `pim doctor|sync|snapshot`, token brokers, multi-account `--account` labels
- Legacy EmailOps UI/sync/export only when operator explicitly uses that path
- Agent proposes editing `providers/emailops` AI providers

## First-time setup (Pimalaya default)

**CONSTRAINT:** Before `make doctor` / `make sync` on a new machine, MUST complete this sequence (or confirm it already ran).

1. `make build && make init` → `~/.config/should-i-read/config.yaml` (mode `0600`)
2. `make setup` for OAuth **client** credentials (Gmail/Outlook apps):
   - Product-owned default when env/keyring/embeds resolve; else BYO or guided DIY
   - MUST NOT put secret literals in YAML; `${EMAILOPS_*}` placeholders + `secrets:` list
3. `./bin/should-i-read config bump` when `polypus.base_url` is still literal `http://127.0.0.1:1320`
4. Set `POLYPUS_BASE_URL` (env or keyring) so `${POLYPUS_BASE_URL}` expands; `make polypus-check`
5. Install Neverest; `neverest init`; wire `token.command` → `should-i-read token gmail` ([`docs/pimalaya-setup.md`](../../docs/pimalaya-setup.md))
6. `./bin/should-i-read token gmail login` (per `--account` label for multi-inbox Neverest TOML)
7. Set `pimalaya.pimdir_path` in config to the store directory containing `pimdir.db`
8. `make doctor` → `make sync` → `make export` (pim snapshot)

Inspect: `./bin/should-i-read config path` / `config show` (secrets as `(set)` / `(unset)`).

**Trust:** OAuth client ids identify the app; mailbox tokens live in host keyring (`GMAIL_OAUTH_TOKEN*`, `OUTLOOK_MSAL_CACHE*`) for the token brokers, not in YAML.

## Multi-account (Neverest + token labels)

**CONSTRAINT:** One OAuth **client** per provider; each Neverest account block uses `token.command_args` with `--account <label>`.

- MUST: one `EMAILOPS_GMAIL_CLIENT_ID` / optional secret for all Gmail mailboxes
- MUST: `token gmail login --account <label>` once per label
- MUST NOT: put refresh tokens or MSAL blobs in `config.yaml`

CORRECT:
```bash
should-i-read token gmail login --account work
# Neverest TOML: token.command_args = ["token", "gmail", "--account", "work"]
```

## EmailOps legacy (optional)

Use only when the operator still wants EmailOps SQLite + desktop UI.

- `make emailops-install` / `make emailops-cli` (submodule)
- `make emailops-ui` → add accounts; `make emailops-sync` / `make emailops-export`
- MUST NOT: use EmailOps AI subcommands for host product AI

## Core constraints

**CONSTRAINT:** Host workflows MUST use `make` / `bin/should-i-read` or documented Make aliases.

- MUST (default): `make doctor|sync|export` → pim lane; `make polypus-check` before AI
- MUST (legacy): `make emailops-*` for EmailOps custody only
- MUST NOT: edit `providers/emailops` for host needs
- MUST NOT: call EmailOps `classify`, `chat`, `embed` for product AI

CORRECT:
```bash
make init && make setup
make polypus-check
make doctor && make sync && make export
```

PROHIBITED:
```bash
make -C providers/emailops cli-fast ARGS="classify --all --json"
# Edit providers/emailops for Polypus injection
```

**CONSTRAINT:** Before host AI work, MUST fail closed on Polypus (`make polypus-check`).

## Operator recipe (default)

1. `make wire-ai-copilots` (Cursor skills)
2. `make build && make init && make setup`
3. `config bump` + `polypus-check` as needed
4. Neverest + `token gmail login` (+ `pimdir_path` in config)
5. `make doctor` → `make sync` → `make export`
6. Report-only eval pipeline (future): snapshot + Polypus ([`docs/report-only-eval.md`](../../docs/report-only-eval.md))

## Pre-completion checklist

- [ ] **Provider untouched:** `git -C providers/emailops status` clean for host tasks
- [ ] **Default lane:** Pimalaya commands used unless operator chose EmailOps legacy
- [ ] **Secrets out of YAML:** `config show` uses `(set)` / `(unset)`; no literals
- [ ] **Polypus:** `polypus check` passes before AI steps
