package token

import (
	"context"
	"strings"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// outlookIMAPScopes are required for Outlook IMAP XOAUTH2 (Microsoft legacy protocol OAuth).
var outlookIMAPScopes = []string{
	"https://outlook.office365.com/IMAP.AccessAsUser.All",
	"offline_access",
	"openid",
}

func newOutlookClient(cfg config.Config, account string) (public.Client, error) {
	const op = "token.newOutlookClient"
	clientID := strings.TrimSpace(cfg.OutlookClientID)
	if clientID == "" {
		return public.Client{}, sirerr.New(sirerr.CodeAuth, op, "outlook client id unset; run should-i-read setup")
	}
	accessor := &keyringMSALCache{account: strings.TrimSpace(account)}
	return public.New(clientID, public.WithCache(accessor))
}

// OutlookLogin runs interactive MSAL login and persists cache to keyring via Export.
func OutlookLogin(ctx context.Context, cfg config.Config, account string) error {
	const op = "token.OutlookLogin"
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	pca, err := newOutlookClient(cfg, account)
	if err != nil {
		return err
	}
	result, err := pca.AcquireTokenInteractive(ctx, outlookIMAPScopes, public.WithRedirectURI("http://localhost"))
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeAuth, op, "interactive outlook login")
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return sirerr.New(sirerr.CodeAuth, op, "empty access token from msal")
	}
	return nil
}
