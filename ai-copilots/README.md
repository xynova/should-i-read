# should-i-read ai-copilots

Operator agent pack for this host module.

Canonical source lives here. Wiring into Cursor (and optional other IDEs) is done by executing [BOOTSTRAP.md](BOOTSTRAP.md).

**Minimal prompt:**

> Wire should-i-read ai-copilots using BOOTSTRAP.md

**Skills:**

| Skill | Role |
|-------|------|
| [should-i-read-operator](skills/should-i-read-operator/SKILL.md) | Host CLI: init, Keychain secrets, mail setup/status, export, Polypus check |
| [host-classify-store](skills/host-classify-store/SKILL.md) | Classify ledger path, mail status/similar, progress JSON import-only |
| [hoststore-sqlite](skills/hoststore-sqlite/SKILL.md) | modernc SQLite, golang-migrate, write actor, no CGO |

Pack skills (golang-quality, operator-config, and so on) remain under `.cursor/skills/` via cursor-packs; they are not duplicated here.
