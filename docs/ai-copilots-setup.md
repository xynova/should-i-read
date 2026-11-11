# AI copilots and Cursor skill wiring

Canonical operator content lives under [`ai-copilots/`](../ai-copilots/). Cursor reads skills from [`.cursor/skills/`](../.cursor/skills/) via symlinks.

## After clone or pull

```bash
go tool task wire-ai-copilots
```

That runs [`scripts/wire-cursor-skills.sh`](../scripts/wire-cursor-skills.sh), which:

1. Links **host** skills with **relative** paths (safe to commit):
   - `should-i-read-operator` → `../../ai-copilots/skills/should-i-read-operator`
   - `pimalaya-ecosystem` → `../../ai-copilots/skills/pimalaya-ecosystem`
2. Optionally links **strop** skills when `github.com/behaviorengineering/strop` is in your module cache (machine-local absolute paths; gitignored).

Shared pack skills (golang-quality, operator-config, etc.) come from the **cursor-packs** Cursor plugin (Customize). Install it on each machine; do not symlink pack content into this repo (see `.cursor/packs/plugin-mode`).

Host overlays stay as real files under `.cursor/rules/` (for example `architecture.mdc`, `pimalaya-ecosystem.mdc`).

## Git hooks

Fleet pre-commit hooks are vendored under `lefthook/` and `scripts/git-hooks/` (no cursor-packs submodule). After clone or worktree add:

```bash
go tool task hooks-install
```

## Strop skills

`go.mod` pins `github.com/behaviorengineering/strop` (indirect) so `go tool task wire-ai-copilots` can link pipeline skills after `go mod download`:

```bash
go mod download
go tool task wire-ai-copilots
```

To bump the pin:

```bash
go get github.com/behaviorengineering/strop@latest
go tool task wire-ai-copilots
```

Or point at a checkout:

```bash
STROP_MOD=/path/to/strop go tool task wire-ai-copilots
```

## Agents

Full wire contract: [`ai-copilots/BOOTSTRAP.md`](../ai-copilots/BOOTSTRAP.md). Entry load order: [`AGENTS.md`](../AGENTS.md).

MUST NOT copy skill bodies into `.cursor/skills/` unless symlinks fail and you explicitly approve a copy fallback.
