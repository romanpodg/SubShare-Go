package storage

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func InitializeSQLiteWithJournalMode(
	dbPath, journalMode string,
	keyring *profilestorage.Keyring,
) (*sql.DB, error) {
	db, err := sql.Open("sqlite", sqliteDataSourceName(dbPath))
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)

	if err := ConfigureSQLitePragmas(db, journalMode); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := MigrateWithKeyring(db, keyring); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return db, nil
}

func sqliteDataSourceName(dbPath string) string {
	dataSourceName := filepath.ToSlash(dbPath)
	separator := "?"
	if strings.Contains(dataSourceName, "?") {
		separator = "&"
	}
	pragmas := url.Values{}
	pragmas.Add("_pragma", "busy_timeout=5000")
	pragmas.Add("_pragma", "foreign_keys=ON")
	return dataSourceName + separator + pragmas.Encode()
}

func ConfigureSQLitePragmas(db *sql.DB, journalMode string) error {
	if err := configureSQLiteJournalMode(db, journalMode); err != nil {
		return err
	}

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -4000",
		"PRAGMA mmap_size = 268435456",
		"PRAGMA temp_store = MEMORY",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("exec %s: %w", pragma, err)
		}
	}

	return nil
}

func configureSQLiteJournalMode(db *sql.DB, journalMode string) error {
	_, err := db.Exec(fmt.Sprintf("PRAGMA journal_mode = %s", journalMode))
	if err == nil {
		return nil
	}
	if journalMode != "WAL" {
		return fmt.Errorf("set journal mode %s: %w", journalMode, err)
	}
	// Some bind mounts do not support WAL shared memory. SQLite owns this
	// fallback, including any checkpoint needed before switching to DELETE.
	log.Printf("WAL mode unavailable (%v), falling back to DELETE", err)
	if _, fallbackErr := db.Exec("PRAGMA journal_mode = DELETE"); fallbackErr != nil {
		return fmt.Errorf("set journal mode fallback DELETE: %w", fallbackErr)
	}
	return nil
}
