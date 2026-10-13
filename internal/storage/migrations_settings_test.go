package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

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
			db := openMigratedTestDB(t)
			{
				_, err := db.Exec(`UPDATE subscription_settings SET extra_status = ? WHERE id = 1`, nullStringValue(test.canonical))
				requireFixture(t, err == nil, err)
			}

			{
				_, err := db.Exec(`UPDATE subscription_delivery_settings SET announcement = ? WHERE id = 1`, test.legacy)
				requireFixture(t, err == nil, err)
			}

			conn, err := db.Conn(context.Background())
			requireFixture(t, err == nil, err)

			defer conn.Close()
			{
				err := migrateCanonicalSubscriptionAnnouncement(context.Background(), conn, db, nil)
				requireFixturef(t, err == nil, "migrate announcement: %v", err)
			}

			var canonical, retired string
			{
				err := conn.QueryRowContext(context.Background(), `SELECT COALESCE(extra_status, '') FROM subscription_settings WHERE id = 1`).Scan(&canonical)
				requireFixture(t, err == nil, err)
			}

			{
				err := conn.QueryRowContext(context.Background(), `SELECT announcement FROM subscription_delivery_settings WHERE id = 1`).Scan(&retired)
				requireFixture(t, err == nil, err)
			}

			requireFixturef(t, canonical == test.want, "canonical=%q want=%q retired=%q", canonical, test.want, retired)
			requireFixturef(t, retired == "", "canonical=%q want=%q retired=%q", canonical, test.want, retired)

		})
	}
}

func TestMigrateHappRoutingDeliveryModePreservesConfigAndDefaultsDisabled(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "routing-mode.db"))
	requireFixture(t, err == nil, err)

	defer db.Close()
	legacyConfig := "  {\n  \"Name\": \"Существующий профиль\",\n  \"FutureField\": true\n}  "
	{
		_, err := db.Exec(`CREATE TABLE routing_settings (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		config_json TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	); INSERT INTO routing_settings(id, config_json) VALUES(1, ?)`, legacyConfig)
		requireFixture(t, err == nil, err)
	}

	conn, err := db.Conn(context.Background())
	requireFixture(t, err == nil, err)

	defer conn.Close()
	{
		err := migrateHappRoutingDeliveryMode(context.Background(), conn, db, nil)
		requireFixturef(t, err == nil, "migrate routing delivery mode: %v", err)
	}

	{
		err := migrateHappRoutingDeliveryMode(context.Background(), conn, db, nil)
		requireFixturef(t, err == nil, "repeat routing delivery mode migration: %v", err)
	}

	var config, mode string
	{
		err := conn.QueryRowContext(context.Background(), `SELECT config_json, delivery_mode FROM routing_settings WHERE id = 1`).Scan(&config, &mode)
		requireFixture(t, err == nil, err)
	}

	requireFixturef(t, config == legacyConfig, "config=%q mode=%q", config, mode)
	requireFixturef(t, mode == "disabled", "config=%q mode=%q", config, mode)

}

