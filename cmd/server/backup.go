package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/storage"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func verifySQLiteBackup(path string) error {
	db, err := sql.Open("sqlite", filepath.ToSlash(path))
	if err != nil {
		return err
	}
	defer db.Close()
	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return err
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity check returned %q", integrity)
	}
	return nil
}

func (a *App) performBackup(backupPath string) error {
	backupPath = strings.TrimSpace(backupPath)
	if backupPath == "" {
		return fmt.Errorf("backup path is empty")
	}
	if err := a.validateBackupPath(backupPath); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	tempPath := backupPath + ".tmp"
	_ = os.Remove(tempPath)
	if _, err := a.db.Exec(`VACUUM INTO ?`, filepath.ToSlash(tempPath)); err != nil {
		return fmt.Errorf("create SQLite backup: %w", err)
	}
	if err := verifySQLiteBackup(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("verify SQLite backup: %w", err)
	}
	paths := sqliteBackupPaths{backup: backupPath, candidate: tempPath, previous: backupPath + ".previous"}
	return publishSQLiteBackup(paths, os.Rename)
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(absolute)
	if resolvedParent, resolveErr := filepath.EvalSymlinks(parent); resolveErr == nil {
		absolute = filepath.Join(resolvedParent, filepath.Base(absolute))
	}
	if runtime.GOOS == "windows" {
		absolute = strings.ToLower(absolute)
	}
	return absolute, nil
}

func (a *App) validateBackupPath(backupPath string) error {
	if strings.TrimSpace(a.dbPath) == "" {
		return nil
	}
	backupCanonical, err := canonicalPath(backupPath)
	if err != nil {
		return fmt.Errorf("resolve backup path: %w", err)
	}
	dbCanonical, err := canonicalPath(a.dbPath)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}

	backupArtifacts := []string{backupCanonical, backupCanonical + ".tmp", backupCanonical + ".previous"}
	databaseArtifacts := []string{
		dbCanonical,
		dbCanonical + "-wal",
		dbCanonical + "-shm",
		dbCanonical + "-journal",
		dbCanonical + ".tmp",
		dbCanonical + ".previous",
	}
	for _, backupArtifact := range backupArtifacts {
		for _, databaseArtifact := range databaseArtifacts {
			if backupArtifact == databaseArtifact {
				return fmt.Errorf("backup path conflicts with the live SQLite database")
			}
		}
	}
	return nil
}

func (a *App) startBackup(backupPath string, interval time.Duration) {
	backupPath = strings.TrimSpace(backupPath)
	if backupPath == "" {
		return
	}
	go func() {
		run := func() {
			if err := a.performBackup(backupPath); err != nil {
				log.Printf("backup failed: %v", err)
				return
			}
			log.Printf("backup completed: file=%s", filepath.Base(backupPath))
		}
		run()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	}()
}

func runValidateBackup(backupPath string, keyring *profilestorage.Keyring) error {
	if keyring == nil {
		return profilestorage.ErrMissingKeyring
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(backupPath)+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open backup read-only: %w", err)
	}
	defer db.Close()

	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity check returned %q", integrity)
	}

	var maxVersion int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&maxVersion); err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	if maxVersion < 11 {
		log.Printf("Backup schema version is %d (pre-encryption). Schema check passed.", maxVersion)
		return nil
	}
	return validateBackupProfiles(db, keyring)
}

func validateBackupProfiles(db *sql.DB, keyring *profilestorage.Keyring) error {
	if err := storage.VerifyStartupEnvelopesAndInvariants(context.Background(), db, keyring); err != nil {
		return fmt.Errorf("backup %w", err)
	}

	totalScanned, err := storage.NewRepository(db, keyring).VerifySecrets(context.Background())
	if err != nil {
		return fmt.Errorf("backup %w", err)
	}

	log.Printf("Backup validation succeeded: scanned %d encrypted profile secret(s).", totalScanned)
	return nil
}
