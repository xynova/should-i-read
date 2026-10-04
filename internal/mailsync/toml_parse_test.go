package mailsync

import "testing"

func TestParseEmailFromMailSyncTOML(t *testing.T) {
	t.Parallel()
	body := `[accounts.gmail]
imap.sasl.xoauth2.username = "me@example.com"
`
	if got := parseEmailFromMailSyncTOML(body); got != "me@example.com" {
		t.Fatalf("got %q", got)
	}
	if got := parseEmailFromMailSyncTOML("no match"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestParseAccountFromMailSyncTOML(t *testing.T) {
	t.Parallel()
	body := `[accounts.work]
imap.server = "imap.gmail.com"
`
	if got := parseAccountFromMailSyncTOML(body); got != "work" {
		t.Fatalf("got %q", got)
	}
}

func TestInferProviderFromWorkAccountGmailIMAP(t *testing.T) {
	t.Parallel()
	body := `[accounts.work]
imap.server = "imap.gmail.com"
`
	if got := InferProviderFromMailSyncTOML(body); got != ProviderGmail {
		t.Fatalf("got %q", got)
	}
}
