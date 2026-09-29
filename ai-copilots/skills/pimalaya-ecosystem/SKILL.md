---
name: pimalaya-ecosystem
description: >-
  Use the host Pimalaya ecosystem AI index and tmp/pimalaya clones when designing
  mail store, sync, CLI orchestration, or architecture for should-i-read. Load
  before proposing Neverest, Himalaya, pimdir, or Polypus mail-layer work.
---

# Pimalaya ecosystem context

**Moral:** Mail architecture for this host is **pimdir + Pimalaya CLIs + Polypus**, not EmailOps-as-platform. Read committed cards first; verify against read-only clones.

Entry: [docs/pimalaya-ecosystem/INDEX.md](../../docs/pimalaya-ecosystem/INDEX.md). Architecture: [architecture.md](../../docs/pimalaya-ecosystem/architecture.md). Integration: [integration.md](../../docs/pimalaya-ecosystem/integration.md). Clone recipe: [reference.md](reference.md).

## When to load

- Designing local mail store, sync, or watch
- Choosing between Himalaya, Neverest, Maildir, or EmailOps for product direction
- Documenting Pimalaya tool allow-lists for Go orchestration
- Agent would patch `providers/emailops` to add mail/AI features

## Constraints

**CONSTRAINT:** MUST read `docs/pimalaya-ecosystem/INDEX.md` before proposing mail-layer architecture or new sync paths.

- Enforcement: agent cites card paths in the proposal
- Violation: STOP, read INDEX and relevant cards, rewrite

**CONSTRAINT:** MUST treat `tmp/pimalaya/**` as read-only upstream evidence.

- Enforcement: no `git add tmp/`; no edits inside clone trees
- Violation: STOP, discard clone changes, re-clone if needed

**CONSTRAINT:** MUST NOT edit `providers/emailops` to satisfy Pimalaya inventory or host mail architecture work.

- Enforcement: `git -C providers/emailops status` before claiming done on architecture docs
- Violation: STOP, move work to host docs or Go under this repo

**CONSTRAINT:** MUST NOT route product AI through Pimalaya tools or EmailOps classify/chat/embed.

- Enforcement: architecture docs reference Polypus only
- Violation: STOP, align with `docs/ai-provider-seam.md`

**CONSTRAINT:** MUST NOT invent `--json` or CLI subcommands without evidence in a card or clone README/`--help`.

- Enforcement: card `evidence` section lists source path
- Violation: STOP, update card from clone

## Checklist (binary)

| Check | Method | Pass | Fail |
|-------|--------|------|------|
| INDEX exists | `test -f docs/pimalaya-ecosystem/INDEX.md` | file present | STOP, run ecosystem index slice |
| Tier A clones | `test -d tmp/pimalaya/pimdir` etc. | dirs present | run `reference.md` clone script |
| tmp ignored | `git check-ignore -v tmp/pimalaya/himalaya` | ignored | do not commit clones |
| Skill wired | `test -f .cursor/skills/pimalaya-ecosystem/SKILL.md` | resolves | run `ai-copilots/BOOTSTRAP.md` Phase 2 |

## CORRECT

```text
Proposal cites docs/pimalaya-ecosystem/apps/neverest.md and architecture.md.
Clones under tmp/pimalaya/ are read-only; EmailOps untouched.
```

## PROHIBITED

```text
Edit providers/emailops to add Polypus sync.
Plan Maildir as the canonical product database without pimdir.
```