func TestMigrateBackgroundJobWarningResultsPreservesExistingJobs(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "background-jobs.db"))
	requireFixturef(t, err == nil, "open sqlite: %v", err)

	defer db.Close()
	{
		_, err := db.Exec(`CREATE TABLE background_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kind TEXT NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','failed')),
		target_type TEXT NOT NULL DEFAULT '', target_id TEXT NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '', run_after DATETIME, started_at DATETIME,
		finished_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
		requireFixturef(t, err == nil, "create legacy jobs table: %v", err)
	}

	{
		_, err := db.Exec(`CREATE INDEX idx_background_jobs_status ON background_jobs(status, run_after, id)`)
		requireFixturef(t, err == nil, "create legacy jobs index: %v", err)
	}

	{
		_, err := db.Exec(`INSERT INTO background_jobs(id, kind, status, error_message) VALUES(7, 'keys_health_check', 'failed', 'legacy error')`)
		requireFixturef(t, err == nil, "insert legacy job: %v", err)
	}

	conn, err := db.Conn(context.Background())
	requireFixturef(t, err == nil, "open migration connection: %v", err)

	defer conn.Close()
	{
		err := migrateBackgroundJobWarningResults(context.Background(), conn, db, nil)
		requireFixturef(t, err == nil, "migrate warning results: %v", err)
	}

	{
		err := migrateBackgroundJobWarningResults(context.Background(), conn, db, nil)
		requireFixturef(t, err == nil, "repeat warning results migration: %v", err)
	}

	var id int64
	var status, message, counts string
	{
		err := db.QueryRow(`SELECT id, status, error_message, result_counts_json FROM background_jobs WHERE id = 7`).Scan(&id, &status, &message, &counts)
		requireFixturef(t, err == nil, "read preserved job: %v", err)
	}

	requireFixturef(t, id == 7, "legacy job was not preserved: id=%d status=%q message=%q counts=%q", id, status, message, counts)
	requireFixturef(t, status == "failed", "legacy job was not preserved: id=%d status=%q message=%q counts=%q", id, status, message, counts)
	requireFixturef(t, message == "legacy error", "legacy job was not preserved: id=%d status=%q message=%q counts=%q", id, status, message, counts)
	requireFixturef(t, counts == "{}", "legacy job was not preserved: id=%d status=%q message=%q counts=%q", id, status, message, counts)

	{
		_, err := db.Exec(`INSERT INTO background_jobs(kind, status, result_counts_json) VALUES('keys_health_check', 'succeeded_with_warnings', '{"persist_failed":1}')`)
		requireFixturef(t, err == nil, "warning status is not accepted after migration: %v", err)
	}

}

func TestLegacyBootstrapPreservesUnlimitedMaxDevices(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy-unlimited-devices.db"))
	requireFixturef(t, err == nil, "open sqlite: %v", err)

	defer db.Close()
	{
		_, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			token TEXT NOT NULL UNIQUE,
			max_devices INTEGER
		);
		INSERT INTO users(name, token, max_devices) VALUES('Unlimited', 'unlimited-token', 0);
	`)
		requireFixturef(t, err == nil, "prepare legacy database: %v", err)
	}

	{
		err := MigrateWithKeyring(db, testKeyring(t))
		requireFixturef(t, err == nil, "migrate legacy database: %v", err)
	}

	var maxDevices int
	{
		err := db.QueryRow(`SELECT max_devices FROM users WHERE token = 'unlimited-token'`).Scan(&maxDevices)
		requireFixturef(t, err == nil, "read migrated device limit: %v", err)
	}

	requireFixturef(t, maxDevices == 0, "legacy unlimited device limit changed to %d", maxDevices)

}

func TestMigrationAddsGeneralExpirationSettingWithoutCopyingHappValue(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "expiration-setting-upgrade.db"))
	requireFixturef(t, err == nil, "open sqlite: %v", err)

	defer db.Close()
	{
		_, err := db.Exec(`
		CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
		CREATE TABLE subscription_settings(
			id INTEGER PRIMARY KEY CHECK (id = 1),
			happ_notify_expiration INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE vless_keys(id INTEGER PRIMARY KEY, label TEXT NOT NULL);
		INSERT INTO subscription_settings(id, happ_notify_expiration) VALUES(1, 1);
	`)
		requireFixturef(t, err == nil, "prepare version-14 settings database: %v", err)
	}

	for version := 1; version <= 14; version++ {
		{
			_, err := db.Exec(`INSERT INTO schema_migrations(version, name) VALUES(?, 'old')`, version)
			requireFixturef(t, err == nil, "record migration %d: %v", version, err)
		}

	}
	{
		err := MigrateWithKeyring(db, nil)
		requireFixturef(t, err == nil, "upgrade settings database: %v", err)
	}

	{
		err := MigrateWithKeyring(db, nil)
		requireFixturef(t, err == nil, "repeat settings upgrade: %v", err)
	}

	var happValue, showExpiration int
	{
		err := db.QueryRow(`SELECT happ_notify_expiration, show_subscription_expiration FROM subscription_settings WHERE id = 1`).Scan(&happValue, &showExpiration)
		requireFixturef(t, err == nil, "read upgraded settings: %v", err)
	}

	requireFixturef(t, happValue == 1, "upgraded values happ=%d show_expiration=%d", happValue, showExpiration)
	requireFixturef(t, showExpiration == 0, "upgraded values happ=%d show_expiration=%d", happValue, showExpiration)

}

func TestMigrationAddsNullableClientDisplayNameIdempotently(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "client-display-name-upgrade.db"))
	requireFixture(t, err == nil, err)

	defer db.Close()
	{
		_, err := db.Exec(`CREATE TABLE vless_keys(id INTEGER PRIMARY KEY, label TEXT NOT NULL); INSERT INTO vless_keys(id, label) VALUES(1, 'Panel');`)
		requireFixture(t, err == nil, err)
	}

	{
		err := migrateClientDisplayName(context.Background(), nil, db, nil)
		requireFixture(t, err == nil, err)
	}

	{
		err := migrateClientDisplayName(context.Background(), nil, db, nil)
		requireFixture(t, err == nil, err)
	}

	var value sql.NullString
	{
		err := db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = 1`).Scan(&value)
		requireFixture(t, err == nil, err)
	}

	requireFixturef(t, !(value.Valid), "legacy row was rewritten: %#v", value)

}
