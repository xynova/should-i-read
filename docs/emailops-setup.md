# EmailOps setup (host)

Host-owned notes for driving the [`providers/emailops`](../providers/emailops) submodule via the **host** Go CLI. Upstream product docs remain in the submodule README. Agents: load skill `should-i-read-operator`.

## Prerequisites

- Go 1.22+ (host CLI)
- Node.js LTS and npm (EmailOps `make install`)
- Rust stable and Cargo (EmailOps CLI build)
- Tauri OS prerequisites only if you run the desktop GUI: https://tauri.app/start/prerequisites/
- A running Polypus gateway on `http://127.0.0.1:1320` before AI work

## Preferred path (top repo)

```bash
# from host repo root
make emailops-submodule
make emailops-install
make emailops-cli
make build

# Optional: share the desktop app data dir (macOS example)
export EMAILOPS_DATA_DIR="$HOME/Library/Application Support/com.emailops.app"

./bin/should-i-read doctor
./bin/should-i-read accounts
./bin/should-i-read sync          # prefer desktop app closed
./bin/should-i-read emails --limit 20
./bin/should-i-read export --limit 50 --mailbox inbox
make polypus-check
```

Make aliases: `make doctor`, `make sync`, `make emails`, `make export`, `make polypus-check`.

### Data directory footgun

`make -C providers/emailops cli-*` defaults `EMAILOPS_DATA_DIR` to `providers/emailops/.emailops-data`. The host CLI does **not** use that path unless you set it. Default is the platform app-data directory (`com.emailops.app`). Override with `EMAILOPS_DATA_DIR` or `--data-dir`.

### EmailOps binary

Resolution order: `EMAILOPS_CLI` → `PATH` (`emailops-cli`) → `providers/emailops/src-tauri/target/debug/emailops-cli` → cargo build (no llama.cpp) on first need.

## Environment

1. Host AI / operator example:

   ```bash
   cp config/emailops-polypus.example.env config/emailops-polypus.env
   ```

2. EmailOps OAuth (submodule only, never commit filled files):

   ```bash
   cd providers/emailops
   cp .env.example .env.local
   cp src-tauri/.env.example src-tauri/.env.local
   ```

Mailbox credentials stay in EmailOps and the OS keychain. AI credentials stay in Polypus.

## Host CLI commands (v1)

| Command | Purpose |
|---------|---------|
| `version` | Build identity |
| `doctor` | EmailOps readiness (`--json`) |
| `accounts` | List accounts |
| `sync` | Download mail |
| `emails` / `show` | List or show messages |
| `export` | Write `tmp/mail-export-<ts>.json` |
| `polypus check` | Fail-closed Polypus health + models |

Do **not** use EmailOps `classify` / `chat` / `embed` / `compose --send` for this host's product path.

## Polypus gate before AI

```bash
make polypus-check
# or
./bin/should-i-read polypus check
./scripts/check-polypus.sh
```

See [ai-provider-seam.md](ai-provider-seam.md). Unwanted-mail stays report-only: [report-only-eval.md](report-only-eval.md).

## Fail-closed rules

- `POLYPUS_BASE_URL` is the only allowed AI base URL (`http://127.0.0.1:1320`).
- Do not enable OpenRouter cloud or Ollama for host AI features.
- Do not use embedded llama.cpp for host AI features.
- Do not edit `providers/emailops` to wire Polypus.
