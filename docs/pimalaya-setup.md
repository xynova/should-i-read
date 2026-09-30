# Pimalaya lane setup (Neverest + pimdir)

**Default mail path for should-i-read.** Neverest syncs into a local **pimdir** store; the host Go CLI reads SQLite and writes report-only artifacts. EmailOps is optional legacy custody ([emailops-setup.md](emailops-setup.md)).

## Operator checklist

```bash
make init && make setup
./bin/should-i-read config bump    # when Polypus URL was literal localhost in YAML
export POLYPUS_BASE_URL=...        # or keyring; expands ${POLYPUS_BASE_URL} in config
./bin/should-i-read token gmail login
make polypus-check
# set pimalaya.pimdir_path in ~/.config/should-i-read/config.yaml
make doctor && make sync && make export
```

## Install Neverest

Neverest is beta (`v0.x`). Pick one:

- CI artifacts from [pimalaya/neverest](https://github.com/pimalaya/neverest) releases
- `cargo install --git https://github.com/pimalaya/neverest`

Confirm on `PATH`:

```bash
neverest --version
```

## First-time Neverest config

```bash
neverest init
neverest check
```

Config lives at `~/.config/neverest/config.toml` (or `NEVEREST_CONFIG`). See `tmp/pimalaya/neverest/config.sample.toml` when ecosystem clones are present.

Point **should-i-read** at your account store directory (the folder that contains `pimdir.db`):

```yaml
pimalaya:
  pimdir_path: "/path/to/account/store"
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

## Host commands

| Command | Role |
|---------|------|
| `should-i-read pim doctor` | `neverest check --json` |
| `should-i-read pim sync` | `neverest sync --json` |
| `should-i-read pim snapshot` | Read `pimdir.db` → `tmp/pim-snapshot-<ts>.json` |
| `should-i-read token gmail login` | One-time browser login; keyring stores refresh material |
| `should-i-read token gmail` | Access token for Neverest (auto-refresh) |
| `should-i-read token gmail status` | Redacted client + token storage status |
| `should-i-read token outlook login` | One-time MSAL browser login; keyring stores MSAL cache |
| `should-i-read token outlook` | Access token for Neverest (MSAL silent refresh) |
| `should-i-read token outlook status` | Redacted client + MSAL cache status |
| `should-i-read polypus check` | Fail-closed Polypus probe (`POLYPUS_BASE_URL` / config) |

Exit **7** means Neverest exit **2** (needs human review: conflicts, duplicates, blocked writes). Mailbox is unchanged.

## Polypus

All AI uses `POLYPUS_BASE_URL` (default `http://127.0.0.1:1320` when unset). See [ai-provider-seam.md](ai-provider-seam.md).

## Verify outside the host

```bash
neverest check
./scripts/check-polypus.sh
```
