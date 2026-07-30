package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestPerformBackupCanReplacePreviousBackup(t *testing.T) {
	app := newIntegrationApp(t)
	if _, err := app.db.Exec(`INSERT INTO users(name, token, activation_code, status) VALUES('first', 'token-1', 'code-1', 'active')`); err != nil {
		t.Fatalf("insert first user: %v", err)
	}
	backupPath := filepath.Join(t.TempDir(), "external", "subshare.db")
	if err := app.performBackup(backupPath); err != nil {
		t.Fatalf("first backup: %v", err)
	}
	if _, err := app.db.Exec(`INSERT INTO users(name, token, activation_code, status) VALUES('second', 'token-2', 'code-2', 'active')`); err != nil {
		t.Fatalf("insert second user: %v", err)
	}
	if err := app.performBackup(backupPath); err != nil {
		t.Fatalf("replace backup: %v", err)
	}

	backup, err := sql.Open("sqlite", filepath.ToSlash(backupPath))
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer backup.Close()
	var users int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if users != 2 {
		t.Fatalf("backup user count = %d, want 2", users)
	}
}

func TestPerformBackupRejectsLiveDatabaseArtifacts(t *testing.T) {
	app := newIntegrationApp(t)
	databasePath := filepath.Join(t.TempDir(), "live.db")
	app.dbPath = databasePath

	for _, dangerousPath := range []string{
		databasePath,
		databasePath + "-wal",
		databasePath + "-shm",
		databasePath + "-journal",
		databasePath + ".tmp",
	} {
		if err := app.validateBackupPath(dangerousPath); err == nil {
			t.Fatalf("dangerous backup path %q was accepted", dangerousPath)
		}
	}
}
