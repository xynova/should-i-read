package hoststore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const classifyLease = 15 * time.Minute

// Store is the host classify ledger.
type Store struct {
	db   *sql.DB
	path string
	now  time.Time
}

// CreateStore opens the ledger, runs migrations, and returns a Store.
func CreateStore(cfg StoreConfig) (*Store, error) {
	const op = "hoststore.CreateStore"
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty store path")
	}
	if cfg.Now.IsZero() {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "store Now clock is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "abs store path")
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir store parent")
	}
	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)",
		filepath.ToSlash(abs),
	)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "open sqlite")
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "ping sqlite")
	}
	if err := applyMigrations(db, abs); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(abs, 0o600); err != nil {
		_ = db.Close()
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "chmod store")
	}
	return &Store{db: db, path: abs, now: cfg.Now.UTC()}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Path returns the absolute database file path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) requireStore(ctx context.Context, op string) error {
	if s == nil || s.db == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return sirerr.New(sirerr.CodeInvalid, op, "missing context deadline")
	}
	return nil
}

func (s *Store) stamp() string {
	return s.now.UTC().Format(time.RFC3339)
}
