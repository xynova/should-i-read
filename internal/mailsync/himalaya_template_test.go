package mailsync

import (
	"strings"
	"testing"
)

func TestRenderHimalayaTOML(t *testing.T) {
	t.Parallel()
	body, err := renderHimalayaTOML("gmail", "/data/with spaces/pim", "[Gmail]/Trash")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[accounts.gmail]",
		`pimdir.account = "gmail"`,
		`pimdir.root = "/data/with spaces/pim"`,
		`mailbox.alias.trash = "gmail/imap/[Gmail]/Trash"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	for _, bad := range []string{"imap.server", "smtp"} {
		if strings.Contains(body, bad) {
			t.Fatalf("unexpected %q in:\n%s", bad, body)
		}
	}
	if _, err := renderHimalayaTOML("gmail", "/data/pim", ""); err == nil {
		t.Fatal("expected empty trash alias to fail")
	}
	outlook, err := renderHimalayaTOML("outlook", "/data/pim", ProviderOutlook.trashMailbox())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(outlook, `mailbox.alias.trash = "outlook/imap/Deleted Items"`) {
		t.Fatalf("outlook trash alias:\n%s", outlook)
	}
}
