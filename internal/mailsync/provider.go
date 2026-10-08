package mailsync

import (
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// Provider is a supported mail sync OAuth provider.
type Provider string

const (
	ProviderGmail   Provider = "gmail"
	ProviderOutlook Provider = "outlook"
)

// ParseProvider normalizes and validates a provider name.
func ParseProvider(raw string) (Provider, error) {
	const op = "mailsync.ParseProvider"
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ProviderGmail):
		return ProviderGmail, nil
	case string(ProviderOutlook):
		return ProviderOutlook, nil
	default:
		return "", sirerr.New(sirerr.CodeInvalid, op, "unsupported provider").With("provider", raw)
	}
}

func (p Provider) imapServer() string {
	switch p {
	case ProviderOutlook:
		return "outlook.office365.com"
	default:
		return "imap.gmail.com"
	}
}

func (p Provider) tokenSubcommand() string {
	return string(p)
}
