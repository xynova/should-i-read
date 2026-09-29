# AI copilots and Cursor skill wiring

Canonical operator content lives under [`ai-copilots/`](../ai-copilots/). Cursor reads skills from [`.cursor/skills/`](../.cursor/skills/) via symlinks.

## After clone or pull

```bash
make wire-ai-copilots
```

That runs [`scripts/wire-cursor-skills.sh`](../scripts/wire-cursor-skills.sh), which:

1. Links **host** skills with **relative** paths (safe to commit):
   - `should-i-read-operator` → `../../ai-copilots/skills/should-i-read-operator`
   - `pimalaya-ecosystem` → `../../ai-copilots/skills/pimalaya-ecosystem`
2. Optionally links **strop** skills when `github.com/behaviorengineering/strop` is in your module cache (machine-local absolute paths; gitignored).

Shared pack skills (golang-quality, operator-config, etc.) are separate symlinks into `.cursor/packs/shared/` and are already tracked in git.

## Optional strop skills

When you work on strop pipelines in this host, add the module and re-wire:

```bash
go get github.com/behaviorengineering/strop@latest
make wire-ai-copilots
```

Or point at a checkout:

```bash
STROP_MOD=/path/to/strop make wire-ai-copilots
```

## Agents

Full wire contract: [`ai-copilots/BOOTSTRAP.md`](../ai-copilots/BOOTSTRAP.md). Entry load order: [`AGENTS.md`](../AGENTS.md).

MUST NOT copy skill bodies into `.cursor/skills/` unless symlinks fail and you explicitly approve a copy fallback.
