package storage

import (
	"context"
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"strings"
	"testing"
)

func TestStorage_LegacySourceOwnedAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForStorage(t)
	repo := NewRepository(db, kr)
	ctx := context.Background()

	// Seed source-owned key directly in DB
	keyID, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
		Label:  "Ext Source Key",
		Status: "active",
		Kind:   "real",
		KeyURL: "vless://44444444-4444-4444-4444-444444444444@e.com:443#ext",
	})
	requireFixturef(t, err == nil, "create key: %v", err)

	{
		_, err := db.Exec(`UPDATE vless_keys SET external_source_id = 99 WHERE id = ?`, keyID)
		requireFixturef(t, err == nil, "set external_source_id: %v", err)
	}

	// Legacy update on source-owned key MUST succeed
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{
		ID:       keyID,
		Label:    "Updated Ext Source Key",
		Status:   "active",
		Kind:     "real",
		BuiltURL: "vless://44444444-4444-4444-4444-444444444444@e.com:443#ext_upd",
	})
	requireFixturef(t, err == nil, "legacy update on source-owned key failed: %v", err)

	// Legacy delete on source-owned key MUST succeed
	err = repo.DeleteLegacy(ctx, keyID)
	requireFixturef(t, err == nil, "legacy delete on source-owned key failed: %v", err)

}

func TestStorage_LegacySecretLifecycleTransitions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForStorage(t)
	repo := NewRepository(db, kr)
	ctx := context.Background()

	realURL1 := "vless://11111111-1111-1111-1111-111111111111@e.com:443#r1"
	realURL2 := "vless://22222222-2222-2222-2222-222222222222@e.com:443#r2"
	infoURL1 := "info://abcdef123456"
	infoURL2 := "info://789012abcdef"

	keyID, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "K", Status: "active", Kind: "real", KeyURL: realURL1})
	requireFixturef(t, err == nil, "fixture CreateLegacy: %v", err)

	// 1. real -> real
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "real", BuiltURL: realURL2})
	requireFixturef(t, err == nil, "real -> real failed: %v", err)

	_, decURI, err := repo.GetLegacyByID(ctx, keyID)
	requireFixturef(t, err == nil, "real -> real URI mismatch: %q", decURI)
	requireFixturef(t, decURI == realURL2, "real -> real URI mismatch: %q", decURI)

	// 2. real -> informational
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "informational", TemplateText: "T1", BuiltURL: infoURL1})
	requireFixturef(t, err == nil, "real -> info failed: %v", err)

	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	requireFixturef(t, err == nil, "real -> info URI mismatch: %q", decURI)
	requireFixturef(t, decURI == infoURL1, "real -> info URI mismatch: %q", decURI)

	// 3. informational -> real
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "real", BuiltURL: realURL1})
	requireFixturef(t, err == nil, "info -> real failed: %v", err)

	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	requireFixturef(t, err == nil, "info -> real URI mismatch: %q", decURI)
	requireFixturef(t, decURI == realURL1, "info -> real URI mismatch: %q", decURI)

	// 4. informational -> informational
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "informational", TemplateText: "T2", BuiltURL: infoURL2})
	requireFixturef(t, err == nil, "info -> info failed: %v", err)

	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	requireFixturef(t, err == nil, "info -> info URI mismatch: %q", decURI)
	requireFixturef(t, decURI == infoURL2, "info -> info URI mismatch: %q", decURI)

	// 5. Stale / missing secret row restoration
	{
		_, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, keyID)
		requireFixturef(t, err == nil, "delete secret row: %v", err)
	}

	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K Restored", Status: "active", Kind: "real", BuiltURL: realURL1})
	requireFixturef(t, err == nil, "update on missing secret row failed: %v", err)

	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	requireFixturef(t, err == nil, "restored secret URI mismatch: %q", decURI)
	requireFixturef(t, decURI == realURL1, "restored secret URI mismatch: %q", decURI)

	// 6. Corrupt encrypted secret overwriting
	{
		_, err := db.Exec(`UPDATE vless_key_secrets SET encrypted_url = 'corrupt' WHERE vless_key_id = ?`, keyID)
		requireFixturef(t, err == nil, "corrupt secret row: %v", err)
	}

	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K Corrupt Fixed", Status: "active", Kind: "real", BuiltURL: realURL2})
	requireFixturef(t, err == nil, "update on corrupt secret row failed: %v", err)

	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	requireFixturef(t, err == nil, "fixed secret URI mismatch: %q", decURI)
	requireFixturef(t, decURI == realURL2, "fixed secret URI mismatch: %q", decURI)

	// 7. Failed update rollback
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: 9999, Label: "Fail", Status: "active", Kind: "real", BuiltURL: realURL1})
	requireFixturef(t, errors.Is(err, keymanagement.ErrKeyNotFound), "expected ErrProfileNotFound on 9999, got %v", err)

}

