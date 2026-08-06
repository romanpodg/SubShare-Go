package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func TestStorage_LegacyKeyCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO users(id, key_assignment_mode) VALUES(1, 'all'), (2, 'manual')`); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	kr := newTestKeyringForStorage(t)
	repo := NewProfileRepository(db, kr)

	ctx := context.Background()
	vlessURI := "vless://11111111-1111-1111-1111-111111111111@example.com:443?type=tcp#LegacyStorage"

	// 1. Create Legacy Key
	keyID, err := repo.CreateLegacy(ctx, profilepersistence.CreateLegacyKeyParams{
		Label:        "Storage Legacy Key",
		Status:       "active",
		Kind:         "real",
		Category:     "DefaultCat",
		TemplateText: "",
		KeyURL:       vlessURI,
	})
	if err != nil {
		t.Fatalf("CreateLegacy failed: %v", err)
	}
	if keyID <= 0 {
		t.Fatalf("expected positive keyID, got %d", keyID)
	}

	var parentLabel, urlBlindIndex string
	if err := db.QueryRow(`SELECT label, url_blind_index FROM vless_keys WHERE id = ?`, keyID).Scan(&parentLabel, &urlBlindIndex); err != nil {
		t.Fatalf("query parent row: %v", err)
	}
	if parentLabel != "Storage Legacy Key" {
		t.Fatalf("unexpected label: %q", parentLabel)
	}
	if urlBlindIndex == "" {
		t.Fatalf("expected non-empty blind index")
	}

	var encURL string
	if err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&encURL); err != nil {
		t.Fatalf("query secret row: %v", err)
	}
	dec, err := profilestorage.Decrypt(encURL, kr, keyID)
	if err != nil || dec.Reveal() != vlessURI {
		t.Fatalf("decrypt failed or mismatched URI: err=%v, uri=%q", err, dec.Reveal())
	}

	var assignedUserCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE key_id = ?`, keyID).Scan(&assignedUserCount); err != nil {
		t.Fatalf("query user_keys: %v", err)
	}
	if assignedUserCount != 1 {
		t.Fatalf("expected 1 assigned user, got %d", assignedUserCount)
	}

	// 2. List Legacy Keys
	keys, err := repo.ListLegacy(ctx)
	if err != nil {
		t.Fatalf("ListLegacy failed: %v", err)
	}
	if len(keys) != 1 || keys[0].ID != keyID || keys[0].URL != vlessURI {
		t.Fatalf("ListLegacy returned unexpected key: %#v", keys)
	}
	if keys[0].URLShort == "" {
		t.Fatalf("expected populated URLShort")
	}

	// 3. Update Legacy Key
	updatedURI := "vless://22222222-2222-2222-2222-222222222222@example.com:8443?type=tcp#UpdatedStorage"
	err = repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{
		ID:           keyID,
		Label:        "Updated Storage Key",
		Status:       "active",
		Kind:         "real",
		Category:     "UpdatedCat",
		TemplateText: "",
		BuiltURL:     updatedURI,
	})
	if err != nil {
		t.Fatalf("UpdateLegacy failed: %v", err)
	}

	if err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&encURL); err != nil {
		t.Fatalf("query updated secret: %v", err)
	}
	decUpd, err := profilestorage.Decrypt(encURL, kr, keyID)
	if err != nil || decUpd.Reveal() != updatedURI {
		t.Fatalf("decryption of updated secret failed: err=%v, uri=%q", err, decUpd.Reveal())
	}

	// 4. Delete Legacy Key
	err = repo.DeleteLegacy(ctx, keyID)
	if err != nil {
		t.Fatalf("DeleteLegacy failed: %v", err)
	}

	var keyCount, secretCount, assignmentCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, keyID).Scan(&keyCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&secretCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE key_id = ?`, keyID).Scan(&assignmentCount)

	if keyCount != 0 || secretCount != 0 || assignmentCount != 0 {
		t.Fatalf("cleanup failed after delete: keys=%d, secrets=%d, assignments=%d", keyCount, secretCount, assignmentCount)
	}
}

func TestStorage_LegacyKeyUpdateOrdering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForStorage(t)
	repo := NewProfileRepository(db, kr)
	ctx := context.Background()

	id1, _ := repo.CreateLegacy(ctx, profilepersistence.CreateLegacyKeyParams{Label: "K1", Status: "active", Kind: "real", KeyURL: "vless://11111111-1111-1111-1111-111111111111@e.com:443#k1"})
	id2, _ := repo.CreateLegacy(ctx, profilepersistence.CreateLegacyKeyParams{Label: "K2", Status: "active", Kind: "real", KeyURL: "vless://22222222-2222-2222-2222-222222222222@e.com:443#k2"})
	id3, _ := repo.CreateLegacy(ctx, profilepersistence.CreateLegacyKeyParams{Label: "K3", Status: "active", Kind: "real", KeyURL: "vless://33333333-3333-3333-3333-333333333333@e.com:443#k3"})

	var s1, s2, s3 int64
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id1).Scan(&s1)
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id2).Scan(&s2)
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id3).Scan(&s3)
	if s1 != 1 || s2 != 2 || s3 != 3 {
		t.Fatalf("unexpected initial sort orders: s1=%d, s2=%d, s3=%d", s1, s2, s3)
	}

	// Update K2 without changing kind -> sort_order MUST stay 2
	err := repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{
		ID:       id2,
		Label:    "K2 Updated",
		Status:   "active",
		Kind:     "real",
		BuiltURL: "vless://22222222-2222-2222-2222-222222222222@e.com:443#k2_upd",
	})
	if err != nil {
		t.Fatalf("UpdateLegacy failed: %v", err)
	}

	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id1).Scan(&s1)
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id2).Scan(&s2)
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id3).Scan(&s3)
	if s1 != 1 || s2 != 2 || s3 != 3 {
		t.Fatalf("updating K2 altered sort orders unexpectedly: s1=%d, s2=%d, s3=%d", s1, s2, s3)
	}

	// Update K2 changing kind from 'real' to 'informational' -> sort_order MUST become MAX + 1 = 4
	err = repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{
		ID:           id2,
		Label:        "K2 Info",
		Status:       "active",
		Kind:         "informational",
		TemplateText: "Info Text",
		BuiltURL:     "info://abcdef123456",
	})
	if err != nil {
		t.Fatalf("UpdateLegacy kind change failed: %v", err)
	}

	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id1).Scan(&s1)
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id2).Scan(&s2)
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id3).Scan(&s3)
	if s1 != 1 || s2 != 4 || s3 != 3 {
		t.Fatalf("updating K2 kind failed to assign MAX+1: s1=%d, s2=%d (expected 4), s3=%d", s1, s2, s3)
	}
}

func TestStorage_LegacySourceOwnedAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForStorage(t)
	repo := NewProfileRepository(db, kr)
	ctx := context.Background()

	// Seed source-owned key directly in DB
	keyID, err := repo.CreateLegacy(ctx, profilepersistence.CreateLegacyKeyParams{
		Label:  "Ext Source Key",
		Status: "active",
		Kind:   "real",
		KeyURL: "vless://44444444-4444-4444-4444-444444444444@e.com:443#ext",
	})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if _, err := db.Exec(`UPDATE vless_keys SET external_source_id = 99 WHERE id = ?`, keyID); err != nil {
		t.Fatalf("set external_source_id: %v", err)
	}

	// Legacy update on source-owned key MUST succeed
	err = repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{
		ID:       keyID,
		Label:    "Updated Ext Source Key",
		Status:   "active",
		Kind:     "real",
		BuiltURL: "vless://44444444-4444-4444-4444-444444444444@e.com:443#ext_upd",
	})
	if err != nil {
		t.Fatalf("legacy update on source-owned key failed: %v", err)
	}

	// Legacy delete on source-owned key MUST succeed
	err = repo.DeleteLegacy(ctx, keyID)
	if err != nil {
		t.Fatalf("legacy delete on source-owned key failed: %v", err)
	}
}

func TestStorage_LegacySecretLifecycleTransitions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForStorage(t)
	repo := NewProfileRepository(db, kr)
	ctx := context.Background()

	realURL1 := "vless://11111111-1111-1111-1111-111111111111@e.com:443#r1"
	realURL2 := "vless://22222222-2222-2222-2222-222222222222@e.com:443#r2"
	infoURL1 := "info://abcdef123456"
	infoURL2 := "info://789012abcdef"

	keyID, _ := repo.CreateLegacy(ctx, profilepersistence.CreateLegacyKeyParams{Label: "K", Status: "active", Kind: "real", KeyURL: realURL1})

	// 1. real -> real
	err := repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "real", BuiltURL: realURL2})
	if err != nil {
		t.Fatalf("real -> real failed: %v", err)
	}
	_, decURI, err := repo.GetByID(ctx, keyID)
	if err != nil || decURI != realURL2 {
		t.Fatalf("real -> real URI mismatch: %q", decURI)
	}

	// 2. real -> informational
	err = repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "informational", TemplateText: "T1", BuiltURL: infoURL1})
	if err != nil {
		t.Fatalf("real -> info failed: %v", err)
	}
	_, decURI, err = repo.GetByID(ctx, keyID)
	if err != nil || decURI != infoURL1 {
		t.Fatalf("real -> info URI mismatch: %q", decURI)
	}

	// 3. informational -> real
	err = repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "real", BuiltURL: realURL1})
	if err != nil {
		t.Fatalf("info -> real failed: %v", err)
	}
	_, decURI, err = repo.GetByID(ctx, keyID)
	if err != nil || decURI != realURL1 {
		t.Fatalf("info -> real URI mismatch: %q", decURI)
	}

	// 4. informational -> informational
	err = repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "informational", TemplateText: "T2", BuiltURL: infoURL2})
	if err != nil {
		t.Fatalf("info -> info failed: %v", err)
	}
	_, decURI, err = repo.GetByID(ctx, keyID)
	if err != nil || decURI != infoURL2 {
		t.Fatalf("info -> info URI mismatch: %q", decURI)
	}

	// 5. Stale / missing secret row restoration
	if _, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, keyID); err != nil {
		t.Fatalf("delete secret row: %v", err)
	}
	err = repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{ID: keyID, Label: "K Restored", Status: "active", Kind: "real", BuiltURL: realURL1})
	if err != nil {
		t.Fatalf("update on missing secret row failed: %v", err)
	}
	_, decURI, err = repo.GetByID(ctx, keyID)
	if err != nil || decURI != realURL1 {
		t.Fatalf("restored secret URI mismatch: %q", decURI)
	}

	// 6. Corrupt encrypted secret overwriting
	if _, err := db.Exec(`UPDATE vless_key_secrets SET encrypted_url = 'corrupt' WHERE vless_key_id = ?`, keyID); err != nil {
		t.Fatalf("corrupt secret row: %v", err)
	}
	err = repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{ID: keyID, Label: "K Corrupt Fixed", Status: "active", Kind: "real", BuiltURL: realURL2})
	if err != nil {
		t.Fatalf("update on corrupt secret row failed: %v", err)
	}
	_, decURI, err = repo.GetByID(ctx, keyID)
	if err != nil || decURI != realURL2 {
		t.Fatalf("fixed secret URI mismatch: %q", decURI)
	}

	// 7. Failed update rollback
	err = repo.UpdateLegacy(ctx, profilepersistence.UpdateLegacyKeyParams{ID: 9999, Label: "Fail", Status: "active", Kind: "real", BuiltURL: realURL1})
	if !errors.Is(err, profilepersistence.ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound on 9999, got %v", err)
	}
}

func newTestKeyringForStorage(t *testing.T) *profilestorage.Keyring {
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
