package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func testKeyring(t *testing.T) *profilestorage.Keyring {
	t.Helper()
	data, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	kr, err := profilestorage.LoadKeyringJSON(data)
	if err != nil {
		t.Fatalf("parse keyring: %v", err)
	}
	return kr
}

func TestMigrateAppliesVersionedMigrationsIdempotently(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	kr := testKeyring(t)
	if err := migrateWithKeyring(db, kr); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if _, err := db.Exec(`UPDATE subscription_settings SET refresh_hours = 0 WHERE id = 1`); err != nil {
		t.Fatalf("prepare no-startup-data-fix assertion: %v", err)
	}
	if err := migrateWithKeyring(db, kr); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var refreshHours int
	if err := db.QueryRow(`SELECT refresh_hours FROM subscription_settings WHERE id = 1`).Scan(&refreshHours); err != nil {
		t.Fatalf("read subscription settings: %v", err)
	}
	if refreshHours != 0 {
		t.Fatalf("startup modified data outside a versioned migration: refresh_hours=%d", refreshHours)
	}

	var versions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatalf("count schema migrations: %v", err)
	}
	if versions != len(schemaMigrations) {
		t.Fatalf("schema migration count = %d, want %d", versions, len(schemaMigrations))
	}
	assertSourceOwnedSchemaMetadata(t, db)

	for _, table := range []string{"audit_events", "subscription_templates", "response_rules", "background_jobs", "api_tokens"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
			t.Fatalf("lookup table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s was not created", table)
		}
	}

	var fallbackRules int
	if err := db.QueryRow(`SELECT COUNT(*) FROM response_rules WHERE name = 'Fallback'`).Scan(&fallbackRules); err != nil {
		t.Fatalf("count fallback rules: %v", err)
	}
	if fallbackRules != 1 {
		t.Fatalf("fallback rule count = %d, want 1", fallbackRules)
	}

	if _, err := db.Exec(`INSERT INTO key_categories(name) VALUES('edge')`); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	activeID, activeKey, _ := kr.GetActiveEncryptionKey()
	_, bikKey, _ := kr.GetActiveBlindIndexKey()
	insRes, err := db.Exec(`INSERT INTO vless_keys(label, url_blind_index, category) VALUES('edge-key', ?, 'edge')`, profilestorage.ComputeBlindIndex(bikKey, "vless://migration-test"))
	if err != nil {
		t.Fatalf("insert categorized key: %v", err)
	}
	edgeID, _ := insRes.LastInsertId()
	secEnv, _ := profilestorage.Encrypt([]byte("vless://migration-test"), activeID, activeKey, edgeID)
	if _, err := db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, edgeID, secEnv); err != nil {
		t.Fatalf("insert categorized key secret: %v", err)
	}
	var categoryID sql.NullInt64
	if err := db.QueryRow(`SELECT category_id FROM vless_keys WHERE label = 'edge-key'`).Scan(&categoryID); err != nil {
		t.Fatalf("read normalized category: %v", err)
	}
	if !categoryID.Valid || categoryID.Int64 == 0 {
		t.Fatal("category trigger did not persist the foreign key")
	}
	var status, protocol, compatibility, warnings string
	var schemaVersion, healthFailures int
	var createdAt time.Time
	if err := db.QueryRow(`SELECT status, protocol, profile_schema_version, profile_compatibility, profile_warnings_json, health_failure_count, created_at FROM vless_keys WHERE label = 'edge-key'`).Scan(
		&status, &protocol, &schemaVersion, &compatibility, &warnings, &healthFailures, &createdAt,
	); err != nil {
		t.Fatalf("read rebuilt defaults: %v", err)
	}
	if status != "active" || protocol != "legacy" || schemaVersion != 0 || compatibility != "legacy" || warnings != "[]" || healthFailures != 0 || createdAt.IsZero() {
		t.Fatalf("rebuilt defaults changed: status=%q protocol=%q version=%d compatibility=%q warnings=%q failures=%d created=%v", status, protocol, schemaVersion, compatibility, warnings, healthFailures, createdAt)
	}
}

