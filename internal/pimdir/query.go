package pimdir

import (
	"database/sql"
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
	defer db.Close()
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "pragma foreign_keys")
	}
	row := db.QueryRow(q, arg)
	s, err := scanEmailSummary(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return EmailSummary{}, sirerr.New(sirerr.CodeNotFound, op, "mail summary not found").With("ref", arg)
		}
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query mail summary")
	}
	return s, nil
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
