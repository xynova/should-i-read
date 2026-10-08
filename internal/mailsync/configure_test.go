package mailsync

import (
	"strings"
	"testing"
)

func TestParseProvider(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want Provider
		ok   bool
	}{
		{"gmail", ProviderGmail, true},
		{"Gmail", ProviderGmail, true},
		{"outlook", ProviderOutlook, true},
		{"fastmail", "", false},
	} {
		got, err := ParseProvider(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("%q: expected error", tc.in)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestRenderAccountTOML(t *testing.T) {
	t.Parallel()
	body, err := renderAccountTOML(accountTemplateData{
		Account:          "gmail",
		IMAPServer:       "imap.gmail.com",
		Email:            "me@example.com",
		StoreRoot:        "/data/pim/gmail",
		TokenCommandTOML: `["/bin/should-i-read","token","gmail","--account","gmail"]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[accounts.gmail]",
		`imap.server = "imap.gmail.com"`,
		`imap.sasl.xoauth2.username = "me@example.com"`,
		`token.command = ["/bin/should-i-read","token","gmail","--account","gmail"]`,
		`store.root = "/data/pim/gmail"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestBuildTokenArgvMultiAccount(t *testing.T) {
	t.Parallel()
	argv := buildTokenArgv("/bin/sir", ProviderGmail, "work")
	if len(argv) != 5 || argv[4] != "work" {
		t.Fatalf("got %v", argv)
	}
	argv2 := buildTokenArgv("/bin/sir", ProviderGmail, "gmail")
	if len(argv2) != 5 || argv2[3] != "--account" || argv2[4] != "gmail" {
		t.Fatalf("got %v", argv2)
	}
}
