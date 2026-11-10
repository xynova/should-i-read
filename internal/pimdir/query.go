package pimdir

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const summarySelect = `
SELECT i.collection, i.link_id, i.seq, m.subject, m.sender, m.sender_name, m.date, m.message_id, i.object_hash
FROM items i
JOIN mail_summary m ON m.collection = i.collection AND m.link_id = i.link_id
WHERE i.deleted = 0`

// FindByObjectHash returns metadata for a live item with the given object hash.
func (r *Reader) FindByObjectHash(hash string) (EmailSummary, error) {
	const op = "pimdir.FindByObjectHash"
	if r == nil {
		return EmailSummary{}, sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return EmailSummary{}, sirerr.New(sirerr.CodeInvalid, op, "empty hash")
	}
	q := summarySelect + `
 AND i.object_hash = ?
 LIMIT 1`
	return r.queryOneSummary(op, q, hash)
}

// FindByMessageID returns the first live item with a body for the given Message-ID.
func (r *Reader) FindByMessageID(messageID string) (EmailSummary, error) {
	const op = "pimdir.FindByMessageID"
	if r == nil {
		return EmailSummary{}, sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return EmailSummary{}, sirerr.New(sirerr.CodeInvalid, op, "empty message id")
	}
	q := summarySelect + `
 AND m.message_id = ?
 AND i.object_hash IS NOT NULL AND TRIM(i.object_hash) != ''
 LIMIT 1`
	return r.queryOneSummary(op, q, messageID)
}

func (r *Reader) queryOneSummary(op, q string, arg string) (EmailSummary, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(r.dbPath))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "open sqlite")
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "pragma foreign_keys")
	}
	row := db.QueryRow(q, arg)
	s, err := scanEmailSummary(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailSummary{}, sirerr.New(sirerr.CodeNotFound, op, "mail summary not found").With("ref", arg)
		}
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query mail summary")
	}
	return s, nil
}

// SummariesByObjectHash returns one mail summary per object hash when the row is live in pimdir.
// Missing hashes are omitted. Duplicate hashes in the input are deduplicated.
func (r *Reader) SummariesByObjectHash(ctx context.Context, hashes []string) (map[string]EmailSummary, error) {
	const op = "pimdir.SummariesByObjectHash"
	if r == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	uniq := make([]string, 0, len(hashes))
	seen := make(map[string]struct{}, len(hashes))
	for _, h := range hashes {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		uniq = append(uniq, h)
	}
	if len(uniq) == 0 {
		return map[string]EmailSummary{}, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(r.dbPath))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "open sqlite")
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "pragma foreign_keys")
	}
	out := make(map[string]EmailSummary, len(uniq))
	const batchSize = 400
	for i := 0; i < len(uniq); i += batchSize {
		j := i + batchSize
		if j > len(uniq) {
			j = len(uniq)
		}
		chunk := uniq[i:j]
		placeholders := sqlPlaceholders(len(chunk))
		q := summarySelect + `
 AND i.object_hash IN (` + placeholders + `)`
		args := make([]any, len(chunk))
		for k, h := range chunk {
			args[k] = h
		}
		rows, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query batch")
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			s, err := scanEmailSummaryRow(rows)
			if err != nil {
				return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan batch row")
			}
			h := strings.TrimSpace(s.ObjectHash)
			if h != "" {
				out[h] = s
			}
		}
		if err := rows.Err(); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate batch")
		}
	}
	return out, nil
}

func scanEmailSummary(row *sql.Row) (EmailSummary, error) {
	var s EmailSummary
	var objectHash, sender, senderName, date, messageID sql.NullString
	if err := row.Scan(&s.Collection, &s.LinkID, &s.Seq, &s.Subject, &sender, &senderName, &date, &messageID, &objectHash); err != nil {
		return EmailSummary{}, err
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
	return s, nil
}