func TestMigrateCanonicalSubscriptionAnnouncementPreservesLegacyData(t *testing.T) {
	tests := []struct {
		name      string
		canonical string
		legacy    string
		want      string
	}{
		{name: "promotes legacy when canonical is empty", legacy: "Legacy announcement", want: "Legacy announcement"},
		{name: "canonical wins when both exist", canonical: "Canonical announcement", legacy: "Legacy announcement", want: "Canonical announcement"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			if _, err := app.db.Exec(`UPDATE subscription_settings SET extra_status = ? WHERE id = 1`, nullStringValue(test.canonical)); err != nil {
				t.Fatal(err)
			}
			if _, err := app.db.Exec(`UPDATE subscription_delivery_settings SET announcement = ? WHERE id = 1`, test.legacy); err != nil {
				t.Fatal(err)
			}
			conn, err := app.db.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := migrateCanonicalSubscriptionAnnouncement(context.Background(), conn, app.db, nil); err != nil {
				t.Fatalf("migrate announcement: %v", err)
			}

			var canonical, retired string
			if err := conn.QueryRowContext(context.Background(), `SELECT COALESCE(extra_status, '') FROM subscription_settings WHERE id = 1`).Scan(&canonical); err != nil {
				t.Fatal(err)
			}
			if err := conn.QueryRowContext(context.Background(), `SELECT announcement FROM subscription_delivery_settings WHERE id = 1`).Scan(&retired); err != nil {
				t.Fatal(err)
			}
			if canonical != test.want || retired != "" {
				t.Fatalf("canonical=%q want=%q retired=%q", canonical, test.want, retired)
			}
		})
	}
}

