# Product-owned OAuth apps (ops)

should-i-read uses **product-owned** Google and Microsoft OAuth *client* apps as the
normal path. End users only consent per mailbox. They do not create Cloud Console
or Entra apps.

Mailbox OAuth *tokens* stay in EmailOps' OS keychain on the device. This host does
not run a cloud token broker.

## Credential resolve order

1. Environment, then platform keyring (`should-i-read secret set …`, service `should-i-read`)
2. Optional `~/.config/should-i-read/secrets.enc.yaml` (SOPS) when present
3. Product-owned values embedded at **release** via Go `-ldflags` into
   `internal/oauthcred` (never commit live secrets to git)
4. `should-i-read setup` BYO paste or guided DIY (advanced)

Host config loading uses [operatorconfig](https://github.com/behaviorengineering/operatorconfig) for path discovery, init, and secret hops 1–2.

Inspect redacted status: `./bin/should-i-read config show` or setup summary.

## Register once (product owner)

### Google (Desktop app)

1. Create a Google Cloud project; enable **Gmail API**.
2. OAuth consent screen: **External**.
3. Create OAuth client → type **Desktop app**.
4. Store client id (and secret if issued) in Keychain or release ldflags:
   - `EMAILOPS_GMAIL_CLIENT_ID`
   - `EMAILOPS_GMAIL_CLIENT_SECRET` (compatibility only; not a security boundary on desktop)
5. While in Testing, add mailbox addresses as test users. Before wide consumer use,
   publish and complete Google OAuth verification (restricted Gmail scopes may need CASA).

### Microsoft (public client)

1. Entra app registration; accounts: organizational + personal when needed.
2. Platform: Mobile and desktop; redirect `http://localhost`.
3. Public client + PKCE; **do not** create a client secret.
4. Store Application (client) ID as `EMAILOPS_OUTLOOK_CLIENT_ID`.

## Release embed (optional)

```bash
go build -ldflags "\
  -X github.com/xynova/should-i-read/internal/oauthcred.ProductGmailClientID=... \
  -X github.com/xynova/should-i-read/internal/oauthcred.ProductOutlookClientID=..." \
  -o bin/should-i-read ./cmd/should-i-read
```

Prefer injecting the Google secret via Keychain at install when required by EmailOps.

## BYO / DIY

Operators who refuse product client ids: `make setup` → paste BYO credentials, or
advanced guided Cloud/Azure steps. See `docs/emailops-setup.md`.

## Trust

Owning the OAuth *app* identifies the client on the consent screen. It does not
grant the product operator standing remote access to user mailboxes. Users who
distrust product client ids can BYO. Auditable claims (open source, allowlists)
are documented separately; do not add a hosted OAuth broker.
