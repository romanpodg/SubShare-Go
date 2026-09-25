package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func createTestAppWithKeyring(t *testing.T) (*App, *profilestorage.Keyring) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_encryption.db")
	kr := testKeyring(t)
	db, err := initializeSQLiteWithJournalMode(dbPath, "DELETE", kr)
	if err != nil {
		t.Fatalf("initialize db: %v", err)
	}
	app := &App{
		db:                       db,
		dbPath:                   dbPath,
		profileKeyring:           kr,
		subscriptionBodyEncoding: "base64",
		profileFingerprintKey:    []byte("0123456789abcdef0123456789abcdef"),
	}
	return app, kr
}

func TestStage7CreateAndUpdateKeyEncryption(t *testing.T) {
	app, kr := createTestAppWithKeyring(t)
	defer app.db.Close()

	activeID, activeKey, err := kr.GetActiveEncryptionKey()
	if err != nil {
		t.Fatalf("active key: %v", err)
	}
	_, bikKey, err := kr.GetActiveBlindIndexKey()
	if err != nil {
		t.Fatalf("bik key: %v", err)
	}

	rawURI := "vless://11111111-2222-3333-4444-555555555555@example.com:443?type=tcp#TestKey"
	blindIndex := profilestorage.ComputeBlindIndex(bikKey, rawURI)

	res, err := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, category, status, key_kind, sort_order) VALUES('TestKey', ?, '', 'active', 'real', 1)`, blindIndex)
	if err != nil {
		t.Fatalf("insert key parent: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	env, err := profilestorage.Encrypt([]byte(rawURI), activeID, activeKey, id)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	if _, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, id, env); err != nil {
		t.Fatalf("insert secret: %v", err)
	}

	var dummy string
	err = app.db.QueryRow(`SELECT url FROM vless_keys WHERE id = ?`, id).Scan(&dummy)
	if err == nil {
		t.Fatal("expected SELECT url FROM vless_keys to fail (column removed in migration 13)")
	}

	var fetchedEnv string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, id).Scan(&fetchedEnv); err != nil {
		t.Fatalf("fetch secret: %v", err)
	}
	dec, err := profilestorage.Decrypt(fetchedEnv, kr, id)
	if err != nil {
		t.Fatalf("decrypt secret: %v", err)
	}
	if dec.Reveal() != rawURI {
		t.Fatalf("decrypted URI = %q, want %q", dec.Reveal(), rawURI)
	}

	_, dupErr := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, category, status, key_kind, sort_order) VALUES('DupKey', ?, '', 'active', 'real', 2)`, blindIndex)
	if dupErr == nil {
		t.Fatal("expected duplicate local blind index insert to fail")
	}
}

func TestStage7SubscriptionDeliveryCorruptedRowExclusion(t *testing.T) {
	app, kr := createTestAppWithKeyring(t)
	defer app.db.Close()

	subID := "sub-encrypted-test"
	if _, err := app.db.Exec(`INSERT INTO users(name, token, subscription_id) VALUES('Test User', 'token123', ?)`, subID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var userID int64
	_ = app.db.QueryRow(`SELECT id FROM users WHERE subscription_id = ?`, subID).Scan(&userID)

	activeID, activeKey, _ := kr.GetActiveEncryptionKey()
	_, bikKey, _ := kr.GetActiveBlindIndexKey()

	validURI := "vless://valid-user@example.com:443?type=tcp#Valid"
	res1, _ := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, status, key_kind, sort_order) VALUES('ValidKey', ?, 'active', 'real', 1)`, profilestorage.ComputeBlindIndex(bikKey, validURI))
	id1, _ := res1.LastInsertId()
	env1, _ := profilestorage.Encrypt([]byte(validURI), activeID, activeKey, id1)
	_, _ = app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, id1, env1)
	_, _ = app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, id1)

	res2, _ := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, status, key_kind, sort_order) VALUES('CorruptKey', ?, 'active', 'real', 2)`, profilestorage.ComputeBlindIndex(bikKey, "corrupt-uri"))
	id2, _ := res2.LastInsertId()
	_, _ = app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, id2, "$subshare-profile$v1$xchacha20poly1305$key-1$badnonce$badcipher")
	_, _ = app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, id2)

	gen, denial, err := app.generateSelectedSubscription(subID, "links")
	if err != nil || denial.Code != 0 {
		t.Fatalf("generateSelectedSubscription err=%v denial.Code=%d", err, denial.Code)
	}
	if !strings.Contains(gen.Body, validURI) {
		t.Fatalf("generated body does not contain valid URI: %s", gen.Body)
	}
	foundExclusion := false
	for _, ex := range gen.Exclusions {
		if ex.Reason == "profile_storage_integrity_error" {
			foundExclusion = true
			break
		}
	}
	if !foundExclusion {
		t.Fatalf("expected exclusion with reason profile_storage_integrity_error, got %#v", gen.Exclusions)
	}

	_, _ = app.db.Exec(`DELETE FROM user_keys WHERE key_id = ?`, id1)
	_, denial503, _ := app.generateSelectedSubscription(subID, "links")
	if denial503.Code != 503 {
		t.Fatalf("expected 503 when all eligible rows are corrupt, got %d", denial503.Code)
	}
}

func TestStage7ValidateBackup(t *testing.T) {
	app, kr := createTestAppWithKeyring(t)
	defer app.db.Close()

	backupFile := filepath.Join(t.TempDir(), "backup_test.db")
	if err := app.performBackup(backupFile); err != nil {
		t.Fatalf("perform backup: %v", err)
	}

	if err := runValidateBackup(backupFile, kr); err != nil {
		t.Fatalf("validate backup: %v", err)
	}
}

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
