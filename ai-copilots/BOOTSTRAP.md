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

Symlink host skills into Cursor discovery:

```bash
ROOT="$(git rev-parse --show-toplevel)"
mkdir -p "$ROOT/.cursor/skills"

wire_skill() {
  local name="$1"
  local TARGET="$ROOT/ai-copilots/skills/$name"
  local LINK="$ROOT/.cursor/skills/$name"
  if [ -L "$LINK" ]; then
    ln -sfn "$TARGET" "$LINK"
  elif [ -e "$LINK" ]; then
    echo "Refusing to replace non-symlink $LINK; remove it or approve copy fallback"
    exit 1
  else
    ln -s "$TARGET" "$LINK"
  fi
  test -f "$LINK/SKILL.md" && echo "Wired: $LINK -> $TARGET"
}

wire_skill should-i-read-operator
wire_skill pimalaya-ecosystem

# strop module skills (collision-safe; do not replace pack-owned skills)
STROP_MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/strop 2>/dev/null || true)"
if [ -n "$STROP_MOD" ] && [ -d "$STROP_MOD/ai-copilots/skills" ]; then
  for name in strop-pipeline-pattern strop-orchestration strop-human-review inference-pace; do
    TARGET="$STROP_MOD/ai-copilots/skills/$name"
    LINK="$ROOT/.cursor/skills/$name"
    if [ -d "$TARGET" ]; then
      if [ -L "$LINK" ] || [ ! -e "$LINK" ]; then
        ln -sfn "$TARGET" "$LINK"
        echo "Wired strop: $LINK -> $TARGET"
      else
        echo "Skip strop wire (exists): $LINK"
      fi
    fi
  done
fi
```

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
git -C "$ROOT/providers/emailops" status --short
```

Pass: symlink resolves; `AGENTS.md` present; EmailOps working tree clean for this task.
Fail: STOP, fix the link, do not copy skill bodies without user approval.
