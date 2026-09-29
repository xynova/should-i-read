# Agents

This host is the should-i-read operator CLI (EmailOps black box + Polypus). Humans start at [README.md](README.md).

**Load these before you operate or extend this host:**

1. [`.cursor/rules/architecture.mdc`](.cursor/rules/architecture.mdc) (always-on: no provider forks, Polypus-only AI)
2. [`.cursor/skills/operator-config/SKILL.md`](.cursor/skills/operator-config/SKILL.md) (XDG config + secrets pattern)
3. [`github.com/behaviorengineering/operatorconfig`](https://github.com/behaviorengineering/operatorconfig) via [`internal/config`](internal/config) and [`internal/secret`](internal/secret) (env → keyring → optional SOPS; `secrets:` list in config drives load-time resolve, same pattern as polypus-local / Polypus `serve`)
4. [`ai-copilots/skills/should-i-read-operator/SKILL.md`](ai-copilots/skills/should-i-read-operator/SKILL.md) (init, setup, secret, ui, doctor, sync, export, multi-account)
5. [`docs/emailops-setup.md`](docs/emailops-setup.md) (human setup notes)
6. [`docs/oauth-product-apps.md`](docs/oauth-product-apps.md) (product-owned OAuth apps vs BYO)
7. [`ai-copilots/skills/pimalaya-ecosystem/SKILL.md`](ai-copilots/skills/pimalaya-ecosystem/SKILL.md) and [`docs/pimalaya-ecosystem/INDEX.md`](docs/pimalaya-ecosystem/INDEX.md) (mail store / Pimalaya CLI architecture context; not EmailOps-as-platform)
8. [`docs/pimalaya-setup.md`](docs/pimalaya-setup.md) (Neverest + pimdir lane; `pim doctor|sync|snapshot`, `token gmail`)
9. **strop** (`github.com/behaviorengineering/strop`): run that module's `ai-copilots/BOOTSTRAP.md` wire phase (or host `ai-copilots/BOOTSTRAP.md` strop block) for `strop-*` skills before JobRunner work
10. **olly** (`github.com/behaviorengineering/olly`): CLI telemetry via `internal/cli/telemetry.go` (failure dumps under `logs/failures`)

## Wire Cursor discovery

Canonical operator skill lives under `ai-copilots/`. Execute [`ai-copilots/BOOTSTRAP.md`](ai-copilots/BOOTSTRAP.md) in **wire mode** so `.cursor/skills/should-i-read-operator` symlinks to that tree.

For operatorconfig library skills, resolve the module with `go list -m -f '{{.Dir}}' github.com/behaviorengineering/operatorconfig` and execute that tree's `ai-copilots/BOOTSTRAP.md` (same as [polypus-local](https://gitlab.com/xynova/polypus-local) AGENTS).

MUST keep Cursor links pointing at this repo's `ai-copilots/` tree.
MUST NOT copy skill bodies into `.cursor/skills/` unless symlinks fail and the user approves.
MUST NOT edit `providers/emailops` for host needs.
