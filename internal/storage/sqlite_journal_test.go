package storage

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureSQLiteJournalModeReadOnlyFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "readonly.db")
	db, err := InitializeSQLiteWithJournalMode(path, "DELETE", newTestKeyring(t))
	requireStartupSuccess(t, err)
	requireStartupSuccess(t, db.Close())
	readonly, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	requireStartupSuccess(t, err)
	t.Cleanup(func() { _ = readonly.Close() })
	// A read-only DELETE database cannot switch to WAL. SQLite can still
	// acknowledge DELETE, exercising the supported fallback without deleting
	// files or depending on filesystem permission behavior.
	requireStartupSuccess(t, ConfigureSQLitePragmas(readonly, "WAL"))
	var mode string
	scanStartupSQL(t, readonly.QueryRow("PRAGMA journal_mode"), &mode)
	requireStartupEqual(t, "read-only fallback journal mode", mode, "delete")
}

func TestConfigureSQLiteJournalModeFailureDiagnostics(t *testing.T) {
	for _, mode := range []string{"DELETE", "WAL"} {
		t.Run(mode, func(t *testing.T) {
			db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "closed.db"))
			requireStartupSuccess(t, err)
			requireStartupSuccess(t, db.Close())
			err = configureSQLiteJournalMode(db, mode)
			requireStartupEqual(t, "journal error returned", err != nil, true)
			prefix := "set journal mode " + mode + ":"
			if mode == "WAL" {
				prefix = "set journal mode fallback DELETE:"
			}
			requireStartupEqual(t, "journal error context", strings.HasPrefix(err.Error(), prefix), true)
			requireStartupEqual(t, "underlying journal error wrapped", errors.Unwrap(err) != nil, true)
		})
	}
}