func TestKeyRepository_CredentialsFailClosedAndHealthBatchAccountsForUnreadable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForStorage(t)
	repo := NewRepository(db, kr)
	ctx := context.Background()
	one, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "One", Status: "active", Kind: "real", KeyURL: "vless://ffffffff-ffff-ffff-ffff-ffffffffffff@example.com:443#one"})
	requireFixturef(t, err == nil, "create one: %v", err)

	two, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "Two", Status: "active", Kind: "real", KeyURL: "vless://11111111-2222-3333-4444-555555555555@example.com:443#two"})
	requireFixturef(t, err == nil, "create two: %v", err)

	var envelopeOne, envelopeTwo string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, one).Scan(&envelopeOne)
		requireFixturef(t, err == nil, "load envelope one: %v", err)
	}

	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, two).Scan(&envelopeTwo)
		requireFixturef(t, err == nil, "load envelope two: %v", err)
	}

	requireFixture(t, !(strings.Contains(envelopeOne, "ffffffff-ffff")), "plaintext credential leaked into envelope")

	{
		_, err := db.Exec(`UPDATE vless_key_secrets SET encrypted_url = ? WHERE vless_key_id = ?`, envelopeOne, two)
		requireFixturef(t, err == nil, "swap AAD-bound envelope: %v", err)
	}

	{
		_, _, err := repo.GetLegacyByID(ctx, two)
		requireFixturef(t, errors.Is(err, keymanagement.ErrStorageIntegrity), "row-bound envelope must fail closed: %v", err)
	}

	{
		_, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, one)
		requireFixturef(t, err == nil, "delete envelope: %v", err)
	}

	{
		_, _, err := repo.GetLegacyByID(ctx, one)
		requireFixturef(t, errors.Is(err, keymanagement.ErrCredentialMissing), "missing envelope must retain missing/integrity identities: %v", err)
		requireFixturef(t, errors.Is(err, keymanagement.ErrStorageIntegrity), "missing envelope must retain missing/integrity identities: %v", err)
	}

	{
		_, err := repo.ListLegacy(ctx)
		requireFixturef(t, errors.Is(err, keymanagement.ErrStorageIntegrity), "legacy list must fail closed: %v", err)
	}

	targets, err := repo.ListHealthCheckTargets(ctx)
	requireFixturef(t, err == nil, "list health targets: %v", err)

	requireFixturef(t, len(targets) == 2, "health batch did not account for unreadable targets: %#v", targets)

	for _, target := range targets {
		requireFixturef(t, target.Unreadable, "unreadable health target exposed credentials or lost its marker: %#v", target)
		requireFixturef(t, target.URL == "", "unreadable health target exposed credentials or lost its marker: %#v", target)

	}
	_ = envelopeTwo
}

func TestKeyRepository_RetiredKeyReadsAndActiveKeyWrites(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	oldKey := []byte("01234567890123456789012345678901")
	newKey := []byte("98765432109876543210987654321098")
	bik := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345")
	oldRing := &profilestorage.Keyring{
		ActiveKeyID: "old", Keys: map[string][]byte{"old": oldKey},
		ActiveBlindIndexKeyID: "bik", BlindIndexKeys: map[string][]byte{"bik": bik},
	}
	oldRepo := NewRepository(db, oldRing)
	ctx := context.Background()
	oldID, err := oldRepo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "Old", Status: "active", Kind: "real", KeyURL: "vless://12121212-1212-1212-1212-121212121212@example.com:443#old"})
	requireFixturef(t, err == nil, "create old envelope: %v", err)

	rotatedRing := &profilestorage.Keyring{
		ActiveKeyID: "new", Keys: map[string][]byte{"old": oldKey, "new": newKey},
		ActiveBlindIndexKeyID: "bik", BlindIndexKeys: map[string][]byte{"bik": bik},
	}
	rotatedRepo := NewRepository(db, rotatedRing)
	{
		_, raw, err := rotatedRepo.GetLegacyByID(ctx, oldID)
		requireFixturef(t, err == nil, "retired key could not decrypt existing envelope: raw=%q err=%v", raw, err)
		requireFixturef(t, strings.HasSuffix(raw, "#old"), "retired key could not decrypt existing envelope: raw=%q err=%v", raw, err)
	}

	newID, err := rotatedRepo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "New", Status: "active", Kind: "real", KeyURL: "vless://34343434-3434-3434-3434-343434343434@example.com:443#new"})
	requireFixturef(t, err == nil, "create with active key: %v", err)

	var envelope string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, newID).Scan(&envelope)
		requireFixturef(t, err == nil, "load new envelope: %v", err)
	}

	requireFixturef(t, strings.Contains(envelope, "$new$"), "new write did not use active key: %q", envelope)

}

func TestKeyRepository_CreateRollbackAfterParentWrite(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	{
		_, err := db.Exec(`CREATE TRIGGER reject_secret BEFORE INSERT ON vless_key_secrets BEGIN SELECT RAISE(ABORT, 'forced secret failure'); END`)
		requireFixturef(t, err == nil, "create trigger: %v", err)
	}

	repo := NewRepository(db, newTestKeyringForStorage(t))
	_, err := repo.CreateLegacy(context.Background(), keymanagement.CreateLegacyKeyParams{
		Label: "Rollback", Status: "active", Kind: "real", Category: "RollbackCategory",
		KeyURL: "vless://56565656-5656-5656-5656-565656565656@example.com:443#rollback",
	})
	requireFixture(t, err != nil, "expected forced secret write failure")

	var keys, categories int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE label = 'Rollback'`).Scan(&keys)
		requireFixturef(t, err == nil, "count keys: %v", err)
	}

	{
		err := db.QueryRow(`SELECT COUNT(*) FROM key_categories WHERE name = 'RollbackCategory'`).Scan(&categories)
		requireFixturef(t, err == nil, "count categories: %v", err)
	}

	requireFixturef(t, keys == 0, "transaction did not roll back parent metadata: keys=%d categories=%d", keys, categories)
	requireFixturef(t, categories == 0, "transaction did not roll back parent metadata: keys=%d categories=%d", keys, categories)

}
