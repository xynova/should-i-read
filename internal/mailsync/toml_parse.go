package mailsync

import (
	"regexp"
	"strings"
)

var xoauth2UsernameRE = regexp.MustCompile(`(?m)^imap\.sasl\.xoauth2\.username\s*=\s*"([^"]+)"`)
var imapServerRE = regexp.MustCompile(`(?m)^imap\.server\s*=\s*"([^"]+)"`)
var storeRootRE = regexp.MustCompile(`(?m)^store\.root\s*=\s*"([^"]+)"`)

func parseEmailFromMailSyncTOML(body string) string {
	m := xoauth2UsernameRE.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func parseAccountFromMailSyncTOML(body string) string {
	const prefix = "[accounts."
	idx := strings.Index(body, prefix)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(prefix):]
	end := strings.IndexByte(rest, ']')
	if end <= 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

// EmailFromMailSyncTOML exposes parse for host configure.
func EmailFromMailSyncTOML(body string) string {
	return parseEmailFromMailSyncTOML(body)
}

// AccountFromMailSyncTOML exposes parse for host configure.
func AccountFromMailSyncTOML(body string) string {
	return parseAccountFromMailSyncTOML(body)
}

func parseStoreRootFromMailSyncTOML(body string) string {
	m := storeRootRE.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// StoreRootFromMailSyncTOML exposes parse for host configure.
func StoreRootFromMailSyncTOML(body string) string {
	return parseStoreRootFromMailSyncTOML(body)
}

func parseIMAPServerFromMailSyncTOML(body string) string {
	m := imapServerRE.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// InferProviderFromMailSyncTOML returns gmail/outlook from IMAP host or account block name.
func InferProviderFromMailSyncTOML(body string) Provider {
	if host := parseIMAPServerFromMailSyncTOML(body); host != "" {
		if p := providerFromIMAPHost(host); p != "" {
			return p
		}
	}
	if acct := parseAccountFromMailSyncTOML(body); acct != "" {
		return inferProvider(acct)
	}
	return ""
}

func providerFromIMAPHost(host string) Provider {
	h := strings.ToLower(strings.TrimSpace(host))
	switch {
	case strings.Contains(h, "gmail.com"):
		return ProviderGmail
	case strings.Contains(h, "office365.com"), strings.Contains(h, "outlook.com"):
		return ProviderOutlook
	default:
		return ""
	}
}
