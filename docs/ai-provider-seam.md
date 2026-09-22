# EmailOps AI provider seam (Polypus)

Audit of how [`providers/emailops`](../providers/emailops) reaches models today, and what this host requires. No Jev wiring yet.

## Constraint

Every EmailOps AI call for this host MUST go through Polypus at `http://127.0.0.1:1320` (`POLYPUS_BASE_URL`). Clients MUST NOT dial Cloudflare, LM Studio, OpenRouter cloud, or Ollama directly.

## Upstream providers (as shipped)

EmailOps resolves AI through `AiService::load_provider` in `src-tauri/src/services/ai.rs`. Provider kinds:

| Kind | Implementation | Protocol | Base URL today | Host status |
|------|----------------|----------|----------------|-------------|
| `llamacpp` | Embedded GGUF runtime | in-process | n/a | **Blocked** for this host (bypasses gateway) |
| `ollama` | `src-tauri/src/ai/ollama.rs` | Ollama `/api/*` | `OLLAMA_HOST` or `http://localhost:11434` | **Blocked** (wrong protocol for Polypus) |
| `openrouter` | `src-tauri/src/ai/openrouter.rs` | OpenAI-ish `/v1/*` | Hardcoded `https://openrouter.ai/api/v1` | **Blocked** until base URL is injectable and points at Polypus |

### Hardcoded OpenRouter base

```13:15:providers/emailops/src-tauri/src/ai/openrouter.rs
const OPENROUTER_BASE_URL: &str = "https://openrouter.ai/api/v1";
const APP_NAME: &str = "emailops";
const APP_URL: &str = "https://github.com/emailops";
```

Chat, embeddings, and model list all concatenate onto that constant. There is no env override. Setting `OPENROUTER_API_KEY` alone still sends traffic to OpenRouter cloud, which violates the host contract.

### Ollama cannot be a Polypus client

Ollama uses `/api/chat`, `/api/generate`, `/api/embeddings`. Polypus exposes OpenAI-compatible `/v1/chat/completions`, `/v1/embeddings`, `/v1/models`. Pointing `OLLAMA_HOST` at `:1320` will fail.

## Operation map (target)

Once a Polypus-backed OpenAI-compatible provider exists in EmailOps (nested-repo change), map features as follows:

| EmailOps operation | Preferred Polypus route | Notes |
|--------------------|-------------------------|-------|
| Chat / drafts / summaries | `POST /v1/chat/completions` | Model id from Polypus allow-list (`EMAILOPS_CHAT_MODEL`) |
| Classification (`classify`) | `POST /v1/chat/completions` | Prefer a cheap/fast allow-listed model (`EMAILOPS_CLASSIFY_MODEL`) |
| Embeddings (`embed`, semantic search) | `POST /v1/embeddings` | Must use an embedding-capable Polypus model (`EMAILOPS_EMBED_MODEL`) |
| Model discovery / doctor | `GET /v1/models` | Enabled list only; inventory via `?view=inventory` when debugging |
| Health before AI | `GET /health` | Upstream probe: `GET /health/backends` |
| Junk / spam heuristics | Local EmailOps `services/junk` (no LLM) | Keep; do not replace with silent remote calls |
| Future Jev decisions | Via Polypus adapter (not EmailOps → TypeSafe direct) | See [report-only-eval.md](report-only-eval.md) |

## Fail-closed behavior (host)

Until the adapter lands:

1. Treat EmailOps AI features as unavailable for this host.
2. Do not configure OpenRouter cloud keys to "make classify work."
3. Do not use embedded llama.cpp as a temporary bypass.
4. Probe Polypus with `./scripts/check-polypus.sh` and stop if health or models fail.

## Required follow-up (nested EmailOps change)

Portable change in the EmailOps repository (not host spill):

1. Add an OpenAI-compatible HTTP provider (or make OpenRouter's base URL configurable).
2. Default or document `base_url = POLYPUS_BASE_URL` for host consumers.
3. Accept Polypus-prefixed model ids (`cf_local/…`, `lm_studio/…`, `router/…`).
4. Keep retries on the client for 503/429 only; Polypus owns circuit breaking.

Host then: set preferences / env from `config/emailops-polypus.example.env`, re-pin the submodule gitlink.

## Verification checklist

- [ ] `curl -sf http://127.0.0.1:1320/health` succeeds
- [ ] `curl -sS http://127.0.0.1:1320/v1/models` lists intended chat and embed ids
- [ ] EmailOps provider base URL resolves to Polypus only (after adapter)
- [ ] No process dials `openrouter.ai`, Cloudflare, or LM Studio from EmailOps
- [ ] Unwanted-mail path remains `report_only`
