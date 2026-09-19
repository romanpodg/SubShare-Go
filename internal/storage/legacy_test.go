package storage

import (
	"context"
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func TestStorage_LegacyKeyCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO users(id, key_assignment_mode) VALUES(1, 'all'), (2, 'manual')`); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	kr := newTestKeyringForStorage(t)
	repo := NewRepository(db, kr)

	ctx := context.Background()
	vlessURI := "vless://11111111-1111-1111-1111-111111111111@example.com:443?type=tcp#LegacyStorage"

	// 1. Create Legacy Key
	keyID, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
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
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{
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
	repo := NewRepository(db, kr)
	ctx := context.Background()

	id1, _ := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "K1", Status: "active", Kind: "real", KeyURL: "vless://11111111-1111-1111-1111-111111111111@e.com:443#k1"})
	id2, _ := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "K2", Status: "active", Kind: "real", KeyURL: "vless://22222222-2222-2222-2222-222222222222@e.com:443#k2"})
	id3, _ := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "K3", Status: "active", Kind: "real", KeyURL: "vless://33333333-3333-3333-3333-333333333333@e.com:443#k3"})

	var s1, s2, s3 int64
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id1).Scan(&s1)
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id2).Scan(&s2)
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id3).Scan(&s3)
	if s1 != 1 || s2 != 2 || s3 != 3 {
		t.Fatalf("unexpected initial sort orders: s1=%d, s2=%d, s3=%d", s1, s2, s3)
	}

	// Update K2 without changing kind -> sort_order MUST stay 2
	err := repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{
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
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{
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
	repo := NewRepository(db, kr)
	ctx := context.Background()

	// Seed source-owned key directly in DB
	keyID, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
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
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{
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
	repo := NewRepository(db, kr)
	ctx := context.Background()

	realURL1 := "vless://11111111-1111-1111-1111-111111111111@e.com:443#r1"
	realURL2 := "vless://22222222-2222-2222-2222-222222222222@e.com:443#r2"
	infoURL1 := "info://abcdef123456"
	infoURL2 := "info://789012abcdef"

	keyID, _ := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "K", Status: "active", Kind: "real", KeyURL: realURL1})

	// 1. real -> real
	err := repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "real", BuiltURL: realURL2})
	if err != nil {
		t.Fatalf("real -> real failed: %v", err)
	}
	_, decURI, err := repo.GetLegacyByID(ctx, keyID)
	if err != nil || decURI != realURL2 {
		t.Fatalf("real -> real URI mismatch: %q", decURI)
	}

	// 2. real -> informational
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "informational", TemplateText: "T1", BuiltURL: infoURL1})
	if err != nil {
		t.Fatalf("real -> info failed: %v", err)
	}
	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	if err != nil || decURI != infoURL1 {
		t.Fatalf("real -> info URI mismatch: %q", decURI)
	}

	// 3. informational -> real
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "real", BuiltURL: realURL1})
	if err != nil {
		t.Fatalf("info -> real failed: %v", err)
	}
	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	if err != nil || decURI != realURL1 {
		t.Fatalf("info -> real URI mismatch: %q", decURI)
	}

	// 4. informational -> informational
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K", Status: "active", Kind: "informational", TemplateText: "T2", BuiltURL: infoURL2})
	if err != nil {
		t.Fatalf("info -> info failed: %v", err)
	}
	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	if err != nil || decURI != infoURL2 {
		t.Fatalf("info -> info URI mismatch: %q", decURI)
	}

	// 5. Stale / missing secret row restoration
	if _, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, keyID); err != nil {
		t.Fatalf("delete secret row: %v", err)
	}
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K Restored", Status: "active", Kind: "real", BuiltURL: realURL1})
	if err != nil {
		t.Fatalf("update on missing secret row failed: %v", err)
	}
	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	if err != nil || decURI != realURL1 {
		t.Fatalf("restored secret URI mismatch: %q", decURI)
	}

	// 6. Corrupt encrypted secret overwriting
	if _, err := db.Exec(`UPDATE vless_key_secrets SET encrypted_url = 'corrupt' WHERE vless_key_id = ?`, keyID); err != nil {
		t.Fatalf("corrupt secret row: %v", err)
	}
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: keyID, Label: "K Corrupt Fixed", Status: "active", Kind: "real", BuiltURL: realURL2})
	if err != nil {
		t.Fatalf("update on corrupt secret row failed: %v", err)
	}
	_, decURI, err = repo.GetLegacyByID(ctx, keyID)
	if err != nil || decURI != realURL2 {
		t.Fatalf("fixed secret URI mismatch: %q", decURI)
	}

	// 7. Failed update rollback
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{ID: 9999, Label: "Fail", Status: "active", Kind: "real", BuiltURL: realURL1})
	if !errors.Is(err, keymanagement.ErrKeyNotFound) {
		t.Fatalf("expected ErrProfileNotFound on 9999, got %v", err)
	}
}

func TestStorage_CategoryManagementAndKeyReordering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForStorage(t)
	repo := NewRepository(db, kr)
	ctx := context.Background()

	// 1. Create Categories
	cat1, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "CatA", Color: "#FF0000"})
	if err != nil || cat1.Name != "CatA" || cat1.Color != "#FF0000" {
		t.Fatalf("CreateKeyCategory CatA failed: err=%v, cat=%#v", err, cat1)
	}
	cat2, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "CatB", Color: "#00FF00"})
	if err != nil || cat2.Name != "CatB" || cat2.Color != "#00FF00" {
		t.Fatalf("CreateKeyCategory CatB failed: err=%v, cat=%#v", err, cat2)
	}

	// Create key assigned to CatA
	k1, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
		Label:    "KeyA",
		Status:   "active",
		Kind:     "real",
		KeyURL:   "vless://11111111-1111-1111-1111-111111111111@e.com:443#a",
		Category: "CatA",
	})
	if err != nil {
		t.Fatalf("CreateLegacy failed: %v", err)
	}
	k2, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
		Label:    "KeyB",
		Status:   "active",
		Kind:     "real",
		KeyURL:   "vless://22222222-2222-2222-2222-222222222222@e.com:443#b",
		Category: "CatB",
	})
	if err != nil {
		t.Fatalf("CreateLegacy failed: %v", err)
	}

	// 2. List Key Categories
	cats, err := repo.ListKeyCategories(ctx)
	if err != nil || len(cats) != 2 {
		t.Fatalf("ListKeyCategories failed: err=%v, cats=%#v", err, cats)
	}

	// 3. Update Category (Rename CatA -> CatARenamed)
	cat1Upd, err := repo.UpdateKeyCategory(ctx, keymanagement.UpdateCategoryParams{
		OldName: "CatA",
		NewName: "CatARenamed",
		Color:   "#0000FF",
	})
	if err != nil || cat1Upd.Name != "CatARenamed" {
		t.Fatalf("UpdateKeyCategory failed: %v", err)
	}
	// Verify key1 updated category to CatARenamed
	var key1Cat string
	_ = db.QueryRow(`SELECT category FROM vless_keys WHERE id = ?`, k1).Scan(&key1Cat)
	if key1Cat != "CatARenamed" {
		t.Fatalf("key1 category not updated: %q", key1Cat)
	}

	// 4. Reorder Key Categories
	err = repo.ReorderKeyCategories(ctx, []string{"CatB", "CatARenamed"})
	if err != nil {
		t.Fatalf("ReorderKeyCategories failed: %v", err)
	}

	// 5. Reorder Keys
	err = repo.ReorderKeys(ctx, []int64{k2, k1})
	if err != nil {
		t.Fatalf("ReorderKeys failed: %v", err)
	}
	var s1, s2 int64
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, k1).Scan(&s1)
	_ = db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, k2).Scan(&s2)
	if s1 != 2 || s2 != 1 {
		t.Fatalf("ReorderKeys incorrect sort_orders: s1=%d, s2=%d", s1, s2)
	}

	// 6. Delete Category with keep_keys
	err = repo.DeleteKeyCategory(ctx, keymanagement.DeleteCategoryParams{
		Name: "CatARenamed",
		Mode: "keep_keys",
	})
	if err != nil {
		t.Fatalf("DeleteKeyCategory keep_keys failed: %v", err)
	}
	var k1Exist int
	_ = db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, k1).Scan(&k1Exist)
	if k1Exist != 1 {
		t.Fatalf("k1 was unexpectedly deleted in keep_keys mode")
	}

	// 7. Delete Category with delete_with_keys
	err = repo.DeleteKeyCategory(ctx, keymanagement.DeleteCategoryParams{
		Name: "CatB",
		Mode: "delete_with_keys",
	})
	if err != nil {
		t.Fatalf("DeleteKeyCategory delete_with_keys failed: %v", err)
	}
	var k2Exist int
	_ = db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, k2).Scan(&k2Exist)
	if k2Exist != 0 {
		t.Fatalf("k2 was not deleted in delete_with_keys mode")
	}
}

func TestKeyRepository_CategoryCollisionsReferencesAndOrdering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db, newTestKeyringForStorage(t))
	ctx := context.Background()

	source, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "Source", Color: "#111111"})
	if err != nil {
		t.Fatalf("create source category: %v", err)
	}
	duplicate, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "Source", Color: "#222222"})
	if err != nil {
		t.Fatalf("upsert duplicate category: %v", err)
	}
	if duplicate.ID != source.ID || duplicate.Color != "#222222" {
		t.Fatalf("duplicate category did not update in place: source=%#v duplicate=%#v", source, duplicate)
	}
	var sourceCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM key_categories WHERE name = 'Source'`).Scan(&sourceCount); err != nil || sourceCount != 1 {
		t.Fatalf("duplicate category row count = %d, err=%v", sourceCount, err)
	}

	target, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "Target", Color: "#333333"})
	if err != nil {
		t.Fatalf("create target category: %v", err)
	}
	localID, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
		Label: "Local", Status: "active", Kind: "real", Category: "Source",
		KeyURL: "vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@example.com:443#local",
	})
	if err != nil {
		t.Fatalf("create local key: %v", err)
	}
	importedID, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
		Label: "Imported", Status: "active", Kind: "real", Category: "Source",
		KeyURL: "vless://bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb@example.com:443#imported",
	})
	if err != nil {
		t.Fatalf("create imported key: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO external_subscription_sources(id, name, key_category_id, key_category) VALUES(99, 'Feed', ?, 'Source')`, source.ID); err != nil {
		t.Fatalf("seed external source: %v", err)
	}
	if _, err := db.Exec(`UPDATE vless_keys SET external_source_id = 99 WHERE id = ?`, importedID); err != nil {
		t.Fatalf("mark imported key: %v", err)
	}

	merged, err := repo.UpdateKeyCategory(ctx, keymanagement.UpdateCategoryParams{
		OldName: "Source", NewName: "Target", Color: "#444444",
	})
	if err != nil {
		t.Fatalf("merge category rename: %v", err)
	}
	if merged.ID != target.ID || merged.Name != "Target" || merged.Color != "#444444" {
		t.Fatalf("unexpected merged category: %#v", merged)
	}
	for _, id := range []int64{localID, importedID} {
		var categoryID int64
		var category string
		if err := db.QueryRow(`SELECT category_id, category FROM vless_keys WHERE id = ?`, id).Scan(&categoryID, &category); err != nil {
			t.Fatalf("load key %d category: %v", id, err)
		}
		if categoryID != target.ID || category != "Target" {
			t.Fatalf("key %d category = (%d, %q), want (%d, Target)", id, categoryID, category, target.ID)
		}
	}
	var sourceCategoryID int64
	var sourceCategory string
	if err := db.QueryRow(`SELECT key_category_id, key_category FROM external_subscription_sources WHERE id = 99`).Scan(&sourceCategoryID, &sourceCategory); err != nil {
		t.Fatalf("load source category reference: %v", err)
	}
	if sourceCategoryID != target.ID || sourceCategory != "Target" {
		t.Fatalf("source reference = (%d, %q), want (%d, Target)", sourceCategoryID, sourceCategory, target.ID)
	}
	if err := repo.ReorderKeyCategories(ctx, []string{"Target", "Unknown"}); err != nil {
		t.Fatalf("partial/unknown category reorder must remain accepted: %v", err)
	}

	if err := repo.ReorderKeys(ctx, []int64{localID}); !errors.Is(err, keymanagement.ErrInvalidKeyOrderCount) {
		t.Fatalf("partial key order error = %v", err)
	}
	if err := repo.ReorderKeys(ctx, []int64{localID, 999}); !errors.Is(err, keymanagement.ErrUnknownKeyInOrder) {
		t.Fatalf("unknown key order error = %v", err)
	}
	if err := repo.ReorderKeys(ctx, []int64{localID, localID}); !errors.Is(err, keymanagement.ErrDuplicateKeyInOrder) {
		t.Fatalf("duplicate key order error = %v", err)
	}
	if err := repo.ReorderKeys(ctx, []int64{importedID, localID}); err != nil {
		t.Fatalf("complete key order: %v", err)
	}
}

