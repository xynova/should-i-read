package hoststore

import (
	"database/sql"
	"embed"
	"errors"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/xynova/should-i-read/internal/sirerr"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func applyMigrations(db *sql.DB, dbPath string) error {
	const op = "hoststore.applyMigrations"
	source, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "create migration source")
	}
	driver, err := sqlite.WithInstance(db, &sqlite.Config{
		DatabaseName: filepath.ToSlash(dbPath),
	})
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "create sqlite migrate driver")
	}
	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "create migrator")
	}
	// Do not call m.Close(): it would close the shared *sql.DB we keep open in Store.
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "migrate up")
	}
	return nil
}
