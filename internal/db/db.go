// Package db manages the naqb SQLite database (~/.naqb/naqb.db).
// It runs goose migrations automatically on Open, making it safe to call
// at startup on every invocation.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, no CGO required
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens (or creates) the naqb database at the given path and runs all
// pending migrations. The caller is responsible for calling db.Close().
//
// Recommended path: config.NaqbDir() + "/naqb.db"
// WAL mode and foreign key enforcement are enabled automatically.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("db: creating directory: %w", err)
	}

	// modernc.org/sqlite does not honour DSN pragma params; use a connect hook instead.
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}

	// Verify the connection is usable.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	// Enable WAL journal mode and foreign-key enforcement.
	// Must be executed before migrations so FK constraints apply from the start.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("db: %s: %w", pragma, err)
		}
	}

	// Run migrations from embedded SQL files.
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db: set dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db: migrations: %w", err)
	}

	return db, nil
}

// DefaultPath returns the canonical path for the naqb database.
// It uses ~/.naqb/naqb.db.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("db: home dir: %w", err)
	}
	return filepath.Join(home, ".naqb", "naqb.db"), nil
}
