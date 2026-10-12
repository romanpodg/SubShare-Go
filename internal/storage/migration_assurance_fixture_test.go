package storage

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

type migrationAssuranceFixture struct {
	db          *sql.DB
	path        string
	keyring     *profilestorage.Keyring
	keyringJSON []byte
}

func newMigrationAssuranceFixture(t *testing.T, version, keys int) migrationAssuranceFixture {
	t.Helper()
	data, err := profilestorage.GenerateKeyringJSON("upgrade-key", "upgrade-index")
	profileContractSuccess(t, err)
	keyring, err := profilestorage.LoadKeyringJSON(data)
	profileContractSuccess(t, err)
	path := filepath.Join(t.TempDir(), "historical.db")
	db, err := sql.Open("sqlite", sqliteDataSourceName(path))
	profileContractSuccess(t, err)
	t.Cleanup(func() { _ = db.Close() })
	profileContractSuccess(t, ConfigureSQLitePragmas(db, "DELETE"))
	f := migrationAssuranceFixture{db, path, keyring, data}
	f.applyThrough(t, version)
	f.seed(t, keys)
	return f
}

func (f migrationAssuranceFixture) applyThrough(t *testing.T, version int) {
	t.Helper()
	profileContractExec(t, f.db, `CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`)
	conn, err := f.db.Conn(context.Background())
	profileContractSuccess(t, err)
	defer conn.Close()
	for _, migration := range SchemaMigrations {
		if migration.Version > version {
			break
		}
		applyAssuranceMigration(t, f, conn, migration)
	}
}

func applyAssuranceMigration(t *testing.T, f migrationAssuranceFixture, conn *sql.Conn, migration schemaMigration) {
	t.Helper()
	ctx := context.Background()
	if migration.runGo == nil {
		profileContractSuccess(t, applySchemaMigration(ctx, conn, migration))
		return
	}
	profileContractSuccess(t, migration.runGo(ctx, conn, f.db, f.keyring))
	profileContractExec(t, f.db, `INSERT INTO schema_migrations(version, name) VALUES(?,?)`, migration.Version, migration.Name)
}

func (f migrationAssuranceFixture) seed(t *testing.T, keys int) {
	t.Helper()
	profileContractExec(t, f.db, `INSERT INTO users(id, name, token, subscription_id, activation_code, status, key_assignment_mode, starts_at, expires_at, max_devices) VALUES(501, 'Upgrade fixture', 'upgrade-fixture-token', 'upgrade-fixture-token', 'upgrade-fixture-code', 'active', 'all', '2025-01-01T00:00:00Z', '2099-01-01T00:00:00Z', 0)`)
	for index := 1; index <= keys; index++ {
		profileContractExec(t, f.db, `INSERT INTO vless_keys(id, label, url, status, key_kind, created_at, sort_order) VALUES(?, ?, ?, 'active', 'real', '2025-01-01T00:00:00Z', ?)`, index, fmt.Sprintf("Fixture %03d", index), migrationAssuranceURI(index), index)
		profileContractExec(t, f.db, `INSERT INTO user_keys(user_id, key_id) VALUES(501, ?)`, index)
	}
}

func migrationAssuranceURI(index int) string {
	return fmt.Sprintf("trojan://upgrade-fixture-%03d@upgrade.example:443#Fixture%%20%03d", index, index)
}

func (f migrationAssuranceFixture) snapshot(t *testing.T) map[string][][]any {
	t.Helper()
	result := make(map[string][][]any)
	for _, table := range []string{"users", "vless_keys", "vless_key_secrets", "user_keys", "sqlite_sequence", "schema_migrations"} {
		rows, err := f.db.Query("SELECT * FROM " + table + " ORDER BY rowid")
		profileContractSuccess(t, err)
		result[table] = profileContractRows(t, rows)
	}
	return result
}

func (f migrationAssuranceFixture) envelopes(t *testing.T) map[int64]string {
	t.Helper()
	rows, err := f.db.Query(`SELECT vless_key_id, encrypted_url FROM vless_key_secrets ORDER BY vless_key_id`)
	profileContractSuccess(t, err)
	defer rows.Close()
	result := make(map[int64]string)
	for rows.Next() {
		var id int64
		var envelope string
		profileContractSuccess(t, rows.Scan(&id, &envelope))
		result[id] = envelope
	}
	profileContractSuccess(t, rows.Err())
	return result
}

func (f migrationAssuranceFixture) assertUpgraded(t *testing.T, keys int) {
	t.Helper()
	profileContractEqual(t, "historical keys retained", profileContractCount(t, f.db, `SELECT COUNT(*) FROM vless_keys WHERE id BETWEEN 1 AND ? AND created_at='2025-01-01T00:00:00Z'`, keys), keys)
	profileContractEqual(t, "assignments retained", profileContractCount(t, f.db, `SELECT COUNT(*) FROM user_keys WHERE user_id=501`), keys)
	profileContractEqual(t, "foreign keys enabled", profileContractCount(t, f.db, `PRAGMA foreign_keys`), 1)
	profileContractEqual(t, "no foreign key violation", profileContractCount(t, f.db, `SELECT COUNT(*) FROM pragma_foreign_key_check`), 0)
	profileContractEqual(t, "plaintext column removed", profileContractCount(t, f.db, `SELECT COUNT(*) FROM pragma_table_info('vless_keys') WHERE name='url'`), 0)
	profileContractEqual(t, "sequence preserved", profileContractCount(t, f.db, `SELECT seq FROM sqlite_sequence WHERE name='vless_keys'`), keys)
	assertAssuranceMigrationIdentities(t, f.db)
	for id, envelope := range f.envelopes(t) {
		secret, err := profilestorage.Decrypt(envelope, f.keyring, id)
		profileContractSuccess(t, err)
		profileContractEqual(t, "authenticated raw preserved", secret.Reveal(), migrationAssuranceURI(int(id)))
	}
	profileContractEqual(t, "complete secret set", len(f.envelopes(t)), keys)
}

func assertAssuranceMigrationIdentities(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, migration := range SchemaMigrations {
		var name string
		profileContractSuccess(t, db.QueryRow(`SELECT name FROM schema_migrations WHERE version=?`, migration.Version).Scan(&name))
		profileContractEqual(t, "historical migration identity", name, migration.Name)
	}
	profileContractEqual(t, "migration set", profileContractCount(t, db, `SELECT COUNT(*) FROM schema_migrations`), len(SchemaMigrations))
}
