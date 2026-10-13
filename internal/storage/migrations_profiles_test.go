package storage

import (
	"database/sql"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfilePersistenceMigrationUpgradesPopulatedVersionEightDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "pre-profile.db"))
	requireFixturef(t, err == nil, "open sqlite: %v", err)

	defer db.Close()
	createPopulatedPreStage5Schema(t, db, 8)
	kr := testKeyring(t)
	{
		err := MigrateWithKeyring(db, kr)
		requireFixturef(t, err == nil, "upgrade version-eight database: %v", err)
	}

	var label, encURL, ref, protocol, compatibility, warnings, createdAt string
	var schemaVersion int
	{
		err := db.QueryRow(`SELECT k.label, s.encrypted_url, k.external_key_ref, k.protocol, k.profile_schema_version, k.profile_compatibility, k.profile_warnings_json, k.created_at FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = 101`).Scan(
			&label, &encURL, &ref, &protocol, &schemaVersion, &compatibility, &warnings, &createdAt,
		)
		requireFixturef(t, err == nil, "read upgraded row: %v", err)
	}

	sec, err := profilestorage.Decrypt(encURL, kr, 101)
	requireFixturef(t, err == nil, "decrypt upgraded row secret: %v", err)

	raw := sec.Reveal()
	requireFixturef(t, label == "vless-existing", "existing row changed during upgrade: label=%q raw=%q ref=%q protocol=%q version=%d compatibility=%q warnings=%q created=%q", label, raw, ref, protocol, schemaVersion, compatibility, warnings, createdAt)
	requireFixturef(t, raw == "vless://legacy-secret@vless.example:443", "existing row changed during upgrade: label=%q raw=%q ref=%q protocol=%q version=%d compatibility=%q warnings=%q created=%q", label, raw, ref, protocol, schemaVersion, compatibility, warnings, createdAt)
	requireFixturef(t, ref == "stable-vless-ref", "existing row changed during upgrade: label=%q raw=%q ref=%q protocol=%q version=%d compatibility=%q warnings=%q created=%q", label, raw, ref, protocol, schemaVersion, compatibility, warnings, createdAt)
	requireFixturef(t, protocol == "legacy", "existing row changed during upgrade: label=%q raw=%q ref=%q protocol=%q version=%d compatibility=%q warnings=%q created=%q", label, raw, ref, protocol, schemaVersion, compatibility, warnings, createdAt)
	requireFixturef(t, schemaVersion == 0, "existing row changed during upgrade: label=%q raw=%q ref=%q protocol=%q version=%d compatibility=%q warnings=%q created=%q", label, raw, ref, protocol, schemaVersion, compatibility, warnings, createdAt)
	requireFixturef(t, compatibility == "legacy", "existing row changed during upgrade: label=%q raw=%q ref=%q protocol=%q version=%d compatibility=%q warnings=%q created=%q", label, raw, ref, protocol, schemaVersion, compatibility, warnings, createdAt)
	requireFixturef(t, warnings == "[]", "existing row changed during upgrade: label=%q raw=%q ref=%q protocol=%q version=%d compatibility=%q warnings=%q created=%q", label, raw, ref, protocol, schemaVersion, compatibility, warnings, createdAt)
	requireFixturef(t, strings.HasPrefix(createdAt, "2025-01-02"), "existing row changed during upgrade: label=%q raw=%q ref=%q protocol=%q version=%d compatibility=%q warnings=%q created=%q", label, raw, ref, protocol, schemaVersion, compatibility, warnings, createdAt)

	var preservedRows int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id IN (101,102,103) AND label IN ('vless-existing','vmess-existing','trojan-existing')`).Scan(&preservedRows)
		requireFixturef(t, err == nil, "existing protocol rows merged or removed: count=%d err=%v", preservedRows, err)
		requireFixturef(t, preservedRows == 3, "existing protocol rows merged or removed: count=%d err=%v", preservedRows, err)
	}

	assertSourceOwnedSchemaMetadata(t, db)

	var assignmentCount int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = 501 AND key_id IN (101, 102)`).Scan(&assignmentCount)
		requireFixturef(t, err == nil, "assignments did not survive migration: count=%d err=%v", assignmentCount, err)
		requireFixturef(t, assignmentCount == 2, "assignments did not survive migration: count=%d err=%v", assignmentCount, err)
	}

	activeID, activeKey, err := kr.GetActiveEncryptionKey()
	requireFixturef(t, err == nil, "fixture GetActiveEncryptionKey: %v", err)

	_, bikKey, err := kr.GetActiveBlindIndexKey()
	requireFixturef(t, err == nil, "fixture GetActiveBlindIndexKey: %v", err)

	inserted, err := db.Exec(`INSERT INTO vless_keys(label, url_blind_index, external_source_id, external_key_ref, protocol, profile_fingerprint, profile_schema_version, profile_compatibility)
		VALUES('same raw, second source', ?, 20, 'stable-vless-ref', 'vless', 'pf1_same', 1, 'full')`, profilestorage.ComputeBlindIndex(bikKey, "vless://legacy-secret@vless.example:443"))
	requireFixturef(t, err == nil, "insert identical raw URI for second source after migration: %v", err)

	insertedID, err := inserted.LastInsertId()
	requireFixturef(t, err == nil, "AUTOINCREMENT sequence was not preserved: id=%d err=%v", insertedID, err)
	requireFixturef(t, insertedID > 103, "AUTOINCREMENT sequence was not preserved: id=%d err=%v", insertedID, err)

	secEnv, err := profilestorage.Encrypt([]byte("vless://legacy-secret@vless.example:443"), activeID, activeKey, insertedID)
	requireFixturef(t, err == nil, "fixture Encrypt: %v", err)

	{
		_, err := db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, insertedID, secEnv)
		requireFixturef(t, err == nil, "fixture Exec: %v", err)
	}

	{
		_, err := db.Exec(`INSERT INTO vless_keys(label, url_blind_index, external_source_id, external_key_ref, protocol, profile_fingerprint)
		VALUES('same semantic, same source', ?, 20, 'different-ref', 'vless', 'pf1_same')`, profilestorage.ComputeBlindIndex(bikKey, "vless://byte-different@example.com:443"))
		requireFixture(t, err != nil, "source-scoped semantic fingerprint uniqueness was not enforced")
	}

	{
		_, err := db.Exec(`INSERT INTO vless_keys(label, url_blind_index) VALUES('duplicate local', ?)`, profilestorage.ComputeBlindIndex(bikKey, "trojan://legacy-secret@trojan.example:443"))
		requireFixture(t, err != nil, "local-key URL uniqueness was not preserved")
	}

}

