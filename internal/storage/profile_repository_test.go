package storage

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func newTestKeyringForRepo(t *testing.T) *profilestorage.Keyring {
	t.Helper()
	data, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	kr, err := profilestorage.LoadKeyringJSON(data)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	return kr
}

func generateTestKeyring(t *testing.T, keyID, keySecret, bikID, bikSecret string) *profilestorage.Keyring {
	t.Helper()
	keyB64 := base64.RawURLEncoding.EncodeToString([]byte(keySecret))
	bikB64 := base64.RawURLEncoding.EncodeToString([]byte(bikSecret))
	jsonStr := fmt.Sprintf(`{
		"active_key_id": %q,
		"keys": {%q: %q},
		"active_blind_index_key_id": %q,
		"blind_index_keys": {%q: %q}
	}`, keyID, keyID, keyB64, bikID, bikID, bikB64)
	kr, err := profilestorage.LoadKeyringJSON([]byte(jsonStr))
	if err != nil {
		t.Fatalf("load keyring (%s, %s): %v", keyID, bikID, err)
	}
	return kr
}

func newTwoKeyKeyring(t *testing.T) (*profilestorage.Keyring, *profilestorage.Keyring, *profilestorage.Keyring) {
	t.Helper()
	keySecret1 := "01234567890123456789012345678901"
	bikSecret1 := "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	if keySecret1 == bikSecret1 {
		t.Fatal("fixture error: encryption and BIK secrets must not be equal")
	}

	krBase := generateTestKeyring(t, "key-1", keySecret1, "bik-1", bikSecret1)
	krDiffBIK := generateTestKeyring(t, "key-1", keySecret1, "bik-2", "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
	krDiffEnc := generateTestKeyring(t, "key-2", "98765432109876543210987654321098", "bik-1", bikSecret1)

	return krBase, krDiffBIK, krDiffEnc
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_repo.db")
	db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	schema := `
		CREATE TABLE key_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			color TEXT NOT NULL DEFAULT '#4B5563',
			sort_order INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE external_subscription_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			key_category_id INTEGER,
			key_category TEXT
		);
		CREATE TABLE vless_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL,
			url_blind_index TEXT,
			category_id INTEGER,
			category TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			check_status TEXT NOT NULL DEFAULT 'unknown',
			check_error TEXT,
			last_checked_at DATETIME,
			last_latency_ms INTEGER,
			key_kind TEXT NOT NULL DEFAULT 'real',
			template_text TEXT,
			sort_order INTEGER NOT NULL DEFAULT 0,
			external_source_id INTEGER,
			external_key_ref TEXT,
			protocol TEXT NOT NULL DEFAULT 'vless',
			profile_schema_version INTEGER NOT NULL DEFAULT 1,
			profile_compatibility TEXT NOT NULL DEFAULT 'full',
			profile_warnings_json TEXT NOT NULL DEFAULT '[]',
			profile_revision INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE vless_key_secrets (
			vless_key_id INTEGER PRIMARY KEY REFERENCES vless_keys(id) ON DELETE CASCADE,
			encrypted_url TEXT
		);
		CREATE UNIQUE INDEX idx_vless_keys_local_blind_index
			ON vless_keys(url_blind_index) WHERE external_source_id IS NULL;
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_assignment_mode TEXT NOT NULL DEFAULT 'all'
		);
		CREATE TABLE user_keys (
			user_id INTEGER NOT NULL,
			key_id INTEGER NOT NULL,
			PRIMARY KEY (user_id, key_id)
		);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("seed schema: %v", err)
	}
	return db
}

func TestProfileRepository_Create_Update_Clone(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForRepo(t)
	repo := NewProfileRepository(db, kr)

	// 1. Create Local Profile
	vlessURI := "vless://user1@example.com:443?encryption=none#Node1"
	createdKey, decryptedURI, err := repo.CreateLocal(ctx, profilepersistence.CreateProfileParams{
		Label:    "Key One",
		Status:   "active",
		Kind:     "real",
		Category: "General",
		Protocol: "vless",
		BuiltURI: vlessURI,
	})
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}
	if createdKey.ID <= 0 || createdKey.Label != "Key One" || createdKey.ProfileRevision != 1 {
		t.Fatalf("unexpected created key metadata: %#v", createdKey)
	}
	if decryptedURI != vlessURI {
		t.Fatalf("decryptedURI = %q, want %q", decryptedURI, vlessURI)
	}

	// Verify plaintext URI is absent from parent storage table
	var parentBlindIndex string
	if err := db.QueryRow(`SELECT url_blind_index FROM vless_keys WHERE id = ?`, createdKey.ID).Scan(&parentBlindIndex); err != nil {
		t.Fatalf("query blind index: %v", err)
	}
	if parentBlindIndex == "" || parentBlindIndex == vlessURI {
		t.Fatalf("invalid parent blind index: %q", parentBlindIndex)
	}

	// 2. Update Local Profile with correct revision
	newURI := "vless://user1-updated@example.com:443?encryption=none#Node1Updated"
	updatedKey, updatedURI, err := repo.UpdateLocal(ctx, profilepersistence.UpdateProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 1,
		Label:            "Key One Updated",
		Status:           "active",
		Kind:             "real",
		Category:         "General",
		Protocol:         "vless",
		NewURI:           newURI,
	})
	if err != nil {
		t.Fatalf("UpdateLocal: %v", err)
	}
	if updatedKey.ProfileRevision != 2 || updatedKey.Label != "Key One Updated" {
		t.Fatalf("unexpected updated key metadata: %#v", updatedKey)
	}
	if updatedURI != newURI {
		t.Fatalf("updatedURI = %q, want %q", updatedURI, newURI)
	}

	// 3. Stale revision update conflict
	_, _, err = repo.UpdateLocal(ctx, profilepersistence.UpdateProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 1, // stale revision!
		Label:            "Stale Edit",
		Status:           "active",
		Kind:             "real",
		Protocol:         "vless",
		NewURI:           newURI,
	})
	if !errors.Is(err, profilepersistence.ErrProfileRevisionConflict) {
		t.Fatalf("expected ErrProfileRevisionConflict, got %v", err)
	}

	// 4. Source-owned read-only rejection
	if _, err := db.Exec(`UPDATE vless_keys SET external_source_id = 99 WHERE id = ?`, createdKey.ID); err != nil {
		t.Fatalf("set external_source_id: %v", err)
	}
	_, _, err = repo.UpdateLocal(ctx, profilepersistence.UpdateProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 2,
		Label:            "Forbidden Edit",
		Status:           "active",
		Kind:             "real",
		Protocol:         "vless",
		NewURI:           newURI,
	})
	if !errors.Is(err, profilepersistence.ErrSourceOwnedProfile) {
		t.Fatalf("expected ErrSourceOwnedProfile, got %v", err)
	}

	// Reset source ID for clone test
	if _, err := db.Exec(`UPDATE vless_keys SET external_source_id = NULL WHERE id = ?`, createdKey.ID); err != nil {
		t.Fatalf("clear external_source_id: %v", err)
	}

	// 5. Clone Local Profile
	clonedKey, clonedURI, err := repo.CloneLocal(ctx, profilepersistence.CloneProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 2,
		NewLabel:         "Key One Copy",
	})
	if err != nil {
		t.Fatalf("CloneLocal: %v", err)
	}
	if clonedKey.ID == createdKey.ID || clonedKey.Label != "Key One Copy" || clonedKey.ProfileRevision != 1 {
		t.Fatalf("unexpected cloned key metadata: %#v", clonedKey)
	}
	if clonedKey.ExternalSourceID != 0 {
		t.Fatalf("cloned key external_source_id = %d, want 0", clonedKey.ExternalSourceID)
	}
	if clonedURI != newURI {
		t.Fatalf("clonedURI = %q, want %q", clonedURI, newURI)
	}

	// 6. Metadata-only updates to a clone must retain its intentional
	// non-canonical blind index instead of colliding with the source row.
	updatedClone, updatedCloneURI, err := repo.UpdateLocal(ctx, profilepersistence.UpdateProfileParams{
		ID:               clonedKey.ID,
		ExpectedRevision: 1,
		Label:            "Key One Copy Renamed",
		Status:           "active",
		Kind:             "real",
		Category:         "General",
		Protocol:         "vless",
		NewURI:           clonedURI,
	})
	if err != nil {
		t.Fatalf("UpdateLocal clone metadata: %v", err)
	}
	if updatedClone.ProfileRevision != 2 || updatedClone.Label != "Key One Copy Renamed" {
		t.Fatalf("unexpected updated clone metadata: %#v", updatedClone)
	}
	if updatedCloneURI != clonedURI {
		t.Fatalf("updated clone URI = %q, want %q", updatedCloneURI, clonedURI)
	}
}

func TestProfileRepository_BlindIndexKeySeparation(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	krBase, krDiffBIK, krDiffEnc := newTwoKeyKeyring(t)
	repoBase := NewProfileRepository(db, krBase)
	repoDiffBIK := NewProfileRepository(db, krDiffBIK)
	repoDiffEnc := NewProfileRepository(db, krDiffEnc)

	uri := "vless://secret-user@example.com:443?encryption=none#TestBlindIndex"

	// Create key with base keyring (encKey: key-1, BIK: bik-1)
	key1, _, err := repoBase.CreateLocal(ctx, profilepersistence.CreateProfileParams{
		Label:    "Key 1",
		Status:   "active",
		Kind:     "real",
		Protocol: "vless",
		BuiltURI: uri,
	})
	if err != nil {
		t.Fatalf("CreateLocal 1: %v", err)
	}

	var blindIndex1, cipherText1 string
	if err := db.QueryRow(`SELECT k.url_blind_index, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = ?`, key1.ID).Scan(&blindIndex1, &cipherText1); err != nil {
		t.Fatalf("query key1: %v", err)
	}

	// 1. Plaintext URI is NOT stored in parent row or blind index
	if blindIndex1 == uri {
		t.Fatalf("url_blind_index matches raw URI")
	}

	// 2. Create key with diff BIK keyring (encKey: key-1, BIK: bik-2)
	key2, _, err := repoDiffBIK.CreateLocal(ctx, profilepersistence.CreateProfileParams{
		Label:    "Key 2",
		Status:   "active",
		Kind:     "real",
		Protocol: "vless",
		BuiltURI: uri,
	})
	if err != nil {
		t.Fatalf("CreateLocal 2: %v", err)
	}

	var blindIndex2 string
	if err := db.QueryRow(`SELECT url_blind_index FROM vless_keys WHERE id = ?`, key2.ID).Scan(&blindIndex2); err != nil {
		t.Fatalf("query key2: %v", err)
	}

	// Changing ONLY blind index key MUST change url_blind_index
	if blindIndex1 == blindIndex2 {
		t.Fatalf("expected different blind indexes when BIK changes, got equal: %q", blindIndex1)
	}

	// Production rejects duplicate deterministic blind indexes. Remove the
	// first row after capturing its ciphertext so the next assertion can
	// isolate encryption-key rotation from duplicate detection.
	if _, err := db.Exec(`DELETE FROM vless_keys WHERE id = ?`, key1.ID); err != nil {
		t.Fatalf("delete key1 before encryption-key comparison: %v", err)
	}

	// 3. Create key with diff Enc keyring (encKey: key-2, BIK: bik-1)
	key3, _, err := repoDiffEnc.CreateLocal(ctx, profilepersistence.CreateProfileParams{
		Label:    "Key 3",
		Status:   "active",
		Kind:     "real",
		Protocol: "vless",
		BuiltURI: uri,
	})
	if err != nil {
		t.Fatalf("CreateLocal 3: %v", err)
	}

	var blindIndex3, cipherText3 string
	if err := db.QueryRow(`SELECT k.url_blind_index, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = ?`, key3.ID).Scan(&blindIndex3, &cipherText3); err != nil {
		t.Fatalf("query key3: %v", err)
	}

	// With SAME raw URI and SAME BIK (bik-1), different encryption keys MUST produce EXACT SAME url_blind_index
	if blindIndex1 != blindIndex3 {
		t.Fatalf("expected identical blind index when BIK is identical, got %q vs %q", blindIndex1, blindIndex3)
	}

	// Proven decryption test: cipherText1 decrypts with krBase (key-1) but FAILS with krDiffEnc (key-2)
	dec1, decErr1 := profilestorage.Decrypt(cipherText1, krBase, key1.ID)
	if decErr1 != nil || dec1.Reveal() != uri {
		t.Fatalf("failed to decrypt cipherText1 with base keyring: %v", decErr1)
	}
	_, decErr1WrongKey := profilestorage.Decrypt(cipherText1, krDiffEnc, key1.ID)
	if decErr1WrongKey == nil {
		t.Fatalf("expected decryption failure when decrypting cipherText1 with wrong encryption key (krDiffEnc)")
	}

	// cipherText3 decrypts with krDiffEnc (key-2) but FAILS with krBase (key-1)
	dec3, decErr3 := profilestorage.Decrypt(cipherText3, krDiffEnc, key3.ID)
	if decErr3 != nil || dec3.Reveal() != uri {
		t.Fatalf("failed to decrypt cipherText3 with diffEnc keyring: %v", decErr3)
	}
	_, decErr3WrongKey := profilestorage.Decrypt(cipherText3, krBase, key3.ID)
	if decErr3WrongKey == nil {
		t.Fatalf("expected decryption failure when decrypting cipherText3 with wrong encryption key (krBase)")
	}
}

func TestProfileRepository_ErrorClassifications(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForRepo(t)
	repo := NewProfileRepository(db, kr)

	// 1. Get non-existent key -> ErrProfileNotFound
	_, _, err := repo.GetByID(ctx, 999)
	if !errors.Is(err, profilepersistence.ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound on GetByID(999), got %v", err)
	}

	// 2. Update non-existent key -> ErrProfileNotFound
	_, _, err = repo.UpdateLocal(ctx, profilepersistence.UpdateProfileParams{
		ID:               999,
		ExpectedRevision: 1,
		Label:            "Non-existent",
		Status:           "active",
		Kind:             "real",
		Protocol:         "vless",
		NewURI:           "vless://a@b.com:443#a",
	})
	if !errors.Is(err, profilepersistence.ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound on UpdateLocal(999), got %v", err)
	}

	// 3. Clone non-existent key -> ErrProfileNotFound
	_, _, err = repo.CloneLocal(ctx, profilepersistence.CloneProfileParams{
		ID:               999,
		ExpectedRevision: 1,
		NewLabel:         "Clone non-existent",
	})
	if !errors.Is(err, profilepersistence.ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound on CloneLocal(999), got %v", err)
	}

	// Create valid base key
	createdKey, _, err := repo.CreateLocal(ctx, profilepersistence.CreateProfileParams{
		Label:    "Key Test",
		Status:   "active",
		Kind:     "real",
		Protocol: "vless",
		BuiltURI: "vless://test@example.com:443#Test",
	})
	if err != nil {
		t.Fatalf("CreateLocal: %v", err)
	}

	// 4. Update with stale revision -> ErrProfileRevisionConflict
	_, _, err = repo.UpdateLocal(ctx, profilepersistence.UpdateProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 99,
		Label:            "Stale Revision",
		Status:           "active",
		Kind:             "real",
		Protocol:         "vless",
		NewURI:           "vless://test@example.com:443#Stale",
	})
	if !errors.Is(err, profilepersistence.ErrProfileRevisionConflict) {
		t.Fatalf("expected ErrProfileRevisionConflict on UpdateLocal stale rev, got %v", err)
	}

	// Verify failed update leaves parent and child unchanged
	var uneditedLabel string
	if err := db.QueryRow(`SELECT label FROM vless_keys WHERE id = ?`, createdKey.ID).Scan(&uneditedLabel); err != nil || uneditedLabel != "Key Test" {
		t.Fatalf("parent row modified after failed update: %q", uneditedLabel)
	}

	// 5. Clone with stale revision -> ErrProfileRevisionConflict
	_, _, err = repo.CloneLocal(ctx, profilepersistence.CloneProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 99,
		NewLabel:         "Stale Clone",
	})
	if !errors.Is(err, profilepersistence.ErrProfileRevisionConflict) {
		t.Fatalf("expected ErrProfileRevisionConflict on CloneLocal stale rev, got %v", err)
	}

	// 6. Missing child secret during clone -> ErrStorageIntegrity
	if _, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, createdKey.ID); err != nil {
		t.Fatalf("delete secret row: %v", err)
	}
	if _, _, err = repo.GetByID(ctx, createdKey.ID); !errors.Is(err, profilepersistence.ErrStorageIntegrity) {
		t.Fatalf("expected ErrStorageIntegrity on GetByID missing secret, got %v", err)
	}
	_, _, err = repo.CloneLocal(ctx, profilepersistence.CloneProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 1,
		NewLabel:         "Missing Secret Clone",
	})
	if !errors.Is(err, profilepersistence.ErrStorageIntegrity) {
		t.Fatalf("expected ErrStorageIntegrity on CloneLocal missing secret, got %v", err)
	}

	// 7. Corrupt ciphertext during clone -> ErrStorageIntegrity
	if _, err := db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, 'v1:key-1:corrupt_junk')`, createdKey.ID); err != nil {
		t.Fatalf("insert corrupt secret: %v", err)
	}
	if _, _, err = repo.GetByID(ctx, createdKey.ID); !errors.Is(err, profilepersistence.ErrStorageIntegrity) {
		t.Fatalf("expected ErrStorageIntegrity on GetByID corrupt ciphertext, got %v", err)
	}
	_, _, err = repo.CloneLocal(ctx, profilepersistence.CloneProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 1,
		NewLabel:         "Corrupt Clone",
	})
	if !errors.Is(err, profilepersistence.ErrStorageIntegrity) {
		t.Fatalf("expected ErrStorageIntegrity on CloneLocal corrupt ciphertext, got %v", err)
	}
}
