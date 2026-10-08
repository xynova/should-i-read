package pimdir

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// EmailSummary is a row from mail_summary joined with items.
type EmailSummary struct {
	Collection string `json:"collection"`
	LinkID     string `json:"link_id"`
	Seq        int64  `json:"seq"`
	Subject    string `json:"subject"`
	Sender     string `json:"sender,omitempty"`
	SenderName string `json:"sender_name,omitempty"`
	Date       string `json:"date,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	ObjectHash string `json:"object_hash,omitempty"`
}

// Reader opens a pimdir store read-only.
type Reader struct {
	storeDir string
	dbPath   string
}

// OpenStore opens the store directory (contains pimdir.db).
func OpenStore(storeDir string) (*Reader, error) {
	const op = "pimdir.OpenStore"
	dir := strings.TrimSpace(storeDir)
	if dir == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "pimdir path is empty")
	}
	dbPath := filepath.Join(dir, "pimdir.db")
	if st, err := os.Stat(dbPath); err != nil || st.IsDir() {
		return nil, sirerr.New(sirerr.CodeNotFound, op, "pimdir.db not found").With("path", dbPath)
	}
	return &Reader{storeDir: dir, dbPath: dbPath}, nil
}

// ListRecentEmails returns live mail rows newest-first by sort_key/seq.
func (r *Reader) ListRecentEmails(limit int) ([]EmailSummary, error) {
	return r.ListRecentEmailsIn(nil, limit)
}

// ListRecentEmailsIn returns newest-first rows. When collections is non-empty, only those folders match.
func (r *Reader) ListRecentEmailsIn(collections []string, limit int) ([]EmailSummary, error) {
	const op = "pimdir.ListRecentEmailsIn"
	if r == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	if limit <= 0 {
		limit = 25
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(r.dbPath))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "open sqlite")
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "pragma foreign_keys")
	}

	collections = normalizeCollections(collections)
	q := `
SELECT i.collection, i.link_id, i.seq, m.subject, m.sender, m.sender_name, m.date, m.message_id, i.object_hash
FROM items i
JOIN mail_summary m ON m.collection = i.collection AND m.link_id = i.link_id
WHERE i.deleted = 0`
	args := []any{}
	if len(collections) > 0 {
		q += ` AND i.collection IN (` + sqlPlaceholders(len(collections)) + `)`
		for _, c := range collections {
			args = append(args, c)
		}
	}
	q += `
ORDER BY i.sort_key DESC, i.seq DESC
LIMIT ?`
	args = append(args, limit)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query mail summaries")
	}
	defer rows.Close()

	out := make([]EmailSummary, 0, limit)
	for rows.Next() {
		var s EmailSummary
		var objectHash, sender, senderName, date, messageID sql.NullString
		if err := rows.Scan(&s.Collection, &s.LinkID, &s.Seq, &s.Subject, &sender, &senderName, &date, &messageID, &objectHash); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan row")
		}
		if objectHash.Valid {
			s.ObjectHash = objectHash.String
		}
		if sender.Valid {
			s.Sender = sender.String
		}
		if senderName.Valid {
			s.SenderName = senderName.String
		}
		if date.Valid {
			s.Date = date.String
		}
		if messageID.Valid {
			s.MessageID = messageID.String
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate rows")
	}
	return out, nil
}

func normalizeCollections(collections []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(collections))
	for _, c := range collections {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

func sqlPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

// ReadBlob returns raw bytes for an object hash under objects/.
func (r *Reader) ReadBlob(hash string) ([]byte, error) {
	const op = "pimdir.ReadBlob"
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty hash")
	}
	if len(hash) < 4 {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "hash too short")
	}
	rel := filepath.Join("objects", hash[0:2], hash[2:4], hash)
	path := filepath.Join(r.storeDir, rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeNotFound, op, "read blob").With("path", path)
	}
	return raw, nil
}
