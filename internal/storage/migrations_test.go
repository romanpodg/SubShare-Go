package storage

import (
	"database/sql"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrateAppliesVersionedMigrationsIdempotently(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migration.db"))
	requireFixturef(t, err == nil, "open sqlite: %v", err)

	defer db.Close()
	{
		_, err := db.Exec(`PRAGMA foreign_keys = ON`)
		requireFixturef(t, err == nil, "enable foreign keys: %v", err)
	}

	kr := testKeyring(t)
	{
		err := MigrateWithKeyring(db, kr)
		requireFixturef(t, err == nil, "first migrate: %v", err)
	}

	{
		_, err := db.Exec(`UPDATE subscription_settings SET refresh_hours = 0 WHERE id = 1`)
		requireFixturef(t, err == nil, "prepare no-startup-data-fix assertion: %v", err)
	}

	{
		err := MigrateWithKeyring(db, kr)
		requireFixturef(t, err == nil, "second migrate: %v", err)
	}

	var refreshHours int
	{
		err := db.QueryRow(`SELECT refresh_hours FROM subscription_settings WHERE id = 1`).Scan(&refreshHours)
		requireFixturef(t, err == nil, "read subscription settings: %v", err)
	}

	requireFixturef(t, refreshHours == 0, "startup modified data outside a versioned migration: refresh_hours=%d", refreshHours)

	var versions int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versions)
		requireFixturef(t, err == nil, "count schema migrations: %v", err)
	}

	requireFixturef(t, versions == len(SchemaMigrations), "schema migration count = %d, want %d", versions, len(SchemaMigrations))

	assertSourceOwnedSchemaMetadata(t, db)

	for _, table := range []string{"audit_events", "subscription_templates", "response_rules", "background_jobs", "api_tokens"} {
		var count int
		{
			err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count)
			requireFixturef(t, err == nil, "lookup table %s: %v", table, err)
		}

		requireFixturef(t, count == 1, "table %s was not created", table)

	}

	var fallbackRules int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM response_rules WHERE name = 'Fallback'`).Scan(&fallbackRules)
		requireFixturef(t, err == nil, "count fallback rules: %v", err)
	}

	requireFixturef(t, fallbackRules == 1, "fallback rule count = %d, want 1", fallbackRules)

	{
		_, err := db.Exec(`INSERT INTO key_categories(name) VALUES('edge')`)
		requireFixturef(t, err == nil, "insert category: %v", err)
	}

	activeID, activeKey, err := kr.GetActiveEncryptionKey()
	requireFixturef(t, err == nil, "fixture GetActiveEncryptionKey: %v", err)

	_, bikKey, err := kr.GetActiveBlindIndexKey()
	requireFixturef(t, err == nil, "fixture GetActiveBlindIndexKey: %v", err)

	insRes, err := db.Exec(`INSERT INTO vless_keys(label, url_blind_index, category) VALUES('edge-key', ?, 'edge')`, profilestorage.ComputeBlindIndex(bikKey, "vless://migration-test"))
	requireFixturef(t, err == nil, "insert categorized key: %v", err)

	edgeID, err := insRes.LastInsertId()
	requireFixturef(t, err == nil, "fixture LastInsertId: %v", err)

	secEnv, err := profilestorage.Encrypt([]byte("vless://migration-test"), activeID, activeKey, edgeID)
	requireFixturef(t, err == nil, "fixture Encrypt: %v", err)

	{
		_, err := db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, edgeID, secEnv)
		requireFixturef(t, err == nil, "insert categorized key secret: %v", err)
	}

	var categoryID sql.NullInt64
	{
		err := db.QueryRow(`SELECT category_id FROM vless_keys WHERE label = 'edge-key'`).Scan(&categoryID)
		requireFixturef(t, err == nil, "read normalized category: %v", err)
	}

	requireFixture(t, categoryID.Valid, "category trigger did not persist the foreign key")
	requireFixture(t, categoryID.Int64 != 0, "category trigger did not persist the foreign key")

	var status, protocol, compatibility, warnings string
	var schemaVersion, healthFailures int
	var createdAt time.Time
	{
		err := db.QueryRow(`SELECT status, protocol, profile_schema_version, profile_compatibility, profile_warnings_json, health_failure_count, created_at FROM vless_keys WHERE label = 'edge-key'`).Scan(
			&status, &protocol, &schemaVersion, &compatibility, &warnings, &healthFailures, &createdAt,
		)
		requireFixturef(t, err == nil, "read rebuilt defaults: %v", err)
	}

	requireFixturef(t, status == "active", "rebuilt defaults changed: status=%q protocol=%q version=%d compatibility=%q warnings=%q failures=%d created=%v", status, protocol, schemaVersion, compatibility, warnings, healthFailures, createdAt)
	requireFixturef(t, protocol == "legacy", "rebuilt defaults changed: status=%q protocol=%q version=%d compatibility=%q warnings=%q failures=%d created=%v", status, protocol, schemaVersion, compatibility, warnings, healthFailures, createdAt)
	requireFixturef(t, schemaVersion == 0, "rebuilt defaults changed: status=%q protocol=%q version=%d compatibility=%q warnings=%q failures=%d created=%v", status, protocol, schemaVersion, compatibility, warnings, healthFailures, createdAt)
	requireFixturef(t, compatibility == "legacy", "rebuilt defaults changed: status=%q protocol=%q version=%d compatibility=%q warnings=%q failures=%d created=%v", status, protocol, schemaVersion, compatibility, warnings, healthFailures, createdAt)
	requireFixturef(t, warnings == "[]", "rebuilt defaults changed: status=%q protocol=%q version=%d compatibility=%q warnings=%q failures=%d created=%v", status, protocol, schemaVersion, compatibility, warnings, healthFailures, createdAt)
	requireFixturef(t, healthFailures == 0, "rebuilt defaults changed: status=%q protocol=%q version=%d compatibility=%q warnings=%q failures=%d created=%v", status, protocol, schemaVersion, compatibility, warnings, healthFailures, createdAt)
	requireFixturef(t, !(createdAt.IsZero()), "rebuilt defaults changed: status=%q protocol=%q version=%d compatibility=%q warnings=%q failures=%d created=%v", status, protocol, schemaVersion, compatibility, warnings, healthFailures, createdAt)

}
