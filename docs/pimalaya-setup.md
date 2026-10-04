# Pimalaya lane setup (mail sync + pimdir)

**Default mail path for should-i-read.** The host installs and invokes a mail sync dependency (Neverest) that writes a local **pimdir** store; the Go CLI reads SQLite and writes report-only artifacts. Operators use `should-i-read` / Make only, starting with `make configure`.

## Operator checklist

```bash
make init && make build
make configure
# or: make configure ARGS='--apply --provider gmail --email you@example.com'
./bin/should-i-read config bump    # when Polypus URL was literal localhost in YAML
make configure --json
make polypus-check
make readiness && make sync && make export
```

`make build` does not install the mail sync dependency. `configure` installs or verifies it as part of onboarding. Agents: see [AGENTS.md](../AGENTS.md) operator setup.

## Mailbox onboarding (`configure`)

One hub for host config, OAuth clients, mail dependency, sync profile, mailbox login, and local store init:

```bash
make configure
# or: make configure ARGS='--apply --provider gmail --email you@example.com'
./bin/should-i-read configure --json   # readiness checklist
./bin/should-i-read mail status        # mail-only JSON checklist
```

`configure` writes `~/.config/should-i-read/mail-sync.toml`, patches `pimalaya.*` in host config, runs browser login on a TTY when needed, then initializes the local store. Scripts pass `--apply` plus `--provider` / `--email` for automation.

## Maintainer mapping (scripts / advanced)

The sync engine is Neverest (`v0.x`, beta). Host scripts and `pim` subcommands remain for automation:

```bash
make ensure                         # should-i-read pim ensure
./bin/should-i-read pim configure ...
./bin/should-i-read pim ensure --check-only
```

Do not teach these verbs in operator help; use `mail setup` / `mail status` instead.

`make readiness` and `make sync` run the dependency check first. The host resolves `pimalaya.neverest_bin` / `NEVEREST_BIN` / PATH / `$CARGO_HOME/bin` so operators do not need cargo bin on PATH.

Maintainer alternatives (not the operator path): release artifacts or `cargo install --git https://github.com/pimalaya/neverest`.

## First-time mail sync config

Host-owned path: use `mail setup` above. Maintainer reference for hand-edited TOML: `tmp/pimalaya/neverest/config.sample.toml` when ecosystem clones are present.

After configure, prefer host checks:

```bash
make readiness    # should-i-read mail readiness
make sync         # should-i-read mail sync
```

## Gmail IMAP with XOAUTH2

Gmail IMAP often uses SASL **XOAUTH2**. Neverest can call an external command for a bearer token (`token.command` in TOML).

Use the host helper (prints access token to stdout only):

```toml
# ~/.config/neverest/config.toml (illustrative; match your account block)
[accounts.gmail.imap.sasl.xoauth2]
token.command = "should-i-read"
token.command_args = ["token", "gmail"]
```

One-time login (stores full OAuth token JSON in OS keyring; refresh is automatic):

```bash
should-i-read token gmail login
```

Neverest can call the same command on every sync; it refreshes when the access token expires and updates keyring before printing stdout:

```bash
should-i-read token gmail
```

Prerequisites: Gmail OAuth **client** id/secret via `should-i-read setup` or keyring (`EMAILOPS_GMAIL_CLIENT_ID`, `EMAILOPS_GMAIL_CLIENT_SECRET`). Use `should-i-read token gmail status` to confirm token storage `(set)`.

Legacy: if you previously set `GMAIL_OAUTH_REFRESH_TOKEN` manually, the broker still loads it until the first successful refresh writes `GMAIL_OAUTH_TOKEN` JSON.

Upstream Gmail-as-source in Neverest may still be incomplete; IMAP + XOAUTH2 is the supported bridge pattern for this host.

### Outlook IMAP with XOAUTH2

Same Neverest `token.command` pattern; the host stores an opaque **MSAL cache** blob in keyring (`OUTLOOK_MSAL_CACHE` or `OUTLOOK_MSAL_CACHE__<label>`), not a pasted refresh token.

