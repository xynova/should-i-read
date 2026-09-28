# should-i-read ai-copilots

Operator agent pack for this host module.

Canonical source lives here. Wiring into Cursor (and optional other IDEs) is done by executing [BOOTSTRAP.md](BOOTSTRAP.md).

**Minimal prompt:**

> Wire should-i-read ai-copilots using BOOTSTRAP.md

**Skills:**

| Skill | Role |
|-------|------|
| [should-i-read-operator](skills/should-i-read-operator/SKILL.md) | Host CLI: init, Keychain secrets, multi-account, ui, doctor, sync, export, Polypus check |

Pack skills (golang-quality, operator-config, and so on) remain under `.cursor/skills/` via cursor-packs; they are not duplicated here.