func TestMigrateHappRoutingDeliveryModePreservesConfigAndDefaultsDisabled(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "routing-mode.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacyConfig := "  {\n  \"Name\": \"Существующий профиль\",\n  \"FutureField\": true\n}  "
	if _, err := db.Exec(`CREATE TABLE routing_settings (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		config_json TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	); INSERT INTO routing_settings(id, config_json) VALUES(1, ?)`, legacyConfig); err != nil {
		t.Fatal(err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := migrateHappRoutingDeliveryMode(context.Background(), conn, db, nil); err != nil {
		t.Fatalf("migrate routing delivery mode: %v", err)
	}
	if err := migrateHappRoutingDeliveryMode(context.Background(), conn, db, nil); err != nil {
		t.Fatalf("repeat routing delivery mode migration: %v", err)
	}
	var config, mode string
	if err := conn.QueryRowContext(context.Background(), `SELECT config_json, delivery_mode FROM routing_settings WHERE id = 1`).Scan(&config, &mode); err != nil {
		t.Fatal(err)
	}
	if config != legacyConfig || mode != routingDeliveryModeDisabled {
		t.Fatalf("config=%q mode=%q", config, mode)
	}
}

func TestMigrateBackgroundJobWarningResultsPreservesExistingJobs(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "background-jobs.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE background_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kind TEXT NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','failed')),
		target_type TEXT NOT NULL DEFAULT '', target_id TEXT NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '', run_after DATETIME, started_at DATETIME,
		finished_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create legacy jobs table: %v", err)
	}
	if _, err := db.Exec(`CREATE INDEX idx_background_jobs_status ON background_jobs(status, run_after, id)`); err != nil {
		t.Fatalf("create legacy jobs index: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO background_jobs(id, kind, status, error_message) VALUES(7, 'keys_health_check', 'failed', 'legacy error')`); err != nil {
		t.Fatalf("insert legacy job: %v", err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("open migration connection: %v", err)
	}
	defer conn.Close()
	if err := migrateBackgroundJobWarningResults(context.Background(), conn, db, nil); err != nil {
		t.Fatalf("migrate warning results: %v", err)
	}
	if err := migrateBackgroundJobWarningResults(context.Background(), conn, db, nil); err != nil {
		t.Fatalf("repeat warning results migration: %v", err)
	}

	var id int64
	var status, message, counts string
	if err := db.QueryRow(`SELECT id, status, error_message, result_counts_json FROM background_jobs WHERE id = 7`).Scan(&id, &status, &message, &counts); err != nil {
		t.Fatalf("read preserved job: %v", err)
	}
	if id != 7 || status != "failed" || message != "legacy error" || counts != "{}" {
		t.Fatalf("legacy job was not preserved: id=%d status=%q message=%q counts=%q", id, status, message, counts)
	}
	if _, err := db.Exec(`INSERT INTO background_jobs(kind, status, result_counts_json) VALUES('keys_health_check', 'succeeded_with_warnings', '{"persist_failed":1}')`); err != nil {
		t.Fatalf("warning status is not accepted after migration: %v", err)
	}
}

func TestLegacyBootstrapPreservesUnlimitedMaxDevices(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy-unlimited-devices.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			max_devices INTEGER
		);
		INSERT INTO users(name, token, max_devices) VALUES('Unlimited', 'unlimited-token', 0);
	`); err != nil {
		t.Fatalf("prepare legacy database: %v", err)
	}

	if err := migrateWithKeyring(db, testKeyring(t)); err != nil {
		t.Fatalf("migrate legacy database: %v", err)
	}
	var maxDevices int
	if err := db.QueryRow(`SELECT max_devices FROM users WHERE token = 'unlimited-token'`).Scan(&maxDevices); err != nil {
		t.Fatalf("read migrated device limit: %v", err)
	}
	if maxDevices != 0 {
		t.Fatalf("legacy unlimited device limit changed to %d", maxDevices)
	}
}

func TestMigrationAddsGeneralExpirationSettingWithoutCopyingHappValue(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "expiration-setting-upgrade.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
		CREATE TABLE subscription_settings(
			id INTEGER PRIMARY KEY CHECK (id = 1),
			happ_notify_expiration INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE vless_keys(id INTEGER PRIMARY KEY, label TEXT NOT NULL);
		INSERT INTO subscription_settings(id, happ_notify_expiration) VALUES(1, 1);
	`); err != nil {
		t.Fatalf("prepare version-14 settings database: %v", err)
	}
	for version := 1; version <= 14; version++ {
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, name) VALUES(?, 'old')`, version); err != nil {
			t.Fatalf("record migration %d: %v", version, err)
		}
	}
	if err := migrateWithKeyring(db, nil); err != nil {
		t.Fatalf("upgrade settings database: %v", err)
	}
	if err := migrateWithKeyring(db, nil); err != nil {
		t.Fatalf("repeat settings upgrade: %v", err)
	}
	var happValue, showExpiration int
	if err := db.QueryRow(`SELECT happ_notify_expiration, show_subscription_expiration FROM subscription_settings WHERE id = 1`).Scan(&happValue, &showExpiration); err != nil {
		t.Fatalf("read upgraded settings: %v", err)
	}
	if happValue != 1 || showExpiration != 0 {
		t.Fatalf("upgraded values happ=%d show_expiration=%d", happValue, showExpiration)
	}
}

func TestMigrationAddsNullableClientDisplayNameIdempotently(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "client-display-name-upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE vless_keys(id INTEGER PRIMARY KEY, label TEXT NOT NULL); INSERT INTO vless_keys(id, label) VALUES(1, 'Panel');`); err != nil {
		t.Fatal(err)
	}
	if err := migrateClientDisplayName(context.Background(), nil, db, nil); err != nil {
		t.Fatal(err)
	}
	if err := migrateClientDisplayName(context.Background(), nil, db, nil); err != nil {
		t.Fatal(err)
	}
	var value sql.NullString
	if err := db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = 1`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value.Valid {
		t.Fatalf("legacy row was rewritten: %#v", value)
	}
}

func TestProfilePersistenceMigrationUpgradesPopulatedVersionEightDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "pre-profile.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	createPopulatedPreStage5Schema(t, db, 8)
	kr := testKeyring(t)
	if err := runVersionedMigrationsWithKeyring(db, kr); err != nil {
		t.Fatalf("upgrade version-eight database: %v", err)
	}
	var label, encURL, ref, protocol, compatibility, warnings, createdAt string
	var schemaVersion int
	if err := db.QueryRow(`SELECT k.label, s.encrypted_url, k.external_key_ref, k.protocol, k.profile_schema_version, k.profile_compatibility, k.profile_warnings_json, k.created_at FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = 101`).Scan(
		&label, &encURL, &ref, &protocol, &schemaVersion, &compatibility, &warnings, &createdAt,
	); err != nil {
		t.Fatalf("read upgraded row: %v", err)
	}
	sec, err := profilestorage.Decrypt(encURL, kr, 101)
	if err != nil {
		t.Fatalf("decrypt upgraded row secret: %v", err)
	}
	raw := sec.Reveal()
	if label != "vless-existing" || raw != "vless://legacy-secret@vless.example:443" || ref != "stable-vless-ref" || protocol != "legacy" || schemaVersion != 0 || compatibility != "legacy" || warnings != "[]" || !strings.HasPrefix(createdAt, "2025-01-02") {
		t.Fatalf("existing row changed during upgrade: label=%q raw=%q ref=%q protocol=%q version=%d compatibility=%q warnings=%q created=%q", label, raw, ref, protocol, schemaVersion, compatibility, warnings, createdAt)
	}
	var preservedRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id IN (101,102,103) AND label IN ('vless-existing','vmess-existing','trojan-existing')`).Scan(&preservedRows); err != nil || preservedRows != 3 {
		t.Fatalf("existing protocol rows merged or removed: count=%d err=%v", preservedRows, err)
	}
	assertSourceOwnedSchemaMetadata(t, db)

	var assignmentCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = 501 AND key_id IN (101, 102)`).Scan(&assignmentCount); err != nil || assignmentCount != 2 {
		t.Fatalf("assignments did not survive migration: count=%d err=%v", assignmentCount, err)
	}
	activeID, activeKey, _ := kr.GetActiveEncryptionKey()
	_, bikKey, _ := kr.GetActiveBlindIndexKey()
	inserted, err := db.Exec(`INSERT INTO vless_keys(label, url_blind_index, external_source_id, external_key_ref, protocol, profile_fingerprint, profile_schema_version, profile_compatibility)
		VALUES('same raw, second source', ?, 20, 'stable-vless-ref', 'vless', 'pf1_same', 1, 'full')`, profilestorage.ComputeBlindIndex(bikKey, "vless://legacy-secret@vless.example:443"))
	if err != nil {
		t.Fatalf("insert identical raw URI for second source after migration: %v", err)
	}
	insertedID, err := inserted.LastInsertId()
	if err != nil || insertedID <= 103 {
		t.Fatalf("AUTOINCREMENT sequence was not preserved: id=%d err=%v", insertedID, err)
	}
	secEnv, _ := profilestorage.Encrypt([]byte("vless://legacy-secret@vless.example:443"), activeID, activeKey, insertedID)
	_, _ = db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, insertedID, secEnv)

	if _, err := db.Exec(`INSERT INTO vless_keys(label, url_blind_index, external_source_id, external_key_ref, protocol, profile_fingerprint)
		VALUES('same semantic, same source', ?, 20, 'different-ref', 'vless', 'pf1_same')`, profilestorage.ComputeBlindIndex(bikKey, "vless://byte-different@example.com:443")); err == nil {
		t.Fatal("source-scoped semantic fingerprint uniqueness was not enforced")
	}
	if _, err := db.Exec(`INSERT INTO vless_keys(label, url_blind_index) VALUES('duplicate local', ?)`, profilestorage.ComputeBlindIndex(bikKey, "trojan://legacy-secret@trojan.example:443")); err == nil {
		t.Fatal("local-key URL uniqueness was not preserved")
	}
}

func TestSourceOwnedURLMigrationRollsBackWithoutMergingRows(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	createPopulatedPreStage5Schema(t, db, 9)
	if _, err := db.Exec(`INSERT INTO vless_keys(
		id, label, url, created_at, check_status, status, key_kind, sort_order, starts_at,
		external_source_id, external_key_ref, category, category_id, health_failure_count,
		protocol, profile_fingerprint, profile_schema_version, profile_compatibility, profile_warnings_json
	) VALUES(104, 'conflicting semantic row', 'hy2://different-auth@example.com', '2025-01-05T00:00:00Z',
		'unknown', 'active', 'real', 4, '2025-01-05T00:00:00Z', 10, 'different-ref', 'edge', 1, 0,
		'hysteria2', 'pf1_existing', 1, 'full', '[]')`); err != nil {
		t.Fatalf("insert pre-migration fingerprint conflict: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(501, 104)`); err != nil {
		t.Fatalf("assign conflicting row: %v", err)
	}

	kr := testKeyring(t)
	if err := runVersionedMigrationsWithKeyring(db, kr); err == nil || !strings.Contains(err.Error(), "source_owned_profile_urls") {
		t.Fatalf("expected safe unique-index migration failure, got %v", err)
	}
	var rowCount, assignments, applied int
	_ = db.QueryRow(`SELECT COUNT(*) FROM vless_keys`).Scan(&rowCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = 501`).Scan(&assignments)
	_ = db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 10`).Scan(&applied)
	if rowCount != 4 || assignments != 3 || applied != 0 {
		t.Fatalf("failed migration mutated data: rows=%d assignments=%d applied=%d", rowCount, assignments, applied)
	}
	var tableSQL string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'vless_keys'`).Scan(&tableSQL); err != nil || !strings.Contains(strings.ToUpper(tableSQL), "URL TEXT NOT NULL UNIQUE") {
		t.Fatalf("rollback did not restore old table: sql=%q err=%v", tableSQL, err)
	}
	var foreignKeysEnabled int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeysEnabled); err != nil || foreignKeysEnabled != 1 {
		t.Fatalf("failed migration did not restore foreign keys: enabled=%d err=%v", foreignKeysEnabled, err)
	}
}

func createPopulatedPreStage5Schema(t *testing.T, db *sql.DB, version int) {
	t.Helper()
	if version != 8 && version != 9 {
		t.Fatalf("unsupported fixture migration version %d", version)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
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
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("prepare version-%d database: %v\nstatement: %s", version, err, statement)
		}
	}
	for migrationVersion := 1; migrationVersion <= version; migrationVersion++ {
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, name) VALUES(?, 'old')`, migrationVersion); err != nil {
			t.Fatalf("record fixture migration %d: %v", migrationVersion, err)
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
		if _, err := db.Exec(`INSERT INTO vless_keys(` + insertColumns + `) VALUES(` + values + `)`); err != nil {
			t.Fatalf("insert fixture key: %v", err)
		}
	}
	if version >= 9 {
		if _, err := db.Exec(`CREATE INDEX idx_vless_keys_external_source_fingerprint ON vless_keys(external_source_id, profile_fingerprint)`); err != nil {
			t.Fatalf("create version-nine fingerprint index: %v", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(501, 101), (501, 102)`); err != nil {
		t.Fatalf("create fixture assignments: %v", err)
	}
}

func assertSourceOwnedSchemaMetadata(t *testing.T, db *sql.DB) {
	t.Helper()
	var tableSQL string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'vless_keys'`).Scan(&tableSQL); err != nil {
		t.Fatalf("read vless_keys SQL: %v", err)
	}
	if strings.Contains(strings.ToUpper(tableSQL), "URL TEXT NOT NULL UNIQUE") {
		t.Fatalf("global URL uniqueness survived migration: %s", tableSQL)
	}
	indexes := map[string]struct {
		unique  int
		partial int
	}{}
	rows, err := db.Query(`PRAGMA index_list(vless_keys)`)
	if err != nil {
		t.Fatalf("list vless indexes: %v", err)
	}
	for rows.Next() {
		var sequence, unique, partial int
		var name, origin string
		if err := rows.Scan(&sequence, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan vless index: %v", err)
		}
		indexes[name] = struct {
			unique  int
			partial int
		}{unique: unique, partial: partial}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		t.Fatalf("iterate vless indexes: %v", err)
	}
	_ = rows.Close()
	for name, wanted := range map[string]struct {
		unique  int
		partial int
	}{
		"idx_vless_keys_external_source_ref":         {1, 0},
		"idx_vless_keys_external_source_fingerprint": {1, 1},
		"idx_vless_keys_local_blind_index":           {1, 1},
		"idx_vless_keys_delivery_health":             {0, 0},
		"idx_vless_keys_category_id":                 {0, 0},
	} {
		if got, ok := indexes[name]; !ok || got != wanted {
			t.Fatalf("index %s metadata=%#v present=%v, want %#v", name, got, ok, wanted)
		}
	}
	var triggers int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name IN ('trg_vless_keys_category_insert','trg_vless_keys_category_id_insert','trg_vless_keys_category_id_update','trg_key_categories_name_compat')`).Scan(&triggers); err != nil || triggers != 4 {
		t.Fatalf("vless trigger count=%d err=%v", triggers, err)
	}
	var violations int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations); err != nil || violations != 0 {
		t.Fatalf("foreign key violations=%d err=%v", violations, err)
	}
	var foreignKeysEnabled int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeysEnabled); err != nil || foreignKeysEnabled != 1 {
		t.Fatalf("foreign keys were not restored: enabled=%d err=%v", foreignKeysEnabled, err)
	}
	var keyForeignKey int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_list('user_keys') WHERE "table" = 'vless_keys'`).Scan(&keyForeignKey); err != nil || keyForeignKey != 1 {
		t.Fatalf("user_keys parent metadata=%d err=%v", keyForeignKey, err)
	}
}
