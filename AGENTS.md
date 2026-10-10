# Agents

This host is the should-i-read operator CLI (mail sync + pimdir + Polypus). Humans start at [README.md](README.md).

## Operator setup (fail closed)

**CONSTRAINT:** Operators and agents drive mail and AI through `go tool task` / `bin/should-i-read` only. The mail sync binary (Neverest) is an implementation detail the host resolves and invokes; do not ask humans to run it or fix PATH for it.

**CONSTRAINT:** `go tool task build` compiles the host Go CLI only. It does **not** install the mail sync dependency or start Polypus.

Before `go tool task mail:readiness` / `go tool task mail:sync` on a machine, agents MUST follow (or confirm already done) the sequence in [`ai-copilots/skills/should-i-read-operator/SKILL.md`](ai-copilots/skills/should-i-read-operator/SKILL.md) and [`docs/pimalaya-setup.md`](docs/pimalaya-setup.md):

```bash
go tool task build && go tool task init
go tool task configure
# or: go tool task configure -- --apply --provider gmail --email you@example.com
./bin/should-i-read config bump          # when Polypus URL was literal localhost
go tool task polypus-check
go tool task mail:readiness && go tool task mail:sync && go tool task mail:export
```

**CONSTRAINT:** MUST drive first-time operator onboarding via `configure` / `go tool task configure`. MUST NOT instruct operators to run `setup`, `mail setup`, `pim ensure`, `pim configure`, `pim init`, or bare `neverest` for first-time setup. MAY use hidden/advanced commands for debugging after `configure --json` shows a specific failure.

Operator CLI stdout is human-first by default; use `should-i-read --json <cmd>` or `SHOULD_I_READ_JSON=1` when a script needs JSON.

| Task / command | Role |
|----------------|------|
| `go tool task configure` / `should-i-read configure` | Operator onboarding hub (config, OAuth apps, mailbox) |
| `go tool task mail:status` (internal) / `should-i-read mail status` | Onboarding checklist (JSON) |
| `go tool task mail:readiness` / `should-i-read mail readiness` | Sync engine check (IMAP); `doctor` is a task alias |
| `go tool task mail:sync` / `mail:report` / `mail:export` | `mail sync` (may classify via strop JobRunner + taxonomy), `mail report`, `mail export` |
| `go tool task polypus-check` | Fail closed before host AI work (`mail report` needs JEV + chat models) |

Nested provider checkouts (read-only for product patches): `providers/taxonomy`, `providers/strop`. Bump submodule gitlinks and `go.mod` after upstream tags; local dev may use `replace` to `./providers/taxonomy` until `v0.2.0` is published.

MUST NOT treat a missing mail sync dependency as a host Go build bug; run `go tool task configure` so the host installs the dependency as part of onboarding.
MUST NOT tell operators to run bare `neverest` or add `~/.cargo/bin` to PATH for product workflows.

## Load order

**Load these before you operate or extend this host:**

1. [`.cursor/rules/architecture.mdc`](.cursor/rules/architecture.mdc) (always-on: Polypus-only AI, host Go)
2. [`.cursor/skills/operator-config/SKILL.md`](.cursor/skills/operator-config/SKILL.md) (XDG config + secrets pattern)
3. [`github.com/behaviorengineering/operatorconfig`](https://github.com/behaviorengineering/operatorconfig) via [`internal/config`](internal/config) and [`internal/secret`](internal/secret) (env → keyring → optional SOPS; `secrets:` list in config drives load-time resolve, same pattern as polypus-local / Polypus `serve`)
4. [`docs/pimalaya-setup.md`](docs/pimalaya-setup.md) (operator path: mail sync, pim, token brokers)
5. [`ai-copilots/skills/should-i-read-operator/SKILL.md`](ai-copilots/skills/should-i-read-operator/SKILL.md) (init, configure hub, polypus check)
6. [`ai-copilots/skills/pimalaya-ecosystem/SKILL.md`](ai-copilots/skills/pimalaya-ecosystem/SKILL.md) and [`docs/pimalaya-ecosystem/INDEX.md`](docs/pimalaya-ecosystem/INDEX.md) (ecosystem index)
7. [`docs/oauth-product-apps.md`](docs/oauth-product-apps.md) (product-owned OAuth apps vs BYO)
8. [`docs/ai-provider-seam.md`](docs/ai-provider-seam.md) (Polypus-only AI)
9. **strop** (`github.com/behaviorengineering/strop`): run that module's `ai-copilots/BOOTSTRAP.md` wire phase (or host `ai-copilots/BOOTSTRAP.md` strop block) for `strop-*` skills before JobRunner work
10. **olly** (`github.com/behaviorengineering/olly`): CLI telemetry via `internal/cli/telemetry.go` (failure dumps under `logs/failures`)

## Wire Cursor discovery

Canonical operator skill lives under `ai-copilots/`. After clone or pull, run `go tool task wire-ai-copilots` (or [`scripts/wire-cursor-skills.sh`](scripts/wire-cursor-skills.sh)). Human notes: [`docs/ai-copilots-setup.md`](docs/ai-copilots-setup.md). Agents may also execute [`ai-copilots/BOOTSTRAP.md`](ai-copilots/BOOTSTRAP.md) in **wire mode** (phases 0 → 2 → 3).

For operatorconfig library skills, resolve the module with `go list -m -f '{{.Dir}}' github.com/behaviorengineering/operatorconfig` and execute that tree's `ai-copilots/BOOTSTRAP.md` (same as [polypus-local](https://gitlab.com/xynova/polypus-local) AGENTS).

MUST keep Cursor links pointing at this repo's `ai-copilots/` tree.
MUST NOT copy skill bodies into `.cursor/skills/` unless symlinks fail and the user approves.
