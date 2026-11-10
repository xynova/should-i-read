package maildelete

import "testing"

func TestNormalizeFolder(t *testing.T) {
	if got := NormalizeFolder("imap/INBOX"); got != "INBOX" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeFolder("INBOX"); got != "INBOX" {
		t.Fatalf("got %q", got)
	}
}

func TestIsTrashFolder(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"[Gmail]/Trash", true},
		{"imap/trash", true},
		{"INBOX", false},
		{"imap/INBOX", false},
	}
	for _, tc := range cases {
		if got := IsTrashFolder(tc.in); got != tc.want {
			t.Fatalf("%q: got %v", tc.in, got)
		}
	}
}
