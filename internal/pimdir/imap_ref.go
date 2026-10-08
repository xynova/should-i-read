package pimdir

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// FindByIMAPRef resolves a Neverest hunk (collection + uid) to a live item summary.
func (r *Reader) FindByIMAPRef(collection, uid string) (EmailSummary, error) {
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
			s, err := r.querySummary(op, summarySelect+`
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
		s, err := r.querySummary(op, summarySelect+`
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

func (r *Reader) querySummary(op, q string, args ...any) (EmailSummary, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(r.dbPath))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "open sqlite")
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "pragma foreign_keys")
	}
	row := db.QueryRow(q, args...)
	s, err := scanEmailSummary(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return EmailSummary{}, sirerr.New(sirerr.CodeNotFound, op, "mail summary not found")
		}
		return EmailSummary{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "query mail summary")
	}
	return s, nil
}
