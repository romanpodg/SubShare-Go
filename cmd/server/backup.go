package main

import (
	"database/sql"
	"fmt"
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
	previousPath := backupPath + ".previous"
	_ = os.Remove(tempPath)
	if _, err := a.db.Exec(`VACUUM INTO ?`, filepath.ToSlash(tempPath)); err != nil {
		return fmt.Errorf("create SQLite backup: %w", err)
	}
	if err := verifySQLiteBackup(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("verify SQLite backup: %w", err)
	}

	_ = os.Remove(previousPath)
	hadPrevious := false
	if _, err := os.Stat(backupPath); err == nil {
		if err := os.Rename(backupPath, previousPath); err != nil {
			_ = os.Remove(tempPath)
			return fmt.Errorf("preserve previous backup: %w", err)
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		_ = os.Remove(tempPath)
		return fmt.Errorf("inspect previous backup: %w", err)
	}
	if err := os.Rename(tempPath, backupPath); err != nil {
		if hadPrevious {
			_ = os.Rename(previousPath, backupPath)
		}
		_ = os.Remove(tempPath)
		return fmt.Errorf("publish backup: %w", err)
	}
	if hadPrevious {
		_ = os.Remove(previousPath)
	}
	return nil
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

	lastID := int64(0)
	totalScanned := 0
	for {
		rows, err := db.Query(`SELECT vless_key_id, encrypted_url FROM vless_key_secrets WHERE vless_key_id > ? ORDER BY vless_key_id ASC LIMIT 1000`, lastID)
		if err != nil {
			return fmt.Errorf("query backup secrets: %w", err)
		}
		count := 0
		for rows.Next() {
			count++
			totalScanned++
			var id int64
			var env string
			if err := rows.Scan(&id, &env); err != nil {
				rows.Close()
				return fmt.Errorf("scan backup secret: %w", err)
			}
			lastID = id
			sec, err := profilestorage.Decrypt(env, keyring, id)
			if err != nil {
				rows.Close()
				return fmt.Errorf("backup row %d decryption failure: %w", id, err)
			}
			if sec.IsZero() {
				rows.Close()
				return fmt.Errorf("backup row %d decrypted to zero bytes", id)
			}
		}
		rows.Close()
		if count == 0 {
			break
		}
	}

	log.Printf("Backup validation succeeded: scanned %d encrypted profile secret(s).", totalScanned)
	return nil
}
