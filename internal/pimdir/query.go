package pimdir

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const summarySelect = `
SELECT i.collection, i.link_id, i.seq, m.subject, m.sender, m.sender_name, m.date, m.message_id, i.object_hash
FROM items i
JOIN mail_summary m ON m.item = i.id
WHERE i.deleted = 0`

// FindByObjectHash returns metadata for a live item with the given object hash.
func (r *Reader) FindByObjectHash(ctx context.Context, hash string) (EmailSummary, error) {
	const op = "pimdir.FindByObjectHash"
	if r == nil {
		return EmailSummary{}, sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return EmailSummary{}, sirerr.New(sirerr.CodeInvalid, op, "empty hash")
	}
	q := summarySelect + `
 AND i.object_hash = ?`
	return r.queryPreferredSummary(ctx, op, q, hash)
}

// FindByMessageID returns the first live item with a body for the given Message-ID.
func (r *Reader) FindByMessageID(ctx context.Context, messageID string) (EmailSummary, error) {
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
	return r.queryOneSummary(ctx, op, q, messageID)
}

func (r *Reader) queryOneSummary(ctx context.Context, op, q string, arg string) (EmailSummary, error) {
	db, err := r.openRo(ctx, op)
	if err != nil {
		return EmailSummary{}, err
	}
	defer func() { _ = db.Close() }()
	row := db.QueryRowContext(ctx, q, arg)
	s, err := scanEmailSummary(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailSummary{}, sirerr.New(sirerr.CodeNotFound, op, "mail summary not found").With("ref", arg)
		}
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query mail summary")
	}
	return s, nil
}

func (r *Reader) queryPreferredSummary(ctx context.Context, op, q string, arg string) (EmailSummary, error) {
	db, err := r.openRo(ctx, op)
	if err != nil {
		return EmailSummary{}, err
	}
	defer func() { _ = db.Close() }()
	rows, err := db.QueryContext(ctx, q, arg)
	if err != nil {
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query mail summary")
	}
	defer func() { _ = rows.Close() }()
	var picked EmailSummary
	found := false
	for rows.Next() {
		s, err := scanEmailSummaryRow(rows)
		if err != nil {
			return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan mail summary")
		}
		picked = keepPreferredSummary(picked, s, found)
		found = true
	}
	if err := rows.Err(); err != nil {
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate mail summary")
	}
	if !found {
		return EmailSummary{}, sirerr.New(sirerr.CodeNotFound, op, "mail summary not found").With("ref", arg)
	}
	return picked, nil
}

// ListLiveObjectHashesForFolder returns live object hashes whose collection is folder
// (Neverest name, for example INBOX) or ends with /folder (pimdir gmail/imap/INBOX).
func (r *Reader) ListLiveObjectHashesForFolder(ctx context.Context, folder string) ([]string, error) {
	const op = "pimdir.ListLiveObjectHashesForFolder"
	if r == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing context deadline")
	}
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty folder")
	}
	db, err := r.openRo(ctx, op)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	q := `
SELECT DISTINCT i.object_hash
FROM items i
WHERE i.deleted = 0
  AND i.object_hash IS NOT NULL AND TRIM(i.object_hash) != ''
  AND (i.collection = ? OR i.collection LIKE '%/' || ?)`
	rows, err := db.QueryContext(ctx, q, folder, folder)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query hashes")
	}
	defer func() { _ = rows.Close() }()
	var out []string
	seen := map[string]struct{}{}
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan hash")
		}
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		out = append(out, hash)
	}
	if err := rows.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate hashes")
	}
	return out, nil
}

// SummariesByObjectHash returns one mail summary per object hash when the row is live in pimdir.
// When a hash is in several folders, INBOX wins over Important over All Mail. Trash loses.
// Missing hashes are omitted. Duplicate hashes in the input are deduplicated.
func (r *Reader) SummariesByObjectHash(ctx context.Context, hashes []string) (map[string]EmailSummary, error) {
	const op = "pimdir.SummariesByObjectHash"
	if r == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	uniq := dedupeObjectHashes(hashes)
	if len(uniq) == 0 {
		return map[string]EmailSummary{}, nil
	}
	db, err := r.openRo(ctx, op)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	out := make(map[string]EmailSummary, len(uniq))
	const batchSize = 400
	for i := 0; i < len(uniq); i += batchSize {
		j := i + batchSize
		if j > len(uniq) {
			j = len(uniq)
		}
		if err := r.mergeSummaryBatch(ctx, op, db, uniq[i:j], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func dedupeObjectHashes(hashes []string) []string {
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
	return uniq
}

func (r *Reader) mergeSummaryBatch(ctx context.Context, op string, db *sql.DB, chunk []string, out map[string]EmailSummary) error {
	placeholders := sqlPlaceholders(len(chunk))
	q := summarySelect + `
 AND i.object_hash IN (` + placeholders + `)`
	args := make([]any, len(chunk))
	for k, h := range chunk {
		args[k] = h
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "query batch")
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		s, err := scanEmailSummaryRow(rows)
		if err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "scan batch row")
		}
		h := strings.TrimSpace(s.ObjectHash)
		if h == "" {
			continue
		}
		prev, ok := out[h]
		out[h] = keepPreferredSummary(prev, s, ok)
	}
	if err := rows.Err(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate batch")
	}
	return nil
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
