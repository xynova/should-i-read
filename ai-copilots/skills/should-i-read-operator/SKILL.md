---
name: should-i-read-operator
description: >-
  Operates the should-i-read host CLI: configure hub, mail status/readiness/sync/report/export/show,
  token gmail|outlook for scripts, Polypus check, init/config.
---

# should-i-read operator

**Moral:** Drive the host Go CLI and XDG config. Mail lane is **Pimalaya** (host-managed sync + pimdir + token brokers). Product AI goes through Polypus only.

Architecture: `.cursor/rules/architecture.mdc`. **Setup:** [`docs/pimalaya-setup.md`](../../docs/pimalaya-setup.md). Polypus: [`docs/ai-provider-seam.md`](../../docs/ai-provider-seam.md). Operator config: skill `operator-config`. Entry: [`AGENTS.md`](../../AGENTS.md).

## When to load

- First-time operator setup (`make configure`, `configure --json`)
- Polypus health before AI work
- Day-2 `make readiness|sync|report|export`, token brokers for multi-account labels

## CLI output

Default stdout is a short human summary (colored on a TTY). Scripts and agents use machine JSON:

- `should-i-read --json configure` (readiness snapshot only; no `--apply`)
- `should-i-read --json mail sync` (full Neverest sync payload)
- `SHOULD_I_READ_JSON=1` forces JSON for any command that supports it

Token bare invoke (`token gmail`) still prints only the access token for Neverest `token_cmd`.

## Mail command vocabulary

| Operator intent | CLI | Make |
|-----------------|-----|------|
| Onboarding checklist (config, token, store) | `mail status` | `make mail-status` |
| Sync engine check (IMAP, credentials) | `mail readiness` | `make readiness` (`make doctor` alias) |
| Pull mail into local store | `mail sync` (`--no-classify` optional) | `make sync` |
| Classify mail (report-only) | `mail report` | `make report` |
| Export summary JSON | `mail export` | `make export` |
| Read one message body | `mail show <ref>` | (CLI only) |

**CONSTRAINT:** MUST use `mail readiness`, `mail sync`, `mail export`, and `mail show` in operator guidance. MUST NOT instruct `pim doctor`, `pim sync`, `pim snapshot`, or `pim show` for normal day-2 work.

## First-time setup

**CONSTRAINT:** Before `make readiness` / `make sync` on a new machine, MUST complete this sequence (or confirm it already ran).

**CONSTRAINT:** `make build` is compile-only. Onboarding uses `make configure` / `should-i-read configure` (see [`AGENTS.md`](../../AGENTS.md)).

**CONSTRAINT:** MUST drive first-time onboarding via **`configure`**. MUST NOT instruct operators to run `setup`, `mail setup`, `pim ensure`, `pim configure`, `pim init`, or bare `neverest` for first-time setup. MAY use hidden `pim` commands only after `configure --json` shows a specific failure.

1. `make build && make init` → `~/.config/should-i-read/config.yaml` (mode `0600`)
2. `make configure` (TTY hub) or `make configure ARGS='--apply --provider gmail --email you@example.com'`
3. `./bin/should-i-read config bump` when `polypus.base_url` is still literal `http://127.0.0.1:1320`
4. `make polypus-check`
5. `make readiness` → `make sync` → `make export` (optional: `mail show <object_hash>` to read one synced body)

Inspect: `./bin/should-i-read configure --json` / `mail status`; `config path` / `config show`.

When the sync dependency is missing, run `make configure` (hub runs the dependency step). That is not a Go build bug.

**Trust:** OAuth client ids identify the app; mailbox tokens live in host keyring, not in YAML.

## Multi-account (token labels)

**CONSTRAINT:** One OAuth **client** per provider; each mail sync account block uses `token.command_args` with `--account <label>`.

- MUST: `token gmail login --account <label>` once per label
- MUST NOT: put refresh tokens or MSAL blobs in `config.yaml`

## Core constraints

**CONSTRAINT:** Host workflows MUST use `make` / `bin/should-i-read` or documented Make aliases.

- MUST: `make configure` for onboarding; `configure --json` for hub checklist; `make readiness|sync|export` for day-2; `make polypus-check` before AI
- MUST NOT: dial OpenRouter, Ollama, or other leaf AI vendors from host product paths

CORRECT:
```bash
make init && make configure
make polypus-check
make readiness && make sync && make export
```

PROHIBITED:
```bash
neverest check   # operators use make readiness
pim doctor       # use mail readiness
```

**CONSTRAINT:** Before host AI work, MUST fail closed on Polypus (`make polypus-check`). Classify Judge uses `polypus.judge_model` (or first `typesafe/jev` id from `/v1/models`). Author uses `polypus.classify_model` (or first non-jev id).

## Operator recipe

1. `make wire-ai-copilots` (Cursor skills)
2. `make build && make init && make configure`
3. `config bump` + `polypus-check` as needed
4. `make readiness` → `make sync` → `make export`

## Pre-completion checklist

- [ ] **Onboarding:** `configure --json` shows `ready` (or clear steps) before claiming sync ready
- [ ] **Secrets out of YAML:** `config show` uses `(set)` / `(unset)`; no literals
- [ ] **Polypus:** `polypus check` passes before AI steps (classify needs a JEV model and a chat model)