```toml
[accounts.outlook.imap.sasl.xoauth2]
token.command = "should-i-read"
token.command_args = ["token", "outlook"]
```

```bash
should-i-read token outlook login
should-i-read token outlook          # silent refresh + cache write-back
should-i-read token outlook status   # redacted client + cache (set)/(unset)
```

Prerequisites: Entra **public client** id via `should-i-read setup` or keyring (`EMAILOPS_OUTLOOK_CLIENT_ID`). Register redirect `http://localhost` for the desktop app. API permissions must include **IMAP.AccessAsUser.All** (Office 365 Exchange Online).

### Multiple Neverest accounts (one keyring namespace per label)

Use `--account` on the host `token` subcommand so each Neverest account block can keep its own keyring entry. The label is arbitrary (for example `personal`, `work`); it does not have to match the mailbox address.

```toml
[accounts.personal.imap.sasl.xoauth2]
token.command = "should-i-read"
token.command_args = ["token", "gmail", "--account", "personal"]

[accounts.work.imap.sasl.xoauth2]
token.command_args = ["token", "gmail", "--account", "work"]
```

Login once per label:

```bash
should-i-read token gmail login --account personal
should-i-read token gmail login --account work
```

Optional alignment with host config: set `pimalaya.default_account` to the same label you use in docs or scripts (Neverest TOML still needs explicit `command_args` per account).

## Sync vs export vs show

`mail sync` (Neverest) stores full message bytes under the pimdir `objects/` tree plus metadata in `pimdir.db`. `mail export` / `make export` only writes summary fields into JSON (subject, sender, date, `object_hash`, etc.). To read body text for one message after sync, use `should-i-read mail show <object_hash>` (copy `object_hash` from the export file). Use `--raw` to dump the full RFC822 blob.

`mail status` is the configure onboarding checklist; `mail readiness` is the Neverest sync-engine check (credentials and IMAP).

## Host commands

| Command | Role |
|---------|------|
| `should-i-read mail setup` | Mailbox onboarding (deprecated: use configure) |
| `should-i-read mail status` | Onboarding checklist (JSON) |
| `should-i-read mail readiness` | Check sync engine (credentials and IMAP) |
| `should-i-read mail sync` | Sync remote mail into local store |
| `should-i-read mail export` | Read local store → `tmp/mail-export-<ts>.json` (summaries only; default 25 rows) |
| `should-i-read mail show <ref>` | Show one synced message body (`object_hash` or Message-ID from export JSON; `--raw` for full RFC822) |
| `should-i-read token gmail login` | One-time browser login; keyring stores refresh material |
| `should-i-read token gmail` | Access token for mail sync XOAUTH2 (auto-refresh) |
| `should-i-read token gmail status` | Redacted client + token storage status |
| `should-i-read token outlook login` | One-time MSAL browser login; keyring stores MSAL cache |
| `should-i-read token outlook` | Access token for mail sync XOAUTH2 (MSAL silent refresh) |
| `should-i-read token outlook status` | Redacted client + MSAL cache status |
| `should-i-read polypus check` | Fail-closed Polypus probe (`POLYPUS_BASE_URL` / config) |

Exit **7** means the sync engine returned exit **2** (needs human review: conflicts, duplicates, blocked writes). Mailbox is unchanged.

## Polypus

All AI uses `POLYPUS_BASE_URL` (default `http://127.0.0.1:1320` when unset). See [ai-provider-seam.md](ai-provider-seam.md).

## Maintainer mapping

Under the hood, `mail readiness` / `mail sync` call Neverest `--json` internally. Operator-facing stdout is a short summary; pass `--json` on the host CLI (or `SHOULD_I_READ_JSON=1`) for the full engine payload. Hidden `pim` subcommands remain for maintainers. Use upstream Neverest docs only when debugging the sync engine itself.