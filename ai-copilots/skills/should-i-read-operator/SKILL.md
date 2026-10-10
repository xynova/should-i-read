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

- First-time operator setup (`go tool task configure`, `configure --json`)
- Polypus health before AI work
- Day-2 `go tool task mail:readiness|mail:sync|mail:report|mail:export`, token brokers for multi-account labels

## CLI output

Default stdout is a short human summary (colored on a TTY). Scripts and agents use machine JSON:

- `should-i-read --json configure` (readiness snapshot only; no `--apply`)
- `should-i-read --json mail sync` (full Neverest sync payload)
- `SHOULD_I_READ_JSON=1` forces JSON for any command that supports it

Token bare invoke (`token gmail`) still prints only the access token for Neverest `token_cmd`.

## Mail command vocabulary

| Operator intent | CLI | Task |
|-----------------|-----|------|
| Onboarding checklist (config, token, store) | `mail status` | `go tool task mail:status` (internal) |
| Sync engine check (IMAP, credentials) | `mail readiness` | `go tool task mail:readiness` (`doctor` alias) |
| Pull mail into local store | `mail sync` (`--no-classify` optional) | `go tool task mail:sync` |
| Classify mail (report-only) | `mail report` | `go tool task mail:report` |
| Export summary JSON | `mail export` | `go tool task mail:export` |
| Read one message body | `mail show <ref>` | (CLI only) |

**CONSTRAINT:** MUST use `mail readiness`, `mail sync`, `mail export`, and `mail show` in operator guidance. MUST NOT instruct `pim doctor`, `pim sync`, `pim snapshot`, or `pim show` for normal day-2 work.

## First-time setup

**CONSTRAINT:** Before `go tool task mail:readiness` / `go tool task mail:sync` on a new machine, MUST complete this sequence (or confirm it already ran).

**CONSTRAINT:** `go tool task build` is compile-only. Onboarding uses `go tool task configure` / `should-i-read configure` (see [`AGENTS.md`](../../AGENTS.md)).

**CONSTRAINT:** MUST drive first-time onboarding via **`configure`**. MUST NOT instruct operators to run `setup`, `mail setup`, `pim ensure`, `pim configure`, `pim init`, or bare `neverest` for first-time setup. MAY use hidden `pim` commands only after `configure --json` shows a specific failure.

1. `go tool task build && go tool task init` → `~/.config/should-i-read/config.yaml` (mode `0600`)
2. `go tool task configure` (TTY hub) or `go tool task configure -- --apply --provider gmail --email you@example.com`
3. `./bin/should-i-read config bump` when `polypus.base_url` is still literal `http://127.0.0.1:1320`
4. `go tool task polypus-check`
5. `go tool task mail:readiness` → `go tool task mail:sync` → `go tool task mail:export` (optional: `mail show <object_hash>` to read one synced body)

Inspect: `./bin/should-i-read configure --json` / `mail status`; `config path` / `config show`.

When the sync dependency is missing, run `go tool task configure` (hub runs the dependency step). That is not a Go build bug.

**Trust:** OAuth client ids identify the app; mailbox tokens live in host keyring, not in YAML.

## Multi-account (token labels)

**CONSTRAINT:** One OAuth **client** per provider; each mail sync account block uses `token.command_args` with `--account <label>`.

- MUST: `token gmail login --account <label>` once per label
- MUST NOT: put refresh tokens or MSAL blobs in `config.yaml`

## Core constraints

**CONSTRAINT:** Host workflows MUST use `go tool task` / `bin/should-i-read` or documented task aliases.

- MUST: `go tool task configure` for onboarding; `configure --json` for hub checklist; `go tool task mail:readiness|sync|export` for day-2; `go tool task polypus-check` before AI
- MUST NOT: dial OpenRouter, Ollama, or other leaf AI vendors from host product paths

CORRECT:
```bash
go tool task init && go tool task configure
go tool task polypus-check
go tool task mail:readiness && go tool task mail:sync && go tool task mail:export
```

PROHIBITED:
```bash
neverest check   # operators use go tool task mail:readiness
pim doctor       # use mail readiness
```

**CONSTRAINT:** Before host AI work, MUST fail closed on Polypus (`go tool task polypus-check`). Classify Judge uses `polypus.judge_model` (or first `typesafe/jev` id from `/v1/models`). Author uses `polypus.classify_model` (or first non-jev id) only when Judge skips and `taxonomy.author_on_skip` is true; Author input is host-cleaned latest body (not bulk classify).

**Classify:** `taxonomy.collections` (default INBOX) and `taxonomy.classify_max` (Operate calls per run). `taxonomy.author_on_skip: false` skips Author chat on Judge skip (default report-only). Every classified message goes through taxonomy **attach** `Operate` (Essence, embed, cosine); inspect `tmp/unwanted-report-*.json`, not only the human summary.

**Attach (only strategy):** catalog `inbox-kind` (seed `config/vocabularies/inbox-kind.yaml`, default under `~/.config/should-i-read/vocabularies/`), `polypus.embed_model` via strop `CreateEmbedder`, Essence chat (same model as classify author), thresholds `attach_min_cosine` / `walk_reinforce_min` with `strop/pkg/embed.CosineSimilarity`. Legacy `taxonomy.strategy: walk` in YAML is treated as attach.

## Operator recipe

1. `go tool task wire-ai-copilots` (Cursor skills; darwin/linux)
2. `go tool task build && go tool task init && go tool task configure`
3. `config bump` + `polypus-check` as needed
4. `go tool task mail:readiness` → `go tool task mail:sync` → `go tool task mail:export`

## Pre-completion checklist

- [ ] **Onboarding:** `configure --json` shows `ready` (or clear steps) before claiming sync ready
- [ ] **Secrets out of YAML:** `config show` uses `(set)` / `(unset)`; no literals
- [ ] **Polypus:** `polypus check` passes before AI steps; `polypus check --classify` smokes SystemOne judge and author pick (Polypus OK alone is not classify-ready)