func TestKeyRepository_CategoryDeletionModesIncludeSourceOwnedKeys(t *testing.T) {
	newFixture := func(t *testing.T, category string) (*Repository, int64, int64) {
		t.Helper()
		db := setupTestDB(t)
		t.Cleanup(func() { _ = db.Close() })
		repo := NewRepository(db, newTestKeyringForStorage(t))
		cat, err := repo.CreateKeyCategory(context.Background(), keymanagement.CreateCategoryParams{Name: category, Color: "#123456"})
		if err != nil {
			t.Fatalf("create category: %v", err)
		}
		id, err := repo.CreateLegacy(context.Background(), keymanagement.CreateLegacyKeyParams{
			Label: category, Status: "active", Kind: "real", Category: category,
			KeyURL: "vless://cccccccc-cccc-cccc-cccc-cccccccccccc@example.com:443#owned",
		})
		if err != nil {
			t.Fatalf("create key: %v", err)
		}
		if _, err := db.Exec(`UPDATE vless_keys SET external_source_id = 77 WHERE id = ?`, id); err != nil {
			t.Fatalf("mark source-owned: %v", err)
		}
		return repo, cat.ID, id
	}

	t.Run("keep_keys", func(t *testing.T) {
		repo, _, id := newFixture(t, "Keep")
		if err := repo.DeleteKeyCategory(context.Background(), keymanagement.DeleteCategoryParams{Name: "Keep", Mode: "keep_keys"}); err != nil {
			t.Fatalf("delete category keep_keys: %v", err)
		}
		var categoryID *int64
		var category string
		if err := repo.db.QueryRow(`SELECT category_id, category FROM vless_keys WHERE id = ?`, id).Scan(&categoryID, &category); err != nil {
			t.Fatalf("source-owned key was deleted: %v", err)
		}
		if categoryID != nil || category != "" {
			t.Fatalf("category was not cleared: id=%v name=%q", categoryID, category)
		}
	})

	t.Run("delete_with_keys", func(t *testing.T) {
		repo, _, id := newFixture(t, "Delete")
		if err := repo.DeleteKeyCategory(context.Background(), keymanagement.DeleteCategoryParams{Name: "Delete", Mode: "delete_with_keys"}); err != nil {
			t.Fatalf("delete category and keys: %v", err)
		}
		var count int
		if err := repo.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, id).Scan(&count); err != nil || count != 0 {
			t.Fatalf("source-owned key count = %d, err=%v", count, err)
		}
	})
}

