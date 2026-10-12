package main

import (
	"fmt"
	"os"
)

// Publication owns a verified candidate, its destination and recoverable prior
// artifact. The rename operation is local to one attempt, never a global hook.
type sqliteBackupPaths struct {
	backup, candidate, previous string
}

func preserveSQLiteBackup(paths sqliteBackupPaths, rename func(string, string) error) (bool, error) {
	_ = os.Remove(paths.previous)
	_, err := os.Stat(paths.backup)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		_ = os.Remove(paths.candidate)
		return false, fmt.Errorf("inspect previous backup: %w", err)
	}
	if err := rename(paths.backup, paths.previous); err != nil {
		_ = os.Remove(paths.candidate)
		return false, fmt.Errorf("preserve previous backup: %w", err)
	}
	return true, nil
}

func publishSQLiteBackup(paths sqliteBackupPaths, rename func(string, string) error) error {
	hadPrevious, err := preserveSQLiteBackup(paths, rename)
	if err != nil {
		return err
	}
	if err := rename(paths.candidate, paths.backup); err != nil {
		if hadPrevious {
			_ = rename(paths.previous, paths.backup)
		}
		_ = os.Remove(paths.candidate)
		return fmt.Errorf("publish backup: %w", err)
	}
	if hadPrevious {
		_ = os.Remove(paths.previous)
	}
	return nil
}
