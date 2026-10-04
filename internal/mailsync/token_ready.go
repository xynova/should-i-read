package mailsync

import (
	"strings"

	"github.com/xynova/should-i-read/internal/token"
)

func oauthTokenReady(provider Provider, account string) bool {
	label := strings.TrimSpace(account)
	if label == "" {
		label = string(provider)
	}
	switch provider {
	case ProviderGmail:
		_, err := token.LoadGmailToken(label)
		return err == nil
	case ProviderOutlook:
		_, err := token.LoadOutlookMSALCache(label)
		return err == nil
	default:
		return false
	}
}
