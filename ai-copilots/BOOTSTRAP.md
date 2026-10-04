# BOOTSTRAP — should-i-read ai-copilots

**Audience:** Any AI agent (Cursor, GitHub Copilot, Claude Code, Codex) in this host workspace.

**Goal:** Wire IDE discovery to canonical content under `ai-copilots/`.
MUST NOT copy skill bodies unless symlinks or junctions fail and the user approves copy fallback.

**Module path:** `github.com/xynova/should-i-read`

---

## When to run

| Mode | Phases |
|------|--------|
| **Wire only** | 0 → 2 → 3 |
| **Refresh + wire** | 0 → 1 → 2 → 3 |

---

## Phase 0 — Resolve module root

From this checkout (host is the module):

```bash
ROOT="$(git rev-parse --show-toplevel)"
test -d "$ROOT/ai-copilots/skills/should-i-read-operator" || {
  echo "missing ai-copilots/skills/should-i-read-operator under $ROOT"
  exit 1
}
echo "Module root: $ROOT"
```

---

## Phase 1 — Refresh content (optional)

Only when the user asked to refresh or regenerate skill bodies. Edit files under `ai-copilots/skills/` in place. Do not invent provider patches.

---

## Phase 2 — Wire Cursor

From the repository root:

```bash
make wire-ai-copilots
```

Implementation: [`scripts/wire-cursor-skills.sh`](../scripts/wire-cursor-skills.sh). Host skills use **relative** symlinks (committed). Strop skills are optional and machine-local (gitignored); see [`docs/ai-copilots-setup.md`](../docs/ai-copilots-setup.md).

Optional other IDEs (same target path):

| IDE | Skills link dir |
|-----|-----------------|
| GitHub Copilot | `.github/skills/should-i-read-operator` |
| Claude Code | `.claude/skills/should-i-read-operator` |
| Codex | `.codex/skills/should-i-read-operator` |

---

## Phase 3 — Verify

```bash
ROOT="$(git rev-parse --show-toplevel)"
test -L "$ROOT/.cursor/skills/should-i-read-operator"
test -f "$ROOT/.cursor/skills/should-i-read-operator/SKILL.md"
test -L "$ROOT/.cursor/skills/pimalaya-ecosystem"
test -f "$ROOT/.cursor/skills/pimalaya-ecosystem/SKILL.md"
test -f "$ROOT/AGENTS.md"
```

Pass: symlink resolves; `AGENTS.md` present.
Fail: STOP, fix the link, do not copy skill bodies without user approval.
