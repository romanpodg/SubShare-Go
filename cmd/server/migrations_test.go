package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

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

	if err := migrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var versions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatalf("count schema migrations: %v", err)
	}
	if versions != len(schemaMigrations) {
		t.Fatalf("schema migration count = %d, want %d", versions, len(schemaMigrations))
	}

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
	if _, err := db.Exec(`INSERT INTO vless_keys(label, url, category) VALUES('edge-key', 'vless://migration-test', 'edge')`); err != nil {
		t.Fatalf("insert categorized key: %v", err)
	}
	var categoryID sql.NullInt64
	if err := db.QueryRow(`SELECT category_id FROM vless_keys WHERE label = 'edge-key'`).Scan(&categoryID); err != nil {
		t.Fatalf("read normalized category: %v", err)
	}
	if !categoryID.Valid || categoryID.Int64 == 0 {
		t.Fatal("category trigger did not persist the foreign key")
	}
}
