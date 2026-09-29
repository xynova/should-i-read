package token

import (
	"context"
	"strings"

	"golang.org/x/oauth2"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/oauthcred"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// GmailBroker implements Provider for Gmail IMAP XOAUTH2.
type GmailBroker struct {
	cfg     config.Config
	account string
}

// CreateGmailBroker returns a broker for the given account label (empty = default).
func CreateGmailBroker(cfg config.Config, account string) *GmailBroker {
	return &GmailBroker{cfg: cfg, account: strings.TrimSpace(account)}
}

// Login runs browser PKCE login and stores tokens in keyring.
func (b *GmailBroker) Login(ctx context.Context) error {
	if b == nil {
		return sirerr.New(sirerr.CodeInvalid, "token.GmailBroker.Login", "nil broker")
	}
	return GmailLogin(ctx, b.cfg, b.account)
}

// AccessToken refreshes if needed, persists updates, returns access token only.
func (b *GmailBroker) AccessToken(ctx context.Context) (string, error) {
	if b == nil {
		return "", sirerr.New(sirerr.CodeInvalid, "token.GmailBroker.AccessToken", "nil broker")
	}
	return gmailAccessToken(ctx, b.cfg, b.account)
}

// Logout removes keyring token material.
func (b *GmailBroker) Logout(ctx context.Context) error {
	if b == nil {
		return sirerr.New(sirerr.CodeInvalid, "token.GmailBroker.Logout", "nil broker")
	}
	_ = ctx
	return DeleteGmailToken(b.account)
}

// Status returns redacted credential status.
func (b *GmailBroker) Status() map[string]string {
	st := oauthcred.Status()
	key := GmailTokenKey(b.account)
	st[key] = GmailTokenStatus(b.account)
	return st
}

// GmailAccessToken is the default-account Neverest hook.
func GmailAccessToken(ctx context.Context, cfg config.Config) (string, error) {
	return gmailAccessToken(ctx, cfg, "")
}

func gmailAccessToken(ctx context.Context, cfg config.Config, account string) (string, error) {
	const op = "token.gmailAccessToken"
	if ctx == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	conf, err := gmailOAuthConfig(cfg)
	if err != nil {
		return "", err
	}
	saved, err := LoadGmailToken(account)
	if err != nil {
		return "", err
	}
	refreshed, err := conf.TokenSource(ctx, saved).Token()
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeAuth, op, "refresh gmail access token")
	}
	if refreshed.AccessToken == "" {
		return "", sirerr.New(sirerr.CodeAuth, op, "empty access token")
	}
	if TokenChanged(saved, refreshed) {
		mergeRefreshToken(saved, refreshed)
		if err := SaveGmailToken(account, refreshed); err != nil {
			return "", err
		}
	}
	return refreshed.AccessToken, nil
}

// mergeRefreshToken keeps the previous refresh token when Google omits a new one.
func mergeRefreshToken(before, after *oauth2.Token) {
	if after.RefreshToken == "" && before != nil && before.RefreshToken != "" {
		after.RefreshToken = before.RefreshToken
	}
}

// GmailCredentialStatus returns redacted oauth client + token storage status.
func GmailCredentialStatus(account string) map[string]string {
	b := CreateGmailBroker(config.Config{}, account)
	return b.Status()
}
