# AI provider seam (Polypus)

Host product AI MUST go through Polypus only. No Jev wiring yet.

**See also:** [pimalaya-ecosystem/INDEX.md](pimalaya-ecosystem/INDEX.md) (mail store and CLI orchestration), [report-only-eval.md](report-only-eval.md) (unwanted-mail exit criteria). Architecture: `.cursor/rules/architecture.mdc`.

## Constraint

Every host AI call MUST go through Polypus at the configured base URL: `polypus.base_url` in `~/.config/should-i-read/config.yaml`, expanded from `${POLYPUS_BASE_URL}` when set, otherwise `POLYPUS_BASE_URL` in the process environment, otherwise `http://127.0.0.1:1320`. Clients MUST NOT dial Cloudflare, LM Studio, OpenRouter cloud, Ollama, or other leaf vendors directly.

## Routes the host may use

| Operation | Preferred Polypus route | Notes |
|-----------|-------------------------|-------|
| Chat / drafts / summaries / TLDR | `POST /v1/chat/completions` | Model id from Polypus allow-list |
| Structured decisions (later Jev) | `POST /v1/chat/completions` (or Polypus Jev adapter) | Prefer a cheap/fast allow-listed model |
| Embeddings / clustering features | `POST /v1/embeddings` | Embedding-capable Polypus model |
| Model discovery | `GET /v1/models` | Enabled list; inventory via `?view=inventory` when debugging |
| Health before AI | `GET /health` | Upstream probe: `GET /health/backends` |

## Fail-closed behavior

1. Probe Polypus with `./scripts/check-polypus.sh` (or `make polypus-check`) and stop if health or models fail.
2. Do not configure OpenRouter cloud keys, Ollama hosts, or embedded llama.cpp for host product AI.
3. Implement classify / Jev / cluster / TLDR as host Go that calls Polypus at `POLYPUS_BASE_URL`.

## Preferred host path

1. Onboard with `make configure`, then day-2 `make readiness` / `make sync` / `make export` (`mail export`).
2. Before AI work: `make polypus-check`.
3. Keep unwanted-mail `report_only` until [report-only-eval.md](report-only-eval.md) exit criteria pass.

## Verification checklist

- [ ] `curl -sf http://127.0.0.1:1320/health` succeeds
- [ ] `curl -sS http://127.0.0.1:1320/v1/models` lists intended chat and embed ids
- [ ] Host code dials only Polypus (no `openrouter.ai`, `:11434`, Cloudflare, LM Studio)
- [ ] Unwanted-mail path remains `report_only`
