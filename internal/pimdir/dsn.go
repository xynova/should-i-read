package pimdir

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func roSQLiteDSN(dbPath string) string {
	return fmt.Sprintf("file:%s?mode=ro", filepath.ToSlash(dbPath))
}

func (r *Reader) openRo(ctx context.Context, op string) (*sql.DB, error) {
	if r == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil reader")
	}
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing context deadline")
	}
	db, err := sql.Open("sqlite", roSQLiteDSN(r.dbPath))
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "open sqlite")
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "pragma foreign_keys")
	}
	return db, nil
}
