package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/platform/configuration"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func TestOpenDatabaseNormalRestartPreservesState(t *testing.T) {
	for _, mode := range []string{"WAL", "DELETE"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			keyringPath := filepath.Join(dir, "keyring.json")
			keyringBytes, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
			requireStartupSuccess(t, err)
			requireStartupSuccess(t, os.WriteFile(keyringPath, keyringBytes, 0o600))
			keyring, err := profilestorage.LoadKeyringJSON(keyringBytes)
			requireStartupSuccess(t, err)
			config := configuration.Config{
				DBPath: filepath.Join(dir, "state", "app.db"), SQLiteJournalMode: mode,
				ProfileKeyring: keyring, AdminUser: "unused-bootstrap-owner",
				// An existing administrator allows startup without a new password.
				AdminPassword: "",
			}
			db := openStartupDatabase(t, config)
			execStartupSQL(t, db, `INSERT INTO admins(username, password_hash, role) VALUES('existing-admin', 'existing-hash', 'operator');
				INSERT INTO users(name, token, subscription_id) VALUES('existing-user', 'fixture-token', 'fixture-subscription')`)
			profile := seedStartupEncryptedProfile(t, db, keyring)
			requireStartupSuccess(t, db.Close())
			for restart := 0; restart < 2; restart++ {
				reopened := openStartupDatabase(t, config)
				_, err := buildApp(config, reopened)
				requireStartupSuccess(t, err)
				assertStartupAccountsAndSchema(t, reopened, mode)
				assertStartupEncryptedProfile(t, reopened, keyring, profile)
				retainedKeyring, err := os.ReadFile(keyringPath)
				requireStartupSuccess(t, err)
				requireStartupEqual(t, "keyring file retained", bytes.Equal(retainedKeyring, keyringBytes), true)
				requireStartupSuccess(t, reopened.Close())
			}
		})
	}
}

