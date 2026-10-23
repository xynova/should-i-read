package pimdir

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func testDeadlineCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestListRecentEmailsFixture(t *testing.T) {
	dir := t.TempDir()
	initFixtureDB(t, dir)

	r, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := r.ListRecentEmails(testDeadlineCtx(t), 10)
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

func TestListRecentEmailsMissingDeadline(t *testing.T) {
	dir := t.TempDir()
	initFixtureDB(t, dir)
	r, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.ListRecentEmails(context.Background(), 10)
	if err == nil {
		t.Fatal("expected missing deadline")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("code: %v", err)
	}
}

func initFixtureDB(t *testing.T, dir string) {
	dbPath := filepath.Join(dir, "pimdir.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
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
INSERT INTO items (collection, link_id, seq, object_hash, sort_key, level, deleted) VALUES ('INBOX', 'msg-1', 1, 'abcd1234efgh5678', '2020-01-02T00:00:00Z', 2, 0);
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
INSERT INTO mail_summary (collection, link_id, message_id, subject, sender, date) VALUES ('INBOX', 'msg-1', '<mid@example.com>', 'Hello pimdir', 'alice@example.com', '2020-01-02T00:00:00Z');
`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	blobPath := filepath.Join(dir, "objects", "ab", "cd", "abcd1234efgh5678")
	if err := os.MkdirAll(filepath.Dir(blobPath), 0o750); err != nil {
		t.Fatal(err)
	}
	const blob = "From: alice@example.com\r\nSubject: Hello pimdir\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nHello body\r\n"
	if err := os.WriteFile(blobPath, []byte(blob), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFindByObjectHashAndMessageID(t *testing.T) {
	dir := t.TempDir()
	initFixtureDB(t, dir)
	r, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	byHash, err := r.FindByObjectHash("abcd1234efgh5678")
	if err != nil {
		t.Fatal(err)
	}
	if byHash.Subject != "Hello pimdir" {
		t.Fatalf("subject: %q", byHash.Subject)
	}
	byMID, err := r.FindByMessageID("<mid@example.com>")
	if err != nil {
		t.Fatal(err)
	}
	if byMID.ObjectHash != "abcd1234efgh5678" {
		t.Fatalf("hash: %q", byMID.ObjectHash)
	}
}

func TestReadBlobAndResolveShowRef(t *testing.T) {
	dir := t.TempDir()
	initFixtureDB(t, dir)
	r, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := r.ReadBlob("abcd1234efgh5678")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Hello body") {
		t.Fatalf("blob: %q", raw)
	}
	_, hash, err := r.ResolveShowRef("abcd1234efgh5678")
	if err != nil || hash != "abcd1234efgh5678" {
		t.Fatalf("hash ref: %v %q", err, hash)
	}
	_, hash, err = r.ResolveShowRef("<mid@example.com>")
	if err != nil || hash != "abcd1234efgh5678" {
		t.Fatalf("mid ref: %v %q", err, hash)
	}
}

func TestListEmailsIn(t *testing.T) {
	dir := t.TempDir()
	initWindowRangeFixtureDB(t, dir)
	r, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	window, err := CreateDateWindow("2026-09-01", "2026-09-08")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := r.ListEmailsIn(ctx, nil, window, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 30 in-window + sep1 lower bound; excludes aug31, sep8, empty key, deleted, other collection.
	if len(rows) != 31 {
		t.Fatalf("in-window count: %d want 31", len(rows))
	}
	subjects := map[string]bool{}
	for _, row := range rows {
		subjects[row.Subject] = true
	}
	if subjects["before-window"] || subjects["at-upper-bound"] || subjects["empty-sort-key"] ||
		subjects["deleted-row"] || subjects["other-collection"] {
		t.Fatalf("excluded rows present: %v", subjects)
	}
	if !subjects["at-lower-bound"] {
		t.Fatal("missing lower bound row")
	}
	if rows[0].Subject != "in-window-29" {
		t.Fatalf("newest first: %q", rows[0].Subject)
	}
	limited, err := r.ListEmailsIn(ctx, nil, window, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 2 {
		t.Fatalf("limit: %d", len(limited))
	}
	if limited[0].Subject != "in-window-29" || limited[1].Subject != "in-window-28" {
		t.Fatalf("limited order: %q %q", limited[0].Subject, limited[1].Subject)
	}
	filtered, err := r.ListEmailsIn(ctx, []string{"INBOX"}, window, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range filtered {
		if row.Collection != "INBOX" {
			t.Fatalf("collection filter: %q", row.Collection)
		}
	}
}

func initWindowRangeFixtureDB(t *testing.T, dir string) {
	initFixtureDB(t, dir)
	dbPath := filepath.Join(dir, "pimdir.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	_, _ = db.Exec(`INSERT INTO collections (id, kind, name) VALUES ('imap/Sent', 'message/rfc822', 'Sent')`)
	extra := []struct {
		link, sortKey, subject, collection string
		deleted                            int
	}{
		{"aug31", "2026-08-31T23:59:59Z", "before-window", "INBOX", 0},
		{"sep1", "2026-09-01T00:00:00Z", "at-lower-bound", "INBOX", 0},
		{"sep8", "2026-09-08T00:00:00Z", "at-upper-bound", "INBOX", 0},
		{"empty", "   ", "empty-sort-key", "INBOX", 0},
		{"del", "2026-09-02T00:00:00Z", "deleted-row", "INBOX", 1},
		{"sent", "2026-09-09T00:00:00Z", "other-collection", "imap/Sent", 0},
	}
	for _, e := range extra {
		hash := fmt.Sprintf("hash%s000000000000000000000000", e.link)
		if len(hash) > 32 {
			hash = hash[:32]
		}
		_, err := db.Exec(`INSERT INTO items (collection, link_id, seq, object_hash, sort_key, level, deleted)
VALUES (?, ?, 1, ?, ?, 2, ?)`, e.collection, e.link, hash, e.sortKey, e.deleted)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`INSERT INTO mail_summary (collection, link_id, message_id, subject, sender, date)
VALUES (?, ?, ?, ?, 'a@example.com', ?)`, e.collection, e.link, "<"+e.link+">", e.subject, e.sortKey)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 30; i++ {
		link := fmt.Sprintf("win%02d", i)
		sk := fmt.Sprintf("2026-09-03T%02d:00:00Z", i)
		subj := fmt.Sprintf("in-window-%d", i)
		hash := fmt.Sprintf("w%02d0000000000000000000000000000", i)
		if len(hash) > 32 {
			hash = hash[:32]
		}
		_, err := db.Exec(`INSERT INTO items (collection, link_id, seq, object_hash, sort_key, level, deleted)
VALUES ('INBOX', ?, 1, ?, ?, 2, 0)`, link, hash, sk)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(`INSERT INTO mail_summary (collection, link_id, message_id, subject, sender, date)
VALUES ('INBOX', ?, ?, ?, 'a@example.com', ?)`, link, "<"+link+">", subj, sk)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDecodeMessagePreview(t *testing.T) {
	raw := []byte("From: a@b.com\r\nSubject: T\r\nContent-Type: text/plain\r\n\r\nPlain text here.\r\n")
	prev, err := DecodeMessagePreview(raw, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prev.Body, "Plain text here") {
		t.Fatalf("body: %q", prev.Body)
	}
}

func TestDecodeMessagePreview_dropsHTMLStyleMetadata(t *testing.T) {
	raw := []byte("From: a@b.com\r\nSubject: T\r\nContent-Type: text/html\r\n\r\n" +
		"<style>@font-face { font-family: 'Glassdoor Sans'; }</style>" +
		"<p>Apparently, I gave the wrong answer.</p>\r\n")
	prev, err := DecodeMessagePreview(raw, 8192)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(prev.Body), "font-family") {
		t.Fatalf("style leaked: %q", prev.Body)
	}
	if !strings.Contains(prev.Body, "wrong answer") {
		t.Fatalf("want visible text: %q", prev.Body)
	}
}
