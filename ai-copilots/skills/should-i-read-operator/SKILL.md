---
name: should-i-read-operator
description: >-
  Operates the should-i-read host CLI against EmailOps (black box) and Polypus:
  init, setup, config, secret, ui, build, doctor, sync, export, polypus check.
  Use when managing inbox sync from this repo, wiring Make targets, guiding
  first-time setup or multi-Gmail accounts, or agents would otherwise edit
  providers/emailops.
---

# should-i-read operator

**Moral:** Drive EmailOps from the host Go CLI and host XDG config. Do not patch the submodule. AI classification is not EmailOps `classify`/`chat`/`embed`.

Architecture: `.cursor/rules/architecture.mdc`. Setup: `docs/emailops-setup.md`. Product OAuth: `docs/oauth-product-apps.md`. Seam: `docs/ai-provider-seam.md`. Operator config skill: `operator-config`. Entry: `AGENTS.md`.

## When to load

- First-time operator setup (init, setup, keyring secrets, connect mailboxes)
- Product-owned vs BYO OAuth client questions
- Init / inspect host config or keyring-backed secrets
- Sync, doctor, list, show, export, or launch EmailOps UI from this host
- Multi-account questions (several Gmail/Outlook inboxes)
- Polypus health before AI work
- Agent proposes editing `providers/emailops` AI providers

## First-time setup

**CONSTRAINT:** Before doctor/sync/ui on a new machine, MUST complete this sequence (or confirm it already ran).

1. `make build && make init` → `~/.config/should-i-read/config.yaml` (mode `0600`) + default data dir
2. `make setup` (preferred) for OAuth *client* credentials:
   - **Product-owned default:** if env/keyring/release embeds already resolve, setup reports `(set)` and skips paste
   - **BYO:** paste org-provided client ids into platform keyring or print export hints when store fails
   - **Guided DIY / gcloud:** advanced only; gcloud fail-closes to guided Console/Entra docs
   - MUST NOT put secret literals in YAML; config keeps `${EMAILOPS_*}` placeholders
   - Manual alternative: `./bin/should-i-read secret set EMAILOPS_GMAIL_CLIENT_ID --stdin` (and optional `…_SECRET`; Outlook id)
3. Add each mailbox via `make ui` (or upstream `emailops-cli accounts add`): one Gmail OAuth *client* for all Gmail inboxes; each address is a separate EmailOps account
4. `./bin/should-i-read accounts` to verify; set `emailops.default_account` or pass `--account` when scoping sync/emails/export
5. `./bin/should-i-read doctor` → `sync` (desktop closed) → `export`; `make polypus-check` before any AI work

Inspect: `./bin/should-i-read config path` / `config show` (secrets as `(set)` / `(unset)`).

**Trust (say plainly):** mail and tokens stay on this computer; the OAuth client id only identifies which app is asking; product does not operate a mail server in this design; BYO replaces the client id without losing local mailbox custody.

## Multi-account

**CONSTRAINT:** Multiple inboxes share one OAuth client credential set per provider; each mailbox is its own EmailOps account record.

- MUST: one `EMAILOPS_GMAIL_CLIENT_ID` / optional `EMAILOPS_GMAIL_CLIENT_SECRET` (keyring, env, SOPS, or product embed) for all Gmail accounts
- MUST: add each Gmail address as a separate account in EmailOps (UI or `accounts add`)
- MUST: keep mailbox OAuth *tokens* in EmailOps' own keychain; host keyring holds OAuth *client* credentials only
- MUST: use `--account` / `emailops.default_account` when the operator wants one inbox; omit for all-accounts behavior where the CLI supports it
- MUST NOT: invent a second host config.yaml per mailbox
- MUST NOT: put per-mailbox refresh tokens in `~/.config/should-i-read/config.yaml`

Enforcement: `accounts` lists N rows; `config show` shows one client id `(set)` / `(unset)`
Violation: STOP, explain one-client-N-accounts, do not fork EmailOps

CORRECT:
```bash
make init
make setup   # product embeds, or BYO paste
make ui      # add alice@…, then bob@…, then …
./bin/should-i-read accounts
./bin/should-i-read sync --account alice@example.com
```

PROHIBITED:
```bash
# Four config.yaml files, one per Gmail
# gmail_client_secret: "literal-in-yaml"
# Edit providers/emailops to hard-code four accounts
```

## Core constraints

**CONSTRAINT:** Product EmailOps operations MUST go through `make` / `bin/should-i-read`, not ad-hoc cargo in the submodule for host workflows.

- MUST: `make build` then `./bin/should-i-read <cmd>` (or Make aliases `init`, `setup`, `ui`, `doctor`, `sync`, `export`, `polypus-check`)
- MUST: use host config (`make init` → `~/.config/should-i-read/config.yaml`); override with `--config` / `SHOULD_I_READ_CONFIG`
- MUST NOT: put secret literals in YAML; use `${EMAILOPS_*}`, `make setup` / `secret set`, env, or product release embeds
- MUST NOT: edit files under `providers/emailops` for host needs
- MUST NOT: call EmailOps `classify`, `chat`, `embed`, or `compose --send` for product AI / mutation
- NEVER: point EmailOps OpenRouter/Ollama at Polypus as a workaround

Enforcement: planned commands are host CLI/Make only; `git -C providers/emailops status` clean
Violation: STOP, rewrite as host CLI usage

CORRECT:
```bash
make build
make init
make setup
make polypus-check
./bin/should-i-read doctor
./bin/should-i-read export --limit 50 --mailbox inbox
```

PROHIBITED:
```bash
# edit providers/emailops/.../openrouter.rs
# gmail_client_secret: "literal-in-yaml"
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
4. `make init` (user config + `~/Library/Application Support/should-i-read/emailops` on macOS)
5. `make setup` (product OAuth if present; else BYO or guided DIY)
6. Connect each mailbox via `make ui` or `emailops-cli accounts add` (one client, N accounts)
7. `make build && ./bin/should-i-read doctor`
8. `./bin/should-i-read sync` (desktop app closed; optional `--account`)
9. `./bin/should-i-read export --limit 50`
10. `make polypus-check` before any classify/TLDR follow-up

## Pre-completion checklist

- [ ] **Provider untouched:** No writes under `providers/`
      Method: `git -C providers/emailops status`
      Pass: clean for this task
      Fail: STOP, revert nested edits
- [ ] **Host CLI used:** Commands go through `bin/should-i-read` or Make aliases
      Method: Review command history in the turn
      Pass: no product path via EmailOps AI subcommands
      Fail: STOP, switch to host CLI
- [ ] **Secrets out of YAML:** Config uses placeholders and `secrets:` list; values via env, keyring, optional SOPS, or product embed
      Method: `config show` shows `(set)` / `(unset)` for secret fields
      Pass: no literal client secrets in config.yaml
      Fail: STOP, move to `make setup` / `secret set` / env
- [ ] **Multi-account model:** One client credential set; N account rows in `accounts`
      Method: Compare `config show` client fields vs `accounts` list
      Pass: single client; multiple accounts if the operator has multiple inboxes
      Fail: STOP, do not invent per-mailbox host configs
