# EmailOps setup (legacy)

**Default operator path is Pimalaya (Neverest + pimdir).** Use [`pimalaya-setup.md`](pimalaya-setup.md) and `make doctor` / `make sync` / `make export` (pim snapshot). This doc is for optional EmailOps custody via the [`providers/emailops`](../providers/emailops) submodule. Make targets: `make emailops-*`. Agents: [`AGENTS.md`](../AGENTS.md), skill `should-i-read-operator`.

## Prerequisites

- Go 1.22+ (host CLI)
- Node.js LTS and npm (EmailOps `make install`)
- Rust stable and Cargo (EmailOps CLI build)
- Tauri OS prerequisites only if you run the desktop GUI: https://tauri.app/start/prerequisites/
- A running Polypus gateway on `http://127.0.0.1:1320` before AI work

## Legacy operator path (EmailOps only)

Shared steps with the default lane: `make init`, `make setup`, `make polypus-check` (see [pimalaya-setup.md](pimalaya-setup.md)).

```bash
make emailops-submodule
make emailops-install
make emailops-cli
make emailops-ui
make emailops-doctor
make emailops-accounts
make emailops-sync
make emailops-emails ARGS="--limit 20"
make emailops-export ARGS="--limit 50 --mailbox inbox"
```

Make aliases: `make emailops-ui`, `make emailops-doctor`, `make emailops-sync`, `make emailops-export`, `make emailops-accounts`, `make emailops-emails`.

### Mail OAuth (product-owned default)

Ordinary installs use **product-owned** Google Desktop + Microsoft public OAuth *client* apps. End users only consent per mailbox in the browser. They do not create Cloud Console or Entra projects.

Credential resolve order (OAuth client ids):

1. Process environment
2. Platform keyring (service `should-i-read`) via [operatorconfig](https://github.com/behaviorengineering/operatorconfig)
3. Optional `~/.config/should-i-read/secrets.enc.yaml` (SOPS) when present
4. Product embeds from release `-ldflags` into `internal/oauthcred` (OAuth fields only)
5. `make setup` BYO paste or advanced guided DIY

Non-OAuth placeholders in YAML expand via the same env/keyring/SOPS path during `config.Load`.

Details for product owners registering apps: [`docs/oauth-product-apps.md`](oauth-product-apps.md).

Trust plain English: mail and tokens stay on this computer; the client id only identifies which app is asking; you can replace product client ids with your own (BYO) without losing local mailbox custody.

### Operator config

- Live file: `~/.config/should-i-read/config.yaml` (mode `0600`)
- Example template: [`config/should-i-read.example.yaml`](../config/should-i-read.example.yaml)
- Override: `--config` or `SHOULD_I_READ_CONFIG` (missing override path fails closed)
- Secrets: top-level `secrets:` list in config declares which env names may use keyring/SOPS on load (polypus-local / Polypus `serve` pattern)
- `should-i-read secret set` only accepts names listed under `secrets:` in the live config
- Resolve order: env → keyring → optional SOPS → (OAuth only) product release embed
- Inspect: `./bin/should-i-read config path` / `config show` (redacted)

### Data directory

Default host data dir is under the should-i-read app id, not EmailOps' own:

- macOS: `~/Library/Application Support/should-i-read/emailops`
- Linux: `$XDG_DATA_HOME/should-i-read/emailops` or `~/.local/share/should-i-read/emailops`

Override with `emailops.data_dir` in config, `EMAILOPS_DATA_DIR`, or `--data-dir`.

`make -C providers/emailops cli-*` still defaults to `providers/emailops/.emailops-data`; do not use that for host workflows.

### EmailOps binary

Resolution order: config `cli_path` → `EMAILOPS_CLI` → `PATH` → `providers/emailops/src-tauri/target/debug/emailops-cli` → cargo build (no llama.cpp) on first need.

## Environment

1. Host config (preferred):

   ```bash
   make init
   # edit ~/.config/should-i-read/config.yaml if needed
   ```

2. Optional Polypus example env (repo-local, no secrets):

   ```bash
   cp config/emailops-polypus.example.env config/emailops-polypus.env
   ```

Mailbox OAuth *tokens* stay in EmailOps' own keychain once accounts are added. Host Keychain holds OAuth *client* secrets injected at launch. AI credentials stay in Polypus.

### Multi-account (several Gmail inboxes)

One Gmail OAuth *client* id/secret covers every Gmail mailbox. Each address is a separate EmailOps *account* (add via `make ui` or `emailops-cli accounts add`). List them with `./bin/should-i-read accounts`. Scope sync/list/export with `--account` or `emailops.default_account` in config. Do not create one `config.yaml` per mailbox; do not store mailbox refresh tokens in the host config file.

## Host CLI commands

| Command | Purpose |
|---------|---------|
| `init` | Create user config + default data dir |
| `setup` | Interactive OAuth client setup (product / BYO / guided DIY) |
| `config path` / `config show` | Resolve path / redacted effective config |
| `secret set` / `secret delete` | Keychain secrets (macOS) |
| `ui` | Launch EmailOps desktop with injected env |
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

See [ai-provider-seam.md](ai-provider-seam.md). EmailOps AI capability inventory and host roadmap: [emailops-ai-capabilities-roadmap.md](emailops-ai-capabilities-roadmap.md). Unwanted-mail stays report-only: [report-only-eval.md](report-only-eval.md).

## Fail-closed rules

- `POLYPUS_BASE_URL` is the only allowed AI base URL (`http://127.0.0.1:1320`).
- Do not enable OpenRouter cloud or Ollama for host AI features.
- Do not use embedded llama.cpp for host AI features.
- Do not edit `providers/emailops` to wire Polypus.
