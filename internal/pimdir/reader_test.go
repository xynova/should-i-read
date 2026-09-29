package pimdir

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestListRecentEmailsFixture(t *testing.T) {
	dir := t.TempDir()
	initFixtureDB(t, dir)

	r, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := r.ListRecentEmails(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("count: %d", len(rows))
	}
	if rows[0].Subject != "Hello pimdir" {
		t.Fatalf("subject: %q", rows[0].Subject)
	}
}

func initFixtureDB(t *testing.T, dir string) {
	dbPath := filepath.Join(dir, "pimdir.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := `
CREATE TABLE store_meta (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  format TEXT NOT NULL DEFAULT 'pimdir',
  version INTEGER NOT NULL,
  hash_algo TEXT NOT NULL,
  created_at TEXT NOT NULL,
  next_seq INTEGER NOT NULL DEFAULT 1,
  next_change INTEGER NOT NULL DEFAULT 1,
  purges INTEGER NOT NULL DEFAULT 0
);
INSERT INTO store_meta (id, format, version, hash_algo, created_at) VALUES (1, 'pimdir', 1, 'blake3', '2020-01-01T00:00:00Z');
CREATE TABLE collections (
  id TEXT PRIMARY KEY,
  account TEXT,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  parent TEXT,
  color TEXT,
  description TEXT,
  sort_order INTEGER,
  conflict TEXT NOT NULL DEFAULT 'manual',
  generation INTEGER NOT NULL DEFAULT 1,
  changed INTEGER NOT NULL DEFAULT 0
);
INSERT INTO collections (id, kind, name) VALUES ('INBOX', 'message/rfc822', 'INBOX');
CREATE TABLE items (
  collection TEXT NOT NULL,
  link_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  flags TEXT,
  object_hash TEXT,
  sort_key TEXT NOT NULL DEFAULT '',
  level INTEGER NOT NULL DEFAULT 1,
  deleted INTEGER NOT NULL DEFAULT 0,
  retained_at TEXT,
  retained_by TEXT,
  conflicted INTEGER NOT NULL DEFAULT 0,
  conflict_object TEXT,
  changed INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (collection, link_id)
);
INSERT INTO items (collection, link_id, seq, sort_key, level, deleted) VALUES ('INBOX', 'msg-1', 1, '2020-01-02T00:00:00Z', 1, 0);
CREATE TABLE mail_summary (
  collection TEXT NOT NULL,
  link_id TEXT NOT NULL,
  message_id TEXT,
  in_reply_to TEXT NOT NULL DEFAULT '[]',
  subject TEXT NOT NULL,
  sender TEXT,
  sender_name TEXT,
  date TEXT,
  size INTEGER,
  attachment INTEGER,
  PRIMARY KEY (collection, link_id)
);
INSERT INTO mail_summary (collection, link_id, subject, sender) VALUES ('INBOX', 'msg-1', 'Hello pimdir', 'alice@example.com');
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
}
