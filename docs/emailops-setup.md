# EmailOps setup (host)

Host-owned notes for running the [`providers/emailops`](../providers/emailops) submodule with Polypus as the only AI gateway. Upstream product docs remain in the submodule README.

## Prerequisites

- Node.js LTS and npm
- Rust stable and Cargo
- Tauri OS prerequisites: https://tauri.app/start/prerequisites/
- CMake and a C++ toolchain (Xcode Command Line Tools on macOS) if you build with embedded llama.cpp
- A running Polypus gateway on `http://127.0.0.1:1320` (see `~/Xynova/ai/polypus` or your local checkout)

Embedded llama.cpp is available upstream for privacy-first local inference. **This host does not use it for AI features.** All EmailOps AI traffic must go through Polypus once the provider adapter exists.

## Submodule

```bash
# from host repo root
git submodule update --init --recursive providers/emailops
cd providers/emailops
git rev-parse --short HEAD   # record the pin when reporting issues
```

Ownership: the nested tree is a separate git repository. Do not stage EmailOps file bytes as ordinary host files. Branch and commit inside `providers/emailops` for upstream-shaped changes, then bump the host gitlink.

## Environment (mail vs AI)

1. Copy host AI example:

   ```bash
   cp config/emailops-polypus.example.env config/emailops-polypus.env
   # edit model ids after probing Polypus /v1/models
   ```

2. Copy EmailOps OAuth templates inside the submodule only (never commit filled files):

   ```bash
   cd providers/emailops
   cp .env.example .env.local
   cp src-tauri/.env.example src-tauri/.env.local
   # fill Gmail / Outlook desktop OAuth clients
   ```

Mailbox credentials stay in EmailOps `.env.local` and the OS keychain. AI backend credentials stay in Polypus (`~/.config/polypus/config.yaml`, `stack/.env`). Client repos must not store remote provider URLs.

## Dev commands (submodule)

From `providers/emailops`:

```bash
make install     # npm install + lefthook
make doctor      # if available via CLI; else build cli then doctor
make cli-fast ARGS="doctor --json"
make dev         # Tauri app (repo-local data dir)
make cli-fast ARGS="accounts --json"
make cli-fast ARGS="emails --limit 20 --json"
```

Useful CLI surface for this host:

| Command | Use |
|---------|-----|
| `doctor` | Readiness: DB, accounts, AI config (loads no model) |
| `sync` | Download mail |
| `emails` / `show` / `search` | Inspect mailbox without AI |
| `classify` | AI classification (requires Polypus-backed provider) |
| `embed` | Semantic embeddings (requires Polypus-backed provider) |
| `chat` | Inbox Q&A (requires Polypus-backed provider) |

Heavy writes (`sync`, `classify`, `embed`) are best run with the desktop app closed.

## Polypus gate before AI

```bash
./scripts/check-polypus.sh
```

Must pass before any classify, embed, or chat work for this host. See [ai-provider-seam.md](ai-provider-seam.md) for why stock OpenRouter and Ollama settings cannot satisfy the contract yet.

## Fail-closed rules

- `POLYPUS_BASE_URL` is the only allowed AI base URL (`http://127.0.0.1:1320`).
- Do not enable OpenRouter against `https://openrouter.ai` from this host.
- Do not set `OLLAMA_HOST` to Polypus (Ollama API shape is not OpenAI `/v1`).
- Do not use embedded llama.cpp for host AI features (`EMAILOPS_ALLOW_EMBEDDED_LLAMACPP=0`).
- Unwanted-mail automation stays `report_only` until evaluation thresholds exist ([report-only-eval.md](report-only-eval.md)).
