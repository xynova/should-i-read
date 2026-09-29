package token

import (
	"context"
	"strings"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/oauthcred"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// OutlookBroker implements Provider for Outlook IMAP XOAUTH2 via MSAL public client.
type OutlookBroker struct {
	cfg     config.Config
	account string
}

// CreateOutlookBroker returns a broker for the given account label (empty = default).
func CreateOutlookBroker(cfg config.Config, account string) *OutlookBroker {
	return &OutlookBroker{cfg: cfg, account: strings.TrimSpace(account)}
}

// Login runs browser MSAL login and stores opaque cache in keyring.
func (b *OutlookBroker) Login(ctx context.Context) error {
	if b == nil {
		return sirerr.New(sirerr.CodeInvalid, "token.OutlookBroker.Login", "nil broker")
	}
	return OutlookLogin(ctx, b.cfg, b.account)
}

// AccessToken silently refreshes when needed, persists cache updates, returns access token only.
func (b *OutlookBroker) AccessToken(ctx context.Context) (string, error) {
	if b == nil {
		return "", sirerr.New(sirerr.CodeInvalid, "token.OutlookBroker.AccessToken", "nil broker")
	}
	return outlookAccessToken(ctx, b.cfg, b.account)
}

// Logout removes keyring MSAL cache material.
func (b *OutlookBroker) Logout(ctx context.Context) error {
	if b == nil {
		return sirerr.New(sirerr.CodeInvalid, "token.OutlookBroker.Logout", "nil broker")
	}
	_ = ctx
	return DeleteOutlookMSALCache(b.account)
}

// Status returns redacted credential status.
func (b *OutlookBroker) Status() map[string]string {
	st := oauthcred.Status()
	key := OutlookMSALCacheKey(b.account)
	st[key] = OutlookMSALCacheStatus(b.account)
	return st
}

// OutlookAccessToken is the default-account Neverest hook.
func OutlookAccessToken(ctx context.Context, cfg config.Config) (string, error) {
	return outlookAccessToken(ctx, cfg, "")
}

func outlookAccessToken(ctx context.Context, cfg config.Config, account string) (string, error) {
	const op = "token.outlookAccessToken"
	if ctx == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	pca, err := newOutlookClient(cfg, account)
	if err != nil {
		return "", err
	}
	accounts, err := pca.Accounts(ctx)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeAuth, op, "list msal accounts")
	}
	if len(accounts) == 0 {
		return "", sirerr.New(sirerr.CodeAuth, op, "outlook msal cache unset; run should-i-read token outlook login")
	}
	acct := pickMSALAccount(accounts, account)
	result, err := pca.AcquireTokenSilent(ctx, outlookIMAPScopes, public.WithSilentAccount(acct))
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeAuth, op, "silent outlook token; try token outlook login")
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return "", sirerr.New(sirerr.CodeAuth, op, "empty access token")
	}
	return result.AccessToken, nil
}

func pickMSALAccount(accounts []public.Account, label string) public.Account {
	label = strings.TrimSpace(label)
	if label == "" || len(accounts) == 1 {
		return accounts[0]
	}
	lower := strings.ToLower(label)
	for _, a := range accounts {
		if strings.EqualFold(a.PreferredUsername, label) {
			return a
		}
		if strings.Contains(strings.ToLower(a.PreferredUsername), lower) {
			return a
		}
	}
	return accounts[0]
}