func TestKeyRepository_BulkMutationsRollbackOnMissingKey(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db, newTestKeyringForStorage(t))
	ctx := context.Background()
	first, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "First", Status: "active", Kind: "real", KeyURL: "vless://dddddddd-dddd-dddd-dddd-dddddddddddd@example.com:443#first"})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "Second", Status: "active", Kind: "real", KeyURL: "vless://eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee@example.com:443#second"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	err = repo.BulkUpdateKeys(ctx, keymanagement.BulkKeyUpdate{IDs: []int64{first, 999, second}, Status: "disabled"})
	var missing keymanagement.KeyNotFoundError
	if !errors.As(err, &missing) || missing.ID != 999 {
		t.Fatalf("bulk update error = %v", err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM vless_keys WHERE id = ?`, first).Scan(&status); err != nil || status != "active" {
		t.Fatalf("bulk update did not roll back first row: status=%q err=%v", status, err)
	}

	err = repo.BulkDeleteKeys(ctx, []int64{first, 999, second})
	if !errors.As(err, &missing) || missing.ID != 999 {
		t.Fatalf("bulk delete error = %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id IN (?, ?)`, first, second).Scan(&count); err != nil || count != 2 {
		t.Fatalf("bulk delete did not roll back: count=%d err=%v", count, err)
	}
}

