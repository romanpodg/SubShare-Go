package storage

import (
	"database/sql"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"path/filepath"
	"testing"
)

func testKeyring(t *testing.T) *profilestorage.Keyring {
	t.Helper()
	data, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	requireFixturef(t, err == nil, "generate keyring: %v", err)

	kr, err := profilestorage.LoadKeyringJSON(data)
	requireFixturef(t, err == nil, "parse keyring: %v", err)

	return kr
}

func openMigratedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := InitializeSQLiteWithJournalMode(filepath.Join(t.TempDir(), "migrated.db"), "DELETE", testKeyring(t))
	requireFixturef(t, err == nil, "open migrated db: %v", err)

	t.Cleanup(func() { _ = db.Close() })
	return db
}

func createPopulatedPreStage5Schema(t *testing.T, db *sql.DB, version int) {
	t.Helper()
	requireFixturef(t, version == 8 || version == 9, "unsupported fixture migration version %d", version)

	{
		_, err := db.Exec(`PRAGMA foreign_keys = ON`)
		requireFixturef(t, err == nil, "enable foreign keys: %v", err)
	}

	profileColumns := ""
	syncCountColumn := ""
	if version >= 9 {
		profileColumns = `,
			protocol TEXT NOT NULL DEFAULT 'legacy',
			profile_fingerprint TEXT,
			profile_schema_version INTEGER NOT NULL DEFAULT 0,
			profile_compatibility TEXT NOT NULL DEFAULT 'legacy',
			profile_warnings_json TEXT NOT NULL DEFAULT '[]'`
		syncCountColumn = `, result_counts_json TEXT NOT NULL DEFAULT '{}'`
	}
	statements := []string{
		`CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE subscription_settings(id INTEGER PRIMARY KEY CHECK (id = 1), happ_notify_expiration INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE external_subscription_sources(id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, key_category TEXT NOT NULL DEFAULT '', key_category_id INTEGER)`,
		`CREATE TABLE key_categories(id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, color TEXT NOT NULL DEFAULT '#d8b33d', sort_order INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE users(id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, token TEXT NOT NULL UNIQUE)`,
		`CREATE TABLE vless_keys(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			url TEXT NOT NULL UNIQUE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			check_status TEXT NOT NULL DEFAULT 'unknown',
			check_error TEXT,
			last_checked_at DATETIME,
			last_latency_ms INTEGER,
			status TEXT NOT NULL DEFAULT 'active',
			key_kind TEXT NOT NULL DEFAULT 'real',
			template_text TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0,
			starts_at DATETIME,
			expires_at DATETIME,
			blocked_reason TEXT,
			external_source_id INTEGER,
			external_key_ref TEXT,
			category TEXT NOT NULL DEFAULT '',
			category_id INTEGER REFERENCES key_categories(id) ON DELETE SET NULL,
			health_failure_count INTEGER NOT NULL DEFAULT 0` + profileColumns + `
		)`,
		`CREATE TABLE user_keys(user_id INTEGER NOT NULL, key_id INTEGER NOT NULL, PRIMARY KEY(user_id, key_id), FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE, FOREIGN KEY(key_id) REFERENCES vless_keys(id) ON DELETE CASCADE)`,
		`CREATE TABLE source_sync_runs(id INTEGER PRIMARY KEY AUTOINCREMENT, source_id INTEGER NOT NULL, status TEXT NOT NULL, imported_count INTEGER NOT NULL DEFAULT 0, skipped_count INTEGER NOT NULL DEFAULT 0, error_message TEXT NOT NULL DEFAULT '', started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, finished_at DATETIME` + syncCountColumn + `)`,
		`CREATE INDEX idx_vless_keys_kind_sort ON vless_keys(key_kind, sort_order, id)`,
		`CREATE INDEX idx_vless_keys_category_sort ON vless_keys(category, sort_order, id)`,
		`CREATE INDEX idx_vless_keys_external_source ON vless_keys(external_source_id)`,
		`CREATE UNIQUE INDEX idx_vless_keys_external_source_ref ON vless_keys(external_source_id, external_key_ref)`,
		`CREATE INDEX idx_vless_keys_category_id ON vless_keys(category_id, sort_order, id)`,
		`CREATE INDEX idx_vless_keys_delivery_health ON vless_keys(status, key_kind, health_failure_count, sort_order, id)`,
		`CREATE TRIGGER trg_vless_keys_category_insert AFTER INSERT ON vless_keys WHEN NEW.category_id IS NULL AND TRIM(COALESCE(NEW.category, '')) <> '' BEGIN UPDATE vless_keys SET category_id = (SELECT id FROM key_categories WHERE name = NEW.category) WHERE id = NEW.id; END`,
		`CREATE TRIGGER trg_vless_keys_category_id_insert AFTER INSERT ON vless_keys WHEN NEW.category_id IS NOT NULL BEGIN UPDATE vless_keys SET category = COALESCE((SELECT name FROM key_categories WHERE id = NEW.category_id), '') WHERE id = NEW.id; END`,
		`CREATE TRIGGER trg_vless_keys_category_id_update AFTER UPDATE OF category_id ON vless_keys BEGIN UPDATE vless_keys SET category = COALESCE((SELECT name FROM key_categories WHERE id = NEW.category_id), '') WHERE id = NEW.id; END`,
		`CREATE TRIGGER trg_key_categories_name_compat AFTER UPDATE OF name ON key_categories BEGIN UPDATE vless_keys SET category = NEW.name WHERE category_id = NEW.id; UPDATE external_subscription_sources SET key_category = NEW.name WHERE key_category_id = NEW.id; END`,
		`INSERT INTO external_subscription_sources(id, name) VALUES(10, 'Source A'), (20, 'Source B')`,
		`INSERT INTO key_categories(id, name, sort_order) VALUES(1, 'edge', 1)`,
		`INSERT INTO users(id, name, token) VALUES(501, 'assigned', 'assigned-token')`,
	}
	for _, statement := range statements {
		{
			_, err := db.Exec(statement)
			requireFixturef(t, err == nil, "prepare version-%d database: %v\nstatement: %s", version, err, statement)
		}

	}
	for migrationVersion := 1; migrationVersion <= version; migrationVersion++ {
		{
			_, err := db.Exec(`INSERT INTO schema_migrations(version, name) VALUES(?, 'old')`, migrationVersion)
			requireFixturef(t, err == nil, "record fixture migration %d: %v", migrationVersion, err)
		}

	}

	profileValues := ""
	if version >= 9 {
		profileValues = `, protocol, profile_fingerprint, profile_schema_version, profile_compatibility, profile_warnings_json`
	}
	insertColumns := `id, label, url, created_at, check_status, status, key_kind, sort_order, starts_at, external_source_id, external_key_ref, category, category_id, health_failure_count` + profileValues
	rows := []string{
		`101, 'vless-existing', 'vless://legacy-secret@vless.example:443', '2025-01-02T03:04:05Z', 'unknown', 'active', 'real', 1, '2025-01-02T03:04:05Z', 10, 'stable-vless-ref', 'edge', 1, 0`,
		`102, 'vmess-existing', 'vmess://legacy-payload', '2025-01-03T03:04:05Z', 'unknown', 'active', 'real', 2, '2025-01-03T03:04:05Z', 20, 'stable-vmess-ref', 'edge', 1, 0`,
		`103, 'trojan-existing', 'trojan://legacy-secret@trojan.example:443', '2025-01-04T03:04:05Z', 'unknown', 'active', 'real', 3, '2025-01-04T03:04:05Z', NULL, NULL, 'edge', 1, 0`,
	}
	if version >= 9 {
		rows[0] += `, 'vless', 'pf1_existing', 1, 'full', '["legacy_warning"]'`
		rows[1] += `, 'vmess', 'pf1_vmess', 1, 'full', '[]'`
		rows[2] += `, 'trojan', NULL, 0, 'legacy', '[]'`
	}
	for _, values := range rows {
		{
			_, err := db.Exec(`INSERT INTO vless_keys(` + insertColumns + `) VALUES(` + values + `)`)
			requireFixturef(t, err == nil, "insert fixture key: %v", err)
		}

	}
	if version >= 9 {
		{
			_, err := db.Exec(`CREATE INDEX idx_vless_keys_external_source_fingerprint ON vless_keys(external_source_id, profile_fingerprint)`)
			requireFixturef(t, err == nil, "create version-nine fingerprint index: %v", err)
		}

	}
	{
		_, err := db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(501, 101), (501, 102)`)
		requireFixturef(t, err == nil, "create fixture assignments: %v", err)
	}

}
