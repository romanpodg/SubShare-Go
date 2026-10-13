package main

import (
	"database/sql"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func profileFixtureInsertID(t *testing.T, result sql.Result) int64 {
	t.Helper()
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read inserted profile fixture ID: %v", err)
	}
	return id
}

func insertProfileFixtureSecret(t *testing.T, app *App, keyID int64, raw string) {
	t.Helper()
	activeID, activeKey, err := app.profileKeyring.GetActiveEncryptionKey()
	if err != nil {
		t.Fatalf("load fixture encryption key: %v", err)
	}
	envelope, err := profilestorage.Encrypt([]byte(raw), activeID, activeKey, keyID)
	if err != nil {
		t.Fatalf("encrypt profile fixture: %v", err)
	}
	if _, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, envelope); err != nil {
		t.Fatalf("insert profile fixture secret: %v", err)
	}
}

func assignProfileFixtureUsers(t *testing.T, app *App, keyID int64, userIDs ...int64) {
	t.Helper()
	for _, userID := range userIDs {
		if _, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, keyID); err != nil {
			t.Fatalf("assign profile fixture: %v", err)
		}
	}
}

func profileFixtureCount(t *testing.T, app *App, query string, args ...any) int {
	t.Helper()
	var count int
	if err := app.db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count profile fixture rows: %v", err)
	}
	return count
}

func profileFixtureRaw(t *testing.T, app *App, keyID int64) string {
	t.Helper()
	var envelope string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelope); err != nil {
		t.Fatalf("read profile fixture secret: %v", err)
	}
	secret, err := profilestorage.Decrypt(envelope, app.profileKeyring, keyID)
	if err != nil {
		t.Fatalf("decrypt profile fixture: %v", err)
	}
	return secret.Reveal()
}
