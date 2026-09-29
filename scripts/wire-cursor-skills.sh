#!/usr/bin/env bash
# Wire Cursor skill discovery to canonical trees (host ai-copilots + optional strop module).
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

HOST_SKILLS=(should-i-read-operator pimalaya-ecosystem)
STROP_SKILLS=(strop-pipeline-pattern strop-orchestration strop-human-review inference-pace)

mkdir -p .cursor/skills

wire_host_skill() {
  local name="$1"
  local link=".cursor/skills/$name"
  local target="../../ai-copilots/skills/$name"
  if [ ! -f "ai-copilots/skills/$name/SKILL.md" ]; then
    echo "wire-cursor-skills: missing ai-copilots/skills/$name/SKILL.md" >&2
    return 1
  fi
  if [ -e "$link" ] && [ ! -L "$link" ]; then
    echo "wire-cursor-skills: refusing to replace non-symlink $link" >&2
    return 1
  fi
  ln -snf "$target" "$link"
  echo "Wired host: $link -> $target"
}

for name in "${HOST_SKILLS[@]}"; do
  wire_host_skill "$name"
done

STROP_MOD="${STROP_MOD:-}"
if [ -z "$STROP_MOD" ]; then
  STROP_MOD="$(go list -m -f '{{.Dir}}' github.com/behaviorengineering/strop 2>/dev/null || true)"
fi
if [ -n "$STROP_MOD" ] && [ -d "$STROP_MOD/ai-copilots/skills" ]; then
  for name in "${STROP_SKILLS[@]}"; do
    target="$STROP_MOD/ai-copilots/skills/$name"
    link=".cursor/skills/$name"
    if [ ! -d "$target" ]; then
      echo "wire-cursor-skills: skip missing strop skill $name"
      continue
    fi
    if [ -e "$link" ] && [ ! -L "$link" ]; then
      echo "wire-cursor-skills: skip strop (non-symlink exists): $link"
      continue
    fi
    ln -sfn "$target" "$link"
    echo "Wired strop: $link -> $target"
  done
else
  for name in "${STROP_SKILLS[@]}"; do
    link=".cursor/skills/$name"
    if [ -L "$link" ]; then
      rm "$link"
      echo "Removed stale strop link: $link"
    fi
  done
  echo "wire-cursor-skills: strop module not available (optional). To enable:"
  echo "  go get github.com/behaviorengineering/strop@latest"
  echo "  make wire-ai-copilots"
fi

for name in "${HOST_SKILLS[@]}"; do
  test -L ".cursor/skills/$name"
  test -f ".cursor/skills/$name/SKILL.md"
done

test -f AGENTS.md
echo "wire-cursor-skills: OK"
