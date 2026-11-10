package pimdir

import (
	"context"
	"testing"
	"time"
)

func TestSummariesByObjectHash(t *testing.T) {
	dir := t.TempDir()
	initFixtureDB(t, dir)
	r, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m, err := r.SummariesByObjectHash(ctx, []string{"abcd1234efgh5678", "missing", "abcd1234efgh5678"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Fatalf("want 1 entry, got %d", len(m))
	}
	s := m["abcd1234efgh5678"]
	if s.Subject != "Hello pimdir" || s.ObjectHash != "abcd1234efgh5678" {
		t.Fatalf("unexpected summary %+v", s)
	}
}
