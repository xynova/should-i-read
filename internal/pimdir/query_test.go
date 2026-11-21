package pimdir

import (
	"context"
	"database/sql"
	"path/filepath"
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

func TestSummariesByObjectHashPrefersInbox(t *testing.T) {
	dir := t.TempDir()
	initFixtureDB(t, dir)
	db, err := sql.Open("sqlite", filepath.Join(dir, "pimdir.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO items (collection, link_id, seq, object_hash, sort_key, level, deleted) VALUES ('gmail/imap/[Gmail]/All Mail', 'msg-1', 99, 'abcd1234efgh5678', '2020-01-02T00:00:00Z', 2, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO mail_summary (item, message_id, subject, sender, date) VALUES (2, '<mid@example.com>', 'All Mail copy', 'alice@example.com', '2020-01-02T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m, err := r.SummariesByObjectHash(ctx, []string{"abcd1234efgh5678"})
	if err != nil {
		t.Fatal(err)
	}
	s := m["abcd1234efgh5678"]
	if s.Collection != "INBOX" || s.Seq != 1 {
		t.Fatalf("want INBOX seq 1, got %+v", s)
	}
	got, err := r.FindByObjectHash(ctx, "abcd1234efgh5678")
	if err != nil {
		t.Fatal(err)
	}
	if got.Collection != "INBOX" || got.Seq != 1 {
		t.Fatalf("FindByObjectHash %+v", got)
	}
}