func openStartupDatabase(t *testing.T, config configuration.Config) *sql.DB {
	t.Helper()
	db, err := openDatabase(config)
	requireStartupSuccess(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func assertStartupAccountsAndSchema(t *testing.T, db *sql.DB, mode string) {
	t.Helper()
	var actualMode, userName string
	var migrations, admins int
	var admin struct{ username, hash, role string }
	scanStartupSQL(t, db.QueryRow("PRAGMA journal_mode"), &actualMode)
	scanStartupSQL(t, db.QueryRow("SELECT COUNT(*) FROM schema_migrations"), &migrations)
	scanStartupSQL(t, db.QueryRow("SELECT COUNT(*) FROM admins"), &admins)
	scanStartupSQL(t, db.QueryRow("SELECT username, password_hash, role FROM admins"), &admin.username, &admin.hash, &admin.role)
	scanStartupSQL(t, db.QueryRow("SELECT name FROM users WHERE subscription_id = 'fixture-subscription'"), &userName)
	requireStartupEqual(t, "journal mode", actualMode, strings.ToLower(mode))
	requireStartupEqual(t, "migration count", migrations, len(storage.SchemaMigrations))
	requireStartupEqual(t, "administrator count", admins, 1)
	expectedAdmin := struct{ username, hash, role string }{"existing-admin", "existing-hash", "operator"}
	requireStartupEqual(t, "administrator retained", admin, expectedAdmin)
	requireStartupEqual(t, "user retained", userName, "existing-user")
}

type startupProfile struct {
	id       int64
	envelope string
}

func assertStartupEncryptedProfile(t *testing.T, db *sql.DB, keyring *profilestorage.Keyring, expected startupProfile) {
	t.Helper()
	var envelope string
	scanStartupSQL(t, db.QueryRow("SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?", expected.id), &envelope)
	requireStartupEqual(t, "encrypted profile retained", envelope, expected.envelope)
	secret, err := profilestorage.Decrypt(envelope, keyring, expected.id)
	requireStartupSuccess(t, err)
	requireStartupEqual(t, "decrypted profile retained", secret.Reveal(), "vless://fixture@example.com:443?type=tcp")
}

func seedStartupEncryptedProfile(t *testing.T, db *sql.DB, keyring *profilestorage.Keyring) startupProfile {
	t.Helper()
	const raw = "vless://fixture@example.com:443?type=tcp"
	keyName, key, err := keyring.GetActiveEncryptionKey()
	requireStartupSuccess(t, err)
	_, blindKey, err := keyring.GetActiveBlindIndexKey()
	requireStartupSuccess(t, err)
	result, err := db.Exec(`INSERT INTO vless_keys(label, url_blind_index, status, key_kind) VALUES('fixture-profile', ?, 'active', 'real')`, profilestorage.ComputeBlindIndex(blindKey, raw))
	requireStartupSuccess(t, err)
	id, err := result.LastInsertId()
	requireStartupSuccess(t, err)
	envelope, err := profilestorage.Encrypt([]byte(raw), keyName, key, id)
	requireStartupSuccess(t, err)
	execStartupSQL(t, db, "INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)", id, envelope)
	return startupProfile{id, envelope}
}

func TestOpenDatabaseDirectoryFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	requireStartupSuccess(t, os.WriteFile(blocker, []byte("keep"), 0o600))
	db, err := openDatabase(configuration.Config{DBPath: filepath.Join(blocker, "app.db")})
	requireStartupEqual(t, "failed startup database", db, (*sql.DB)(nil))
	requireStartupEqual(t, "directory error returned", err != nil, true)
	requireStartupEqual(t, "directory diagnostic", err.Error(), "create DB_PATH directory")
	data, err := os.ReadFile(blocker)
	requireStartupSuccess(t, err)
	requireStartupEqual(t, "obstructing file retained", string(data), "keep")
}

func TestOpenDatabaseMigrationFailureCanBeRetriedWithoutLosingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed-schema.db")
	db, err := sql.Open("sqlite", filepath.ToSlash(path))
	requireStartupSuccess(t, err)
	t.Cleanup(func() { _ = db.Close() })
	execStartupSQL(t, db, `CREATE TABLE schema_migrations (unexpected TEXT);
		CREATE TABLE retained_data (value TEXT);
		INSERT INTO retained_data VALUES ('keep after failed migration')`)
	requireStartupSuccess(t, db.Close())
	config := configuration.Config{DBPath: path, SQLiteJournalMode: "DELETE", ProfileKeyring: testKeyring(t)}
	failed, err := openDatabase(config)
	requireStartupEqual(t, "failed migration database", failed, (*sql.DB)(nil))
	requireStartupEqual(t, "migration error returned", err != nil, true)
	requireStartupEqual(t, "migration diagnostic", strings.Contains(err.Error(), "migrate db: check migration 0:"), true)
	// Repair only the disposable fixture, then exercise a real startup retry.
	repair, err := sql.Open("sqlite", filepath.ToSlash(path))
	requireStartupSuccess(t, err)
	t.Cleanup(func() { _ = repair.Close() })
	execStartupSQL(t, repair, "DROP TABLE schema_migrations")
	requireStartupSuccess(t, repair.Close())
	retried := openStartupDatabase(t, config)
	var value string
	scanStartupSQL(t, retried.QueryRow("SELECT value FROM retained_data"), &value)
	requireStartupEqual(t, "data retained after failed migration and retry", value, "keep after failed migration")
}

func execStartupSQL(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	_, err := db.Exec(query, args...)
	requireStartupSuccess(t, err)
}

func scanStartupSQL(t *testing.T, row *sql.Row, destinations ...any) {
	t.Helper()
	requireStartupSuccess(t, row.Scan(destinations...))
}

func requireStartupSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Report the invariant without emitting generated credential/keyring bytes.
func requireStartupEqual[T comparable](t *testing.T, invariant string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("startup invariant failed: %s", invariant)
	}
}
