package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// openTestDB opens a temporary SQLite DB and returns it with a cleanup func.
func openTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db, func() { db.Close() }
}

func TestOpen_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "naqb.db")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected DB file to exist at %s: %v", path, err)
	}
}

func TestOpen_Idempotent(t *testing.T) {
	// Opening and closing the same file twice should not error (goose is idempotent).
	dir := t.TempDir()
	path := filepath.Join(dir, "naqb.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open (should be idempotent): %v", err)
	}
	db2.Close()
}

func TestOpen_TablesExist(t *testing.T) {
	db, cleanup := openTestDB(t)
	defer cleanup()

	for _, table := range []string{"sessions", "messages", "jobs"} {
		row := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table)
		var name string
		if err := row.Scan(&name); err != nil {
			t.Errorf("table %q should exist after Open: %v", table, err)
		}
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if path == "" {
		t.Error("DefaultPath returned empty string")
	}
	if filepath.Base(path) != "naqb.db" {
		t.Errorf("DefaultPath base = %q, want naqb.db", filepath.Base(path))
	}
}
