# EmailOps AI capabilities and host roadmap

Inventory of what [`providers/emailops`](../providers/emailops) does in the AI arena, and how **should-i-read** plans to relate to it. Upstream product docs remain in the submodule (for example `docs/site/en/ai-features.md`). This file is host-owned so the roadmap can stay product-specific without spilling into EmailOps.

**Related:** [ai-provider-seam.md](ai-provider-seam.md) (Polypus constraint and blocked providers), [report-only-eval.md](report-only-eval.md) (unwanted-mail exit criteria), [emailops-setup.md](emailops-setup.md) (operator setup). Architecture: `.cursor/rules/architecture.mdc`.

**Target mail layer (architecture):** EmailOps is the **current** black-box custody and UI experiment, not the long-term store design. For Pimalaya-shaped mail (pimdir local store, Neverest sync, Himalaya CLI tools, host orchestration), use the committed agent index at [pimalaya-ecosystem/INDEX.md](pimalaya-ecosystem/INDEX.md) and [pimalaya-ecosystem/architecture.md](pimalaya-ecosystem/architecture.md).

## Purpose

1. Name EmailOps AI capabilities in one place for operators and agents.
2. Draw a hard boundary: EmailOps AI product vs should-i-read product AI.
3. Hold a phased roadmap we can edit without forking the submodule.

## Boundary

| Concern | EmailOps | should-i-read host |
|---------|----------|--------------------|
| Sync, accounts, local SQLite, export | Yes (black box CLI + UI) | Orchestrates via Go CLI / Make |
| In-app chat, classify, embed, drafts, … | Yes (EmailOps product) | **Unused** for host product AI |
| Inference backends | llama.cpp, Ollama, OpenRouter | **Polypus only** (`POLYPUS_BASE_URL`, default `http://127.0.0.1:1320`) |
| Unwanted-mail / Jev / cluster / TLDR | Not this product’s path | Host Go → Polypus; report-only until eval passes |
| Nested-repo edits for host needs | Forbidden | Put work in host Go or a portable upstream PR |

There is **no** env injection today that points EmailOps OpenRouter or Ollama at Polypus. OpenRouter’s base URL is hardcoded; Ollama’s HTTP shape does not match Polypus OpenAI routes. See [ai-provider-seam.md](ai-provider-seam.md).

## Capability inventory

### Stable AI features

| Capability | What it does |
|------------|----------------|
| **Chat with mailbox** | Natural-language Q&A over one account; RAG and/or tool calls against local DB; citations; streaming |
| **AI drafts** | Reply drafts from the open thread; persona / style / tone prompts |
| **Classification** | Tags: priority, intent, topic. Rules first (no model); model for the rest |
| **Tag Board** | Board UI over classification tags (company / priority / intent / topic) |
| **Semantic search / embeddings** | Meaning search; feeds chat retrieval; “find similar” |
| **Translation** | Translate messages and compose text via editable prompts |

### Experimental AI features

| Capability | What it does |
|------------|----------------|
| **Tasks** | Extract action items, commitments, deadlines |
| **Memory** | Long-lived contact / domain / project facts for chat |
| **Lenses** | Schema-typed extracted views over mail (for example invoices) |

### Non-LLM and controls

| Capability | Notes |
|------------|--------|
| **Rule-based tags** | Pattern match on sender/subject; no model call |
| **Junk heuristics** | Local EmailOps junk path; not Jev |
| **Master AI off** | Settings → AI Backend & Models → AI Features; plain mail client when off |
| **CLI mirrors** | `emailops-cli chat`, `classify`, `embed`, `doctor` (same service layer as the app) |

### Inference backends (EmailOps)

| Kind | Where it runs | Host product status |
|------|---------------|---------------------|
| `llamacpp` | In-process GGUF (default) | Blocked for host product AI |
| `ollama` | Local daemon `:11434` | Blocked (wrong protocol for Polypus) |
| `openrouter` | Cloud `openrouter.ai` | Blocked (hardcoded base; must not use cloud keys as a bypass) |

## Host adoption map

How should-i-read treats each EmailOps AI capability for **product** work:

| Capability | Host stance |
|------------|-------------|
| Sync / accounts / export / doctor (non-AI) | **Use as-is** via host CLI |
| Chat, drafts, translation, Tag Board, tasks, memory, lenses | **Ignore for product** (EmailOps-only UX; may explore personally) |
| `classify` / `embed` CLI or in-app | **Ignore for product**; do not point at OpenRouter/Ollama/llama.cpp for host features |
| Unwanted-mail decisions, clusters, TLDRs | **Rebuild on host + Polypus** (report-only first) |
| EmailOps desktop talking to Polypus | **Optional upstream** contribution (injectable OpenAI-compatible base URL in EmailOps); not required for host pipeline |

## Roadmap phases

Edit this section as milestones land. Dates are not commitments; order is the contract.

### Phase 0 — Mail custody (current)

- Host config, OAuth client secrets, EmailOps UI / sync / export.
- EmailOps in-app AI left unused for product features.
- Polypus not required for mail-only operation.

### Phase 1 — Polypus gate ready

- `make polypus-check` / `./bin/should-i-read polypus check` fail-closed before any host AI.
- Model ids documented (see `config/emailops-polypus.example.env`); values must match Polypus allow-list.
- No host or EmailOps dials to leaf AI vendors for product work.

### Phase 2 — Report-only unwanted mail

- Host Go: export candidates → Jev via Polypus → cluster → TLDR via Polypus chat → artifact under `tmp/`.
- Exit criteria and no-mailbox-mutation rules: [report-only-eval.md](report-only-eval.md).
- Still no EmailOps `classify` / `chat` / `embed` for this path.

### Phase 3 — Eval pass, then mutation policy

- Only after Phase 2 exit criteria pass: decide quarantine / trash / mark-read (separate milestone).
- Keep report artifacts for audit.

### Optional track — EmailOps UI via Polypus

- Portable change in EmailOps: injectable OpenAI-compatible base URL (and related config).
- Nested-repo PR; host only bumps submodule pin after it lands.
- Not a substitute for Phase 2 host pipeline.

## Out of scope

- Patching `providers/emailops` so OpenRouter “just works” against Polypus from this host.
- Setting `OPENROUTER_API_KEY` or enabling embedded llama.cpp for should-i-read product AI.
- Pointing `OLLAMA_HOST` at `:1320`.
- Mailbox mutation as part of the first unwanted-mail milestone.
- Host product feature logic in TypeScript, Python, or EmailOps source trees.

## Verification checklist

- [ ] This doc lists capabilities that still match upstream `ai-features` when EmailOps is bumped.
- [ ] Host AI paths grep clean of `openrouter.ai`, `:11434`, and non-loopback AI bases.
- [ ] Unwanted-mail path remains `report_only` until [report-only-eval.md](report-only-eval.md) exits.
- [ ] `git -C providers/emailops status` stays clean for host roadmap work.
