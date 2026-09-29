# Go host integration (MCP-like tools)

should-i-read orchestrates **allow-listed Pimalaya binaries** the way an agent host orchestrates MCP tools: fixed names, argv contracts, structured stdout, exit semantics. AI stays in the host via Polypus only ([ai-provider-seam.md](../ai-provider-seam.md)).

## Mapping

| Concept | Host (Go) meaning |
|---------|-------------------|
| Tool | Basename on `PATH`: `neverest`, `himalaya`, `ortie`, `sirup` |
| Invoke | `exec.CommandContext(ctx, binary, args...)`; no shell unless a Pimalaya doc requires a `*.command` helper |
| Input | Flags, optional stdin (message bodies); account config in tool XDG TOML |
| Output | Prefer `--json` stdout; decode failure → wrapped host error |
| Secrets | [operator-config](../../.cursor/skills/operator-config/SKILL.md): env, keyring, `password.command` / bearer `token.command`; never commit tokens in docs or cards |
| AI | Host → `POLYPUS_BASE_URL` only; tools do not classify or chat |

## v1 allow-list (host: `internal/pimalaya`)

| Tool | Typical argv | Role |
|------|--------------|------|
| `neverest` | `sync`, `init`, `check`, `conflict …` + `--json` | Ingest / reconcile **pimdir** replica |
| `himalaya` | `envelope`, `message`, `mailbox`, `flag` + `--json` | Live query/mutate when store projection is insufficient |
| `ortie` | token/account commands + `--json` | OAuth lifecycle |
| `sirup` | `start`, `configure` (optional) | Unix-socket sessions for Himalaya |

**Out of v1 allow-list:** `himalaya-tui`, vim/emacs plugins, `android`, `pimalaya-linux`, `comodoro`, interactive wizards except as one-time operator setup.

## Exit codes

| Tool | Notable exits |
|------|----------------|
| neverest | **2** = reconciled but human action required (conflicts, duplicate UID, refused writes) |
| himalaya | Standard non-zero on failure; parse stderr + exit code |
| ortie | Non-zero on OAuth failure |

Host SHOULD map exit 2 from neverest to a structured "needs_review" state, not silent success.

## Config coexistence

| Component | Config location |
|-----------|-----------------|
| should-i-read | `~/.config/should-i-read/config.yaml` (XDG) |
| neverest | `~/.config/neverest/config.toml` or `NEVEREST_CONFIG` |
| himalaya | `~/.config/himalaya/config.toml` |
| ortie | ortie XDG TOML (see ortie README) |

Glue may share **account identifiers** and **secret commands** across files without merging formats in the first slice.

## Ownership boundary

| Layer | Owns |
|-------|------|
| Pimalaya CLIs | Protocol bytes, sync, flags, local pimdir replica writes via neverest |
| should-i-read | Policy, sequencing, projections, report artifacts, unwanted-mail decisions, Polypus prompts, UX |

Tools MUST NOT own "should I read?" product logic.

## Future (not v1)

- Go **reader profile** on pimdir SQLite per `store/pimdir.md` (no Rust FFI)
- Carillon hooks triggering `neverest sync` when watch stack stabilises