func TestSourceOwnedURLMigrationRollsBackWithoutMergingRows(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "rollback.db"))
	requireFixturef(t, err == nil, "open sqlite: %v", err)

	defer db.Close()
	createPopulatedPreStage5Schema(t, db, 9)
	{
		_, err := db.Exec(`INSERT INTO vless_keys(
		id, label, url, created_at, check_status, status, key_kind, sort_order, starts_at,
		external_source_id, external_key_ref, category, category_id, health_failure_count,
		protocol, profile_fingerprint, profile_schema_version, profile_compatibility, profile_warnings_json
	) VALUES(104, 'conflicting semantic row', 'hy2://different-auth@example.com', '2025-01-05T00:00:00Z',
		'unknown', 'active', 'real', 4, '2025-01-05T00:00:00Z', 10, 'different-ref', 'edge', 1, 0,
		'hysteria2', 'pf1_existing', 1, 'full', '[]')`)
		requireFixturef(t, err == nil, "insert pre-migration fingerprint conflict: %v", err)
	}

	{
		_, err := db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(501, 104)`)
		requireFixturef(t, err == nil, "assign conflicting row: %v", err)
	}

	kr := testKeyring(t)
	{
		err := MigrateWithKeyring(db, kr)
		requireFixturef(t, err != nil, "expected safe unique-index migration failure, got %v", err)
		requireFixturef(t, strings.Contains(err.Error(), "source_owned_profile_urls"), "expected safe unique-index migration failure, got %v", err)
	}

	var rowCount, assignments, applied int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys`).Scan(&rowCount)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = 501`).Scan(&assignments)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 10`).Scan(&applied)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	requireFixturef(t, rowCount == 4, "failed migration mutated data: rows=%d assignments=%d applied=%d", rowCount, assignments, applied)
	requireFixturef(t, assignments == 3, "failed migration mutated data: rows=%d assignments=%d applied=%d", rowCount, assignments, applied)
	requireFixturef(t, applied == 0, "failed migration mutated data: rows=%d assignments=%d applied=%d", rowCount, assignments, applied)

	var tableSQL string
	{
		err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'vless_keys'`).Scan(&tableSQL)
		requireFixturef(t, err == nil, "rollback did not restore old table: sql=%q err=%v", tableSQL, err)
		requireFixturef(t, strings.Contains(strings.ToUpper(tableSQL), "URL TEXT NOT NULL UNIQUE"), "rollback did not restore old table: sql=%q err=%v", tableSQL, err)
	}

	var foreignKeysEnabled int
	{
		err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeysEnabled)
		requireFixturef(t, err == nil, "failed migration did not restore foreign keys: enabled=%d err=%v", foreignKeysEnabled, err)
		requireFixturef(t, foreignKeysEnabled == 1, "failed migration did not restore foreign keys: enabled=%d err=%v", foreignKeysEnabled, err)
	}

}

func assertSourceOwnedSchemaMetadata(t *testing.T, db *sql.DB) {
	t.Helper()
	var tableSQL string
	{
		err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'vless_keys'`).Scan(&tableSQL)
		requireFixturef(t, err == nil, "read vless_keys SQL: %v", err)
	}

	requireFixturef(t, !(strings.Contains(strings.ToUpper(tableSQL), "URL TEXT NOT NULL UNIQUE")), "global URL uniqueness survived migration: %s", tableSQL)

	indexes := map[string]struct {
		unique  int
		partial int
	}{}
	rows, err := db.Query(`PRAGMA index_list(vless_keys)`)
	requireFixturef(t, err == nil, "list vless indexes: %v", err)

	for rows.Next() {
		var sequence, unique, partial int
		var name, origin string
		{
			err := rows.Scan(&sequence, &name, &unique, &origin, &partial)
			requireFixturef(t, err == nil, "scan vless index: %v", err)
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
		{
			got, ok := indexes[name]
			requireFixturef(t, ok, "index %s metadata=%#v present=%v, want %#v", name, got, ok, wanted)
			requireFixturef(t, got == wanted, "index %s metadata=%#v present=%v, want %#v", name, got, ok, wanted)
		}

	}
	var triggers int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name IN ('trg_vless_keys_category_insert','trg_vless_keys_category_id_insert','trg_vless_keys_category_id_update','trg_key_categories_name_compat')`).Scan(&triggers)
		requireFixturef(t, err == nil, "vless trigger count=%d err=%v", triggers, err)
		requireFixturef(t, triggers == 4, "vless trigger count=%d err=%v", triggers, err)
	}

	var violations int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&violations)
		requireFixturef(t, err == nil, "foreign key violations=%d err=%v", violations, err)
		requireFixturef(t, violations == 0, "foreign key violations=%d err=%v", violations, err)
	}

	var foreignKeysEnabled int
	{
		err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeysEnabled)
		requireFixturef(t, err == nil, "foreign keys were not restored: enabled=%d err=%v", foreignKeysEnabled, err)
		requireFixturef(t, foreignKeysEnabled == 1, "foreign keys were not restored: enabled=%d err=%v", foreignKeysEnabled, err)
	}

	var keyForeignKey int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM pragma_foreign_key_list('user_keys') WHERE "table" = 'vless_keys'`).Scan(&keyForeignKey)
		requireFixturef(t, err == nil, "user_keys parent metadata=%d err=%v", keyForeignKey, err)
		requireFixturef(t, keyForeignKey == 1, "user_keys parent metadata=%d err=%v", keyForeignKey, err)
	}

}
