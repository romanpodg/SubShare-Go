package main

import (
	"database/sql"
	"github.com/romanpodg/SubShare-Go/internal/storage"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestExistingDatabaseSnapshotMigration is opt-in because CI has no user
// database. It snapshots a live WAL database with VACUUM INTO, migrates only
// the copy and verifies integrity. The source database is never modified.
func TestExistingDatabaseSnapshotMigration(t *testing.T) {
	sourcePath := os.Getenv("SUBSHARE_MIGRATION_SOURCE")
	if sourcePath == "" {
		t.Skip("SUBSHARE_MIGRATION_SOURCE is not set")
	}
	absoluteSource, err := filepath.Abs(sourcePath)
	if err != nil {
		t.Fatalf("resolve source database: %v", err)
	}
	if _, err := os.Stat(absoluteSource); err != nil {
		t.Fatalf("source database: %v", err)
	}

	snapshotPath := filepath.Join(t.TempDir(), "existing-snapshot.db")
	source, err := sql.Open("sqlite", filepath.ToSlash(absoluteSource))
	if err != nil {
		t.Fatalf("open source database: %v", err)
	}
	if _, err := source.Exec(`VACUUM INTO ?`, filepath.ToSlash(snapshotPath)); err != nil {
		_ = source.Close()
		t.Fatalf("create consistent snapshot: %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("close source database: %v", err)
	}

	migrated, err := initializeSQLite(snapshotPath)
	if err != nil {
		t.Fatalf("migrate snapshot: %v", err)
	}
	defer migrated.Close()

	var integrity string
	if err := migrated.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatalf("integrity check: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("snapshot integrity = %q, want ok", integrity)
	}
	var version int
	if err := migrated.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != storage.SchemaMigrations[len(storage.SchemaMigrations)-1].Version {
		t.Fatalf("schema version = %d, want %d", version, storage.SchemaMigrations[len(storage.SchemaMigrations)-1].Version)
	}
}
