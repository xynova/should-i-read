package pimalaya

import "testing"

func TestCollectionExcluded(t *testing.T) {
	t.Parallel()
	excludes := []string{"[Gmail]/Spam", "[Gmail]/Trash"}
	for _, name := range []string{"[Gmail]/Spam", "imap/[Gmail]/Spam", "[gmail]/spam"} {
		if !CollectionExcluded(name, excludes) {
			t.Fatalf("expected %q excluded", name)
		}
	}
	if CollectionExcluded("INBOX", excludes) {
		t.Fatal("INBOX must not be excluded")
	}
	if CollectionExcluded("[Gmail]/All Mail", excludes) {
		t.Fatal("All Mail must not be excluded")
	}
}
