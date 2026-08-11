package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func newTestKeyring(t *testing.T) *profilestorage.Keyring {
	t.Helper()
	data, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	kr, err := profilestorage.LoadKeyringJSON(data)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	return kr
}

func TestConfigureSQLitePragmas(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pragmas.db")
	db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if err := ConfigureSQLitePragmas(db, "DELETE"); err != nil {
		t.Fatalf("configure pragmas: %v", err)
	}

	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "delete" {
		t.Fatalf("journal_mode = %q, want 'delete'", journalMode)
	}

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}
}

func TestInitializeSQLiteWithJournalMode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "init.db")
	kr := newTestKeyring(t)

	migrated := false
	dummyMigrator := func(db *sql.DB, keyring *profilestorage.Keyring) error {
		migrated = true
		if keyring != kr {
			t.Fatalf("migrator received wrong keyring instance")
		}
		_, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY)`)
		return err
	}

	db, err := InitializeSQLiteWithJournalMode(dbPath, "DELETE", kr, dummyMigrator)
	if err != nil {
		t.Fatalf("initialize sqlite: %v", err)
	}
	defer db.Close()

	if !migrated {
		t.Fatal("migrator callback was not executed")
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("db ping failed: %v", err)
	}
}

func TestInitializeSQLiteAppliesConnectionPragmasAcrossThePool(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pool-pragmas.db")
	db, err := InitializeSQLiteWithJournalMode(dbPath, "WAL", newTestKeyring(t), nil)
	if err != nil {
		t.Fatalf("initialize sqlite: %v", err)
	}
	defer db.Close()

	connections := make([]*sql.Conn, 0, 4)
	defer func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	}()
	for index := 0; index < 4; index++ {
		connection, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("acquire pooled connection %d: %v", index, err)
		}
		connections = append(connections, connection)
	}
	for index, connection := range connections {
		var busyTimeout, foreignKeys int
		if err := connection.QueryRowContext(t.Context(), "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("read busy_timeout from connection %d: %v", index, err)
		}
		if err := connection.QueryRowContext(t.Context(), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("read foreign_keys from connection %d: %v", index, err)
		}
		if busyTimeout != 5000 || foreignKeys != 1 {
			t.Fatalf("connection %d pragmas: busy_timeout=%d foreign_keys=%d", index, busyTimeout, foreignKeys)
		}
	}
}

func TestCleanupSQLiteSidecars(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	if err := os.WriteFile(walPath, []byte("wal"), 0600); err != nil {
		t.Fatalf("write wal: %v", err)
	}
	if err := os.WriteFile(shmPath, []byte("shm"), 0600); err != nil {
		t.Fatalf("write shm: %v", err)
	}

	if err := CleanupSQLiteSidecars(dbPath); err != nil {
		t.Fatalf("cleanup sidecars: %v", err)
	}

	if _, err := os.Stat(walPath); !os.IsNotExist(err) {
		t.Fatalf("wal file still exists")
	}
	if _, err := os.Stat(shmPath); !os.IsNotExist(err) {
		t.Fatalf("shm file still exists")
	}
}

func TestIsRecoverableSQLiteIO(t *testing.T) {
	if IsRecoverableSQLiteIO(nil) {
		t.Fatal("nil error returned true")
	}
	if IsRecoverableSQLiteIO(sql.ErrNoRows) {
		t.Fatal("normal sql errors returned true")
	}
	if !IsRecoverableSQLiteIO(fmt.Errorf("disk I/O error")) {
		t.Fatal("disk I/O error was not identified as recoverable")
	}
	if !IsRecoverableSQLiteIO(fmt.Errorf("sqlite error (4874)")) {
		t.Fatal("code 4874 error was not identified as recoverable")
	}
}

func TestVerifyStartupEnvelopesAndInvariants(t *testing.T) {
	ctx := context.Background()

	t.Run("schema version under 11 skips check", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "v10.db")
		db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer db.Close()
		if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER); INSERT INTO schema_migrations VALUES (10);`); err != nil {
			t.Fatalf("seed v10: %v", err)
		}

		if err := VerifyStartupEnvelopesAndInvariants(ctx, db, nil); err != nil {
			t.Fatalf("expected nil error for v10 schema, got %v", err)
		}
	})

	t.Run("missing keyring on v11 returns ErrMissingKeyring", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "v11_nokeyring.db")
		db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer db.Close()
		if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER); INSERT INTO schema_migrations VALUES (11);`); err != nil {
			t.Fatalf("seed v11: %v", err)
		}

		if err := VerifyStartupEnvelopesAndInvariants(ctx, db, nil); err != profilestorage.ErrMissingKeyring {
			t.Fatalf("expected ErrMissingKeyring, got %v", err)
		}
	})

	t.Run("missing child secret returns error", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "v11_missing_secret.db")
		kr := newTestKeyring(t)
		db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer db.Close()

		schema := `
			CREATE TABLE schema_migrations (version INTEGER);
			INSERT INTO schema_migrations VALUES (11);
			CREATE TABLE vless_keys (id INTEGER PRIMARY KEY, label TEXT);
			CREATE TABLE vless_key_secrets (vless_key_id INTEGER PRIMARY KEY, encrypted_url TEXT);
			INSERT INTO vless_keys (id, label) VALUES (1, 'orphaned parent');
		`
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("seed schema: %v", err)
		}

		err = VerifyStartupEnvelopesAndInvariants(ctx, db, kr)
		if err == nil || !strings.Contains(err.Error(), "parents without secrets") {
			t.Fatalf("expected missing secret error, got %v", err)
		}
	})

	t.Run("unknown encryption key ID returns error", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "v11_unknown_key.db")
		kr := newTestKeyring(t)
		db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer db.Close()

		data, err := profilestorage.GenerateKeyringJSON("other-key", "other-bik")
		if err != nil {
			t.Fatalf("generate other keyring: %v", err)
		}
		otherKr, err := profilestorage.LoadKeyringJSON(data)
		if err != nil {
			t.Fatalf("load other keyring: %v", err)
		}

		activeID, activeKey, _ := otherKr.GetActiveEncryptionKey()
		env, err := profilestorage.Encrypt([]byte("vless://test"), activeID, activeKey, 1)
		if err != nil {
			t.Fatalf("encrypt secret: %v", err)
		}

		schema := `
			CREATE TABLE schema_migrations (version INTEGER);
			INSERT INTO schema_migrations VALUES (11);
			CREATE TABLE vless_keys (id INTEGER PRIMARY KEY, label TEXT);
			CREATE TABLE vless_key_secrets (vless_key_id INTEGER PRIMARY KEY, encrypted_url TEXT);
		`
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("seed schema: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO vless_keys (id, label) VALUES (1, 'key1')`); err != nil {
			t.Fatalf("insert parent: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO vless_key_secrets (vless_key_id, encrypted_url) VALUES (1, ?)`, env); err != nil {
			t.Fatalf("insert secret: %v", err)
		}

		err = VerifyStartupEnvelopesAndInvariants(ctx, db, kr)
		if err == nil || !strings.Contains(err.Error(), "unknown key id") {
			t.Fatalf("expected unknown key id error, got %v", err)
		}
	})

	t.Run("valid envelope and invariant succeeds", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "v11_valid.db")
		kr := newTestKeyring(t)
		db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer db.Close()

		activeID, activeKey, _ := kr.GetActiveEncryptionKey()
		env, err := profilestorage.Encrypt([]byte("vless://test"), activeID, activeKey, 1)
		if err != nil {
			t.Fatalf("encrypt secret: %v", err)
		}

		schema := `
			CREATE TABLE schema_migrations (version INTEGER);
			INSERT INTO schema_migrations VALUES (11);
			CREATE TABLE vless_keys (id INTEGER PRIMARY KEY, label TEXT);
			CREATE TABLE vless_key_secrets (vless_key_id INTEGER PRIMARY KEY, encrypted_url TEXT);
		`
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("seed schema: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO vless_keys (id, label) VALUES (1, 'key1')`); err != nil {
			t.Fatalf("insert parent: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO vless_key_secrets (vless_key_id, encrypted_url) VALUES (1, ?)`, env); err != nil {
			t.Fatalf("insert secret: %v", err)
		}

		if err := VerifyStartupEnvelopesAndInvariants(ctx, db, kr); err != nil {
			t.Fatalf("unexpected verification error: %v", err)
		}
	})
}
