package pimdir

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// FindByIMAPRef resolves a Neverest hunk (collection + uid) to a live item summary.
func (r *Reader) FindByIMAPRef(ctx context.Context, collection, uid string) (EmailSummary, error) {
	const op = "pimdir.FindByIMAPRef"
	if r == nil {
		return EmailSummary{}, sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	collection = strings.TrimSpace(collection)
	uid = strings.TrimSpace(uid)
	if collection == "" || uid == "" {
		return EmailSummary{}, sirerr.New(sirerr.CodeInvalid, op, "collection and uid required")
	}
	candidates := []string{collection}
	if !strings.HasPrefix(collection, "imap/") {
		candidates = append(candidates, "imap/"+collection)
	}
	seq, seqErr := strconv.ParseInt(uid, 10, 64)
	for _, col := range candidates {
		if seqErr == nil {
			s, err := r.querySummary(ctx, op, summarySelect+`
 AND i.collection = ?
 AND i.seq = ?
 LIMIT 1`, col, seq)
			if err == nil {
				return s, nil
			}
			if code, ok := sirerr.AsCode(err); ok && code != sirerr.CodeNotFound {
				return EmailSummary{}, err
			}
		}
		s, err := r.querySummary(ctx, op, summarySelect+`
 AND i.collection = ?
 AND i.link_id = ?
 LIMIT 1`, col, uid)
		if err == nil {
			return s, nil
		}
		if code, ok := sirerr.AsCode(err); ok && code != sirerr.CodeNotFound {
			return EmailSummary{}, err
		}
	}
	return EmailSummary{}, sirerr.New(sirerr.CodeNotFound, op, "imap ref not found").
		With("collection", collection).With("uid", uid)
}

func (r *Reader) querySummary(ctx context.Context, op, q string, args ...any) (EmailSummary, error) {
	db, err := r.openRo(ctx, op)
	if err != nil {
		return EmailSummary{}, err
	}
	defer func() { _ = db.Close() }()
	row := db.QueryRowContext(ctx, q, args...)
	s, err := scanEmailSummary(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailSummary{}, sirerr.New(sirerr.CodeNotFound, op, "mail summary not found")
		}
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query mail summary")
	}
	return s, nil
}
