// Package oauthcred resolves EmailOps OAuth client credentials.
//
// Resolve order:
//  1. Environment, then Keychain (via secret.Resolve)
//  2. Product-owned values embedded at release via -ldflags (never committed as literals in source)
//
// Client IDs are public config. A Google Desktop client_secret is compatibility
// metadata only when present; it is not a security boundary on a local machine.
package oauthcred

import (
	"errors"
	"strings"

	"github.com/xynova/should-i-read/internal/secret"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	// EnvGmailClientID is the Gmail OAuth client id env / Keychain account.
	EnvGmailClientID = "EMAILOPS_GMAIL_CLIENT_ID"
	// EnvGmailClientSecret is the optional Gmail Desktop client secret env / Keychain account.
	EnvGmailClientSecret = "EMAILOPS_GMAIL_CLIENT_SECRET"
	// EnvOutlookClientID is the Outlook public client id env / Keychain account.
	EnvOutlookClientID = "EMAILOPS_OUTLOOK_CLIENT_ID"
)

// Product-owned defaults may be set at release with -ldflags, for example:
//
//	-X github.com/xynova/should-i-read/internal/oauthcred.ProductGmailClientID=...
//
// Source builds leave these empty so operators use setup / Keychain / env.
var (
	ProductGmailClientID     string
	ProductGmailClientSecret string
	ProductOutlookClientID   string
)

// ErrNotFound means the credential is unset in env, Keychain, and product embed.
var ErrNotFound = errors.New("oauth credential not found")

// Resolve returns env/Keychain first, then a product-owned embed when present.
func Resolve(name string) (string, error) {
	const op = "oauthcred.Resolve"
	name = strings.TrimSpace(name)
	if name == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "credential name is empty")
	}
	if v, err := secret.Resolve(name); err == nil && strings.TrimSpace(v) != "" {
		return v, nil
	} else if err != nil && !errors.Is(err, secret.ErrNotFound) {
		return "", err
	}
	if v := productValue(name); v != "" {
		return v, nil
	}
	return "", sirerr.Wrap(ErrNotFound, sirerr.CodeNotFound, op, "credential unset").With("name", name)
}

// Status returns a redacted view of which credentials are available.
func Status() map[string]string {
	return map[string]string{
		EnvGmailClientID:     setUnset(lookup(EnvGmailClientID)),
		EnvGmailClientSecret: setUnset(lookup(EnvGmailClientSecret)),
		EnvOutlookClientID:   setUnset(lookup(EnvOutlookClientID)),
	}
}

// HasAny reports whether at least one OAuth client id is resolved.
func HasAny() bool {
	if v, err := Resolve(EnvGmailClientID); err == nil && v != "" {
		return true
	}
	if v, err := Resolve(EnvOutlookClientID); err == nil && v != "" {
		return true
	}
	return false
}

func lookup(name string) string {
	v, err := Resolve(name)
	if err != nil {
		return ""
	}
	return v
}

func productValue(name string) string {
	switch name {
	case EnvGmailClientID:
		return strings.TrimSpace(ProductGmailClientID)
	case EnvGmailClientSecret:
		return strings.TrimSpace(ProductGmailClientSecret)
	case EnvOutlookClientID:
		return strings.TrimSpace(ProductOutlookClientID)
	default:
		return ""
	}
}

func setUnset(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(unset)"
	}
	return "(set)"
}