func TestKeyRepository_CredentialsFailClosedAndHealthBatchAccountsForUnreadable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForStorage(t)
	repo := NewRepository(db, kr)
	ctx := context.Background()
	one, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "One", Status: "active", Kind: "real", KeyURL: "vless://ffffffff-ffff-ffff-ffff-ffffffffffff@example.com:443#one"})
	if err != nil {
		t.Fatalf("create one: %v", err)
	}
	two, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "Two", Status: "active", Kind: "real", KeyURL: "vless://11111111-2222-3333-4444-555555555555@example.com:443#two"})
	if err != nil {
		t.Fatalf("create two: %v", err)
	}
	var envelopeOne, envelopeTwo string
	if err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, one).Scan(&envelopeOne); err != nil {
		t.Fatalf("load envelope one: %v", err)
	}
	if err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, two).Scan(&envelopeTwo); err != nil {
		t.Fatalf("load envelope two: %v", err)
	}
	if strings.Contains(envelopeOne, "ffffffff-ffff") {
		t.Fatal("plaintext credential leaked into envelope")
	}
	if _, err := db.Exec(`UPDATE vless_key_secrets SET encrypted_url = ? WHERE vless_key_id = ?`, envelopeOne, two); err != nil {
		t.Fatalf("swap AAD-bound envelope: %v", err)
	}
	if _, _, err := repo.GetLegacyByID(ctx, two); !errors.Is(err, keymanagement.ErrStorageIntegrity) {
		t.Fatalf("row-bound envelope must fail closed: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, one); err != nil {
		t.Fatalf("delete envelope: %v", err)
	}
	if _, _, err := repo.GetLegacyByID(ctx, one); !errors.Is(err, keymanagement.ErrCredentialMissing) || !errors.Is(err, keymanagement.ErrStorageIntegrity) {
		t.Fatalf("missing envelope must retain missing/integrity identities: %v", err)
	}
	if _, err := repo.ListLegacy(ctx); !errors.Is(err, keymanagement.ErrStorageIntegrity) {
		t.Fatalf("legacy list must fail closed: %v", err)
	}
	targets, err := repo.ListHealthCheckTargets(ctx)
	if err != nil {
		t.Fatalf("list health targets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("health batch did not account for unreadable targets: %#v", targets)
	}
	for _, target := range targets {
		if !target.Unreadable || target.URL != "" {
			t.Fatalf("unreadable health target exposed credentials or lost its marker: %#v", target)
		}
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
	if err != nil {
		t.Fatalf("create old envelope: %v", err)
	}
	rotatedRing := &profilestorage.Keyring{
		ActiveKeyID: "new", Keys: map[string][]byte{"old": oldKey, "new": newKey},
		ActiveBlindIndexKeyID: "bik", BlindIndexKeys: map[string][]byte{"bik": bik},
	}
	rotatedRepo := NewRepository(db, rotatedRing)
	if _, raw, err := rotatedRepo.GetLegacyByID(ctx, oldID); err != nil || !strings.HasSuffix(raw, "#old") {
		t.Fatalf("retired key could not decrypt existing envelope: raw=%q err=%v", raw, err)
	}
	newID, err := rotatedRepo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "New", Status: "active", Kind: "real", KeyURL: "vless://34343434-3434-3434-3434-343434343434@example.com:443#new"})
	if err != nil {
		t.Fatalf("create with active key: %v", err)
	}
	var envelope string
	if err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, newID).Scan(&envelope); err != nil {
		t.Fatalf("load new envelope: %v", err)
	}
	if !strings.Contains(envelope, "$new$") {
		t.Fatalf("new write did not use active key: %q", envelope)
	}
}

func TestKeyRepository_CreateRollbackAfterParentWrite(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER reject_secret BEFORE INSERT ON vless_key_secrets BEGIN SELECT RAISE(ABORT, 'forced secret failure'); END`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	repo := NewRepository(db, newTestKeyringForStorage(t))
	_, err := repo.CreateLegacy(context.Background(), keymanagement.CreateLegacyKeyParams{
		Label: "Rollback", Status: "active", Kind: "real", Category: "RollbackCategory",
		KeyURL: "vless://56565656-5656-5656-5656-565656565656@example.com:443#rollback",
	})
	if err == nil {
		t.Fatal("expected forced secret write failure")
	}
	var keys, categories int
	if err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE label = 'Rollback'`).Scan(&keys); err != nil {
		t.Fatalf("count keys: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM key_categories WHERE name = 'RollbackCategory'`).Scan(&categories); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if keys != 0 || categories != 0 {
		t.Fatalf("transaction did not roll back parent metadata: keys=%d categories=%d", keys, categories)
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
