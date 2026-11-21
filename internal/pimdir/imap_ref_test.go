package pimdir

import (
	"context"
	"testing"
	"time"
)

func TestFindByIMAPRef_seqAndCollection(t *testing.T) {
	dir := t.TempDir()
	initFixtureDB(t, dir)
	reader, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sum, err := reader.FindByIMAPRef(ctx, "INBOX", "1")
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
