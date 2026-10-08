# AI provider seam (Polypus)

Host product AI MUST go through Polypus only. Taxonomy `Operate` walks the catalog tree (children per hop). Each hop uses Polypus `POST /v1/systemone` (JEV, one noul per packed sibling). Author uses `POST /v1/chat/completions`. Strop JobRunner job `mail_classify` wraps one `Operate`; it does not call TypeSafe. Library sources: `providers/taxonomy`, `providers/strop` (submodules; `go.mod` pins releases).

**See also:** [pimalaya-ecosystem/INDEX.md](pimalaya-ecosystem/INDEX.md) (mail store and CLI orchestration), [report-only-eval.md](report-only-eval.md) (unwanted-mail exit criteria). Architecture: `.cursor/rules/architecture.mdc`.

## Constraint

Every host AI call MUST go through Polypus at the configured base URL: `polypus.base_url` in `~/.config/should-i-read/config.yaml`, expanded from `${POLYPUS_BASE_URL}` when set, otherwise `POLYPUS_BASE_URL` in the process environment, otherwise `http://127.0.0.1:1320`. Clients MUST NOT dial Cloudflare, LM Studio, OpenRouter cloud, Ollama, or other leaf vendors directly.

## Routes the host may use

| Operation | Preferred Polypus route | Notes |
|-----------|-------------------------|-------|
| Chat / drafts / summaries / TLDR | `POST /v1/chat/completions` | Model id from Polypus allow-list |
| Taxonomy Judge (classify) | `POST /v1/systemone` | `polypus.judge_model` when set (smoked), else catalog `typesafe/jev` id, else off-catalog candidates (`cf_local/typesafe/jev`, `typesafe/jev`); each choice is verified with a SystemOne readiness probe |
| Taxonomy Author (classify) | `POST /v1/chat/completions` | `polypus.classify_model` (e.g. Granite micro on `cf_local`); runs only when Judge skips and `taxonomy.author_on_skip` is true. Host prepares Author input: latest-message trim, deterministic denoise, extractive essence (no extra LLM hop). Judge stays on SystemOne (JEV) with full message text. |
| Embeddings / attach classify | `POST /v1/embeddings` via strop `LLMFactory.CreateEmbedder` (dspy-go) | Same Polypus base URL; host does not call `polypus.Client.Embed` on classify |
| Model discovery | `GET /v1/models` | Enabled list; inventory via `?view=inventory` when debugging |
| Health before AI | `GET /health` | Upstream probe: `GET /health/backends` |

## Fail-closed behavior

1. `mail report` and post-sync classify call Polypus check; `mail report` fails closed when Polypus is down. `mail sync` still completes the replica when classify is skipped.
2. Probe Polypus with `./scripts/check-polypus.sh` (or `make polypus-check`) before other AI work. Gateway health plus a non-empty model list does not prove classify readiness; run `should-i-read polypus check --classify` to smoke SystemOne judge and chat author (same probes as `mail report`).
3. Do not configure OpenRouter cloud keys, Ollama hosts, or embedded llama.cpp for host product AI.
4. Implement cluster / TLDR as host Go that calls Polypus at `POLYPUS_BASE_URL`.

## Preferred host path

1. Onboard with `make configure`, then day-2 `make readiness` / `make sync` (may classify fetches) / `make report` or `make export`.
2. Before AI work: `make polypus-check`.
3. Keep unwanted-mail `report_only` until [report-only-eval.md](report-only-eval.md) exit criteria pass.

## Verification checklist

- [ ] `curl -sf http://127.0.0.1:1320/health` succeeds
- [ ] `curl -sS http://127.0.0.1:1320/v1/models` lists intended chat and embed ids (JEV may be absent when `models.sync` lists only synced chat/TTS/STT ids)
- [ ] `should-i-read polypus check --classify` resolves judge via SystemOne and picks a chat author model

## Judge discovery vs OpenAI Decisions

| Surface | Host use |
|---------|----------|
| `POST /v1/chat/completions` | Author |
| `POST /v1/systemone` | Judge (JEV) |
| OpenAI Decisions preview | Not integrated; no stable public HTTP contract for this host |

Do not infer judge availability from `GET /v1/models` alone when sync omits SystemOne backends.
- [ ] Host code dials only Polypus (no `openrouter.ai`, `:11434`, Cloudflare, LM Studio)
- [ ] Unwanted-mail path remains `report_only`
