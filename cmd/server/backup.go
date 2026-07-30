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

func (a *App) startBackup() {
	backupPath := strings.TrimSpace(os.Getenv("BACKUP_PATH"))
	if backupPath == "" {
		return
	}
	interval, err := time.ParseDuration(strings.TrimSpace(os.Getenv("BACKUP_INTERVAL")))
	if err != nil || interval <= 0 {
		interval = time.Hour
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
