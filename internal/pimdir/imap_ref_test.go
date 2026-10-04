package pimdir

import (
	"testing"
)

func TestFindByIMAPRef_seqAndCollection(t *testing.T) {
	dir := t.TempDir()
	initFixtureDB(t, dir)
	reader, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := reader.FindByIMAPRef("INBOX", "1")
	if err != nil {
		t.Fatal(err)
	}
	if sum.ObjectHash != "abcd1234efgh5678" {
		t.Fatalf("hash: %q", sum.ObjectHash)
	}
	if sum.Subject != "Hello pimdir" {
		t.Fatalf("subject: %q", sum.Subject)
	}
}
