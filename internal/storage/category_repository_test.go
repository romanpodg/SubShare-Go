package storage

import (
	"context"
	"database/sql"
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"testing"
)

func TestStorage_CategoryManagementAndKeyReordering(t *testing.T) {
	fixture := createCategoryOrderingFixture(t)
	checkCategoryRenameAndOrdering(t, fixture)
	checkCategoryDeletionModes(t, fixture)
}

type categoryOrderingFixture struct {
	db            *sql.DB
	repo          *Repository
	ctx           context.Context
	first, second int64
}

func createCategoryOrderingFixture(t *testing.T) categoryOrderingFixture {
	t.Helper()

	db := setupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	kr := newTestKeyringForStorage(t)
	repo := NewRepository(db, kr)
	ctx := context.Background()

	// 1. Create Categories
	cat1, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "CatA", Color: "#FF0000"})
	requireFixturef(t, err == nil, "CreateKeyCategory CatA failed: err=%v, cat=%#v", err, cat1)
	requireFixturef(t, cat1.Name == "CatA", "CreateKeyCategory CatA failed: err=%v, cat=%#v", err, cat1)
	requireFixturef(t, cat1.Color == "#FF0000", "CreateKeyCategory CatA failed: err=%v, cat=%#v", err, cat1)

	cat2, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "CatB", Color: "#00FF00"})
	requireFixturef(t, err == nil, "CreateKeyCategory CatB failed: err=%v, cat=%#v", err, cat2)
	requireFixturef(t, cat2.Name == "CatB", "CreateKeyCategory CatB failed: err=%v, cat=%#v", err, cat2)
	requireFixturef(t, cat2.Color == "#00FF00", "CreateKeyCategory CatB failed: err=%v, cat=%#v", err, cat2)

	// Create key assigned to CatA
	k1, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
		Label:    "KeyA",
		Status:   "active",
		Kind:     "real",
		KeyURL:   "vless://11111111-1111-1111-1111-111111111111@e.com:443#a",
		Category: "CatA",
	})
	requireFixturef(t, err == nil, "CreateLegacy failed: %v", err)

	k2, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
		Label:    "KeyB",
		Status:   "active",
		Kind:     "real",
		KeyURL:   "vless://22222222-2222-2222-2222-222222222222@e.com:443#b",
		Category: "CatB",
	})
	requireFixturef(t, err == nil, "CreateLegacy failed: %v", err)

	return categoryOrderingFixture{db: db, repo: repo, ctx: ctx, first: k1, second: k2}
}
func checkCategoryRenameAndOrdering(t *testing.T, fixture categoryOrderingFixture) {
	t.Helper()
	ctx, db, repo, k1, k2 := fixture.ctx, fixture.db, fixture.repo, fixture.first, fixture.second
	// 2. List Key Categories
	cats, err := repo.ListKeyCategories(ctx)
	requireFixturef(t, err == nil, "ListKeyCategories failed: err=%v, cats=%#v", err, cats)
	requireFixturef(t, len(cats) == 2, "ListKeyCategories failed: err=%v, cats=%#v", err, cats)

	// 3. Update Category (Rename CatA -> CatARenamed)
	cat1Upd, err := repo.UpdateKeyCategory(ctx, keymanagement.UpdateCategoryParams{
		OldName: "CatA",
		NewName: "CatARenamed",
		Color:   "#0000FF",
	})
	requireFixturef(t, err == nil, "UpdateKeyCategory failed: %v", err)
	requireFixturef(t, cat1Upd.Name == "CatARenamed", "UpdateKeyCategory failed: %v", err)

	// Verify key1 updated category to CatARenamed
	var key1Cat string
	{
		err := db.QueryRow(`SELECT category FROM vless_keys WHERE id = ?`, k1).Scan(&key1Cat)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	requireFixturef(t, key1Cat == "CatARenamed", "key1 category not updated: %q", key1Cat)

	// 4. Reorder Key Categories
	err = repo.ReorderKeyCategories(ctx, []string{"CatB", "CatARenamed"})
	requireFixturef(t, err == nil, "ReorderKeyCategories failed: %v", err)

	// 5. Reorder Keys
	err = repo.ReorderKeys(ctx, []int64{k2, k1})
	requireFixturef(t, err == nil, "ReorderKeys failed: %v", err)

	var s1, s2 int64
	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, k1).Scan(&s1)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, k2).Scan(&s2)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	requireFixturef(t, s1 == 2, "ReorderKeys incorrect sort_orders: s1=%d, s2=%d", s1, s2)
	requireFixturef(t, s2 == 1, "ReorderKeys incorrect sort_orders: s1=%d, s2=%d", s1, s2)

}
func checkCategoryDeletionModes(t *testing.T, fixture categoryOrderingFixture) {
	t.Helper()
	ctx, db, repo, k1, k2 := fixture.ctx, fixture.db, fixture.repo, fixture.first, fixture.second
	// 6. Delete Category with keep_keys
	err := repo.DeleteKeyCategory(ctx, keymanagement.DeleteCategoryParams{
		Name: "CatARenamed",
		Mode: "keep_keys",
	})
	requireFixturef(t, err == nil, "DeleteKeyCategory keep_keys failed: %v", err)

	var k1Exist int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, k1).Scan(&k1Exist)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	requireFixturef(t, k1Exist == 1, "k1 was unexpectedly deleted in keep_keys mode")

	// 7. Delete Category with delete_with_keys
	err = repo.DeleteKeyCategory(ctx, keymanagement.DeleteCategoryParams{
		Name: "CatB",
		Mode: "delete_with_keys",
	})
	requireFixturef(t, err == nil, "DeleteKeyCategory delete_with_keys failed: %v", err)

	var k2Exist int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, k2).Scan(&k2Exist)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	requireFixturef(t, k2Exist == 0, "k2 was not deleted in delete_with_keys mode")

}
func TestKeyRepository_CategoryCollisionsReferencesAndOrdering(t *testing.T) {
	fixture := createCategoryCollisionFixture(t)
	checkCategoryMergeReferences(t, fixture)
	checkCategoryReorderValidation(t, fixture)
}

type categoryCollisionFixture struct {
	db                            *sql.DB
	repo                          *Repository
	ctx                           context.Context
	targetID, localID, importedID int64
}

func createCategoryCollisionFixture(t *testing.T) categoryCollisionFixture {
	t.Helper()

	db := setupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewRepository(db, newTestKeyringForStorage(t))
	ctx := context.Background()

	source, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "Source", Color: "#111111"})
	requireFixturef(t, err == nil, "create source category: %v", err)

	duplicate, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "Source", Color: "#222222"})
	requireFixturef(t, err == nil, "upsert duplicate category: %v", err)

	requireFixturef(t, duplicate.ID == source.ID, "duplicate category did not update in place: source=%#v duplicate=%#v", source, duplicate)
	requireFixturef(t, duplicate.Color == "#222222", "duplicate category did not update in place: source=%#v duplicate=%#v", source, duplicate)

	var sourceCount int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM key_categories WHERE name = 'Source'`).Scan(&sourceCount)
		requireFixturef(t, err == nil, "duplicate category row count = %d, err=%v", sourceCount, err)
		requireFixturef(t, sourceCount == 1, "duplicate category row count = %d, err=%v", sourceCount, err)
	}

	target, err := repo.CreateKeyCategory(ctx, keymanagement.CreateCategoryParams{Name: "Target", Color: "#333333"})
	requireFixturef(t, err == nil, "create target category: %v", err)

	localID, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
		Label: "Local", Status: "active", Kind: "real", Category: "Source",
		KeyURL: "vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@example.com:443#local",
	})
	requireFixturef(t, err == nil, "create local key: %v", err)

	importedID, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{
		Label: "Imported", Status: "active", Kind: "real", Category: "Source",
		KeyURL: "vless://bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb@example.com:443#imported",
	})
	requireFixturef(t, err == nil, "create imported key: %v", err)

	{
		_, err := db.Exec(`INSERT INTO external_subscription_sources(id, name, source_url, key_category_id, key_category) VALUES(99, 'Feed', 'https://feed.example/sub', ?, 'Source')`, source.ID)
		requireFixturef(t, err == nil, "seed external source: %v", err)
	}

	{
		_, err := db.Exec(`UPDATE vless_keys SET external_source_id = 99 WHERE id = ?`, importedID)
		requireFixturef(t, err == nil, "mark imported key: %v", err)
	}

	return categoryCollisionFixture{db: db, repo: repo, ctx: ctx, targetID: target.ID, localID: localID, importedID: importedID}
}
func checkCategoryMergeReferences(t *testing.T, fixture categoryCollisionFixture) {
	t.Helper()
	ctx, db, repo, localID, importedID := fixture.ctx, fixture.db, fixture.repo, fixture.localID, fixture.importedID
	merged, err := repo.UpdateKeyCategory(ctx, keymanagement.UpdateCategoryParams{
		OldName: "Source", NewName: "Target", Color: "#444444",
	})
	requireFixturef(t, err == nil, "merge category rename: %v", err)

	requireFixturef(t, merged.ID == fixture.targetID, "unexpected merged category: %#v", merged)
	requireFixturef(t, merged.Name == "Target", "unexpected merged category: %#v", merged)
	requireFixturef(t, merged.Color == "#444444", "unexpected merged category: %#v", merged)

	for _, id := range []int64{localID, importedID} {
		var categoryID int64
		var category string
		{
			err := db.QueryRow(`SELECT category_id, category FROM vless_keys WHERE id = ?`, id).Scan(&categoryID, &category)
			requireFixturef(t, err == nil, "load key %d category: %v", id, err)
		}

		requireFixturef(t, categoryID == fixture.targetID, "key %d category = (%d, %q), want (%d, Target)", id, categoryID, category, fixture.targetID)
		requireFixturef(t, category == "Target", "key %d category = (%d, %q), want (%d, Target)", id, categoryID, category, fixture.targetID)

	}
	var sourceCategoryID int64
	var sourceCategory string
	{
		err := db.QueryRow(`SELECT key_category_id, key_category FROM external_subscription_sources WHERE id = 99`).Scan(&sourceCategoryID, &sourceCategory)
		requireFixturef(t, err == nil, "load source category reference: %v", err)
	}

	requireFixturef(t, sourceCategoryID == fixture.targetID, "source reference = (%d, %q), want (%d, Target)", sourceCategoryID, sourceCategory, fixture.targetID)
	requireFixturef(t, sourceCategory == "Target", "source reference = (%d, %q), want (%d, Target)", sourceCategoryID, sourceCategory, fixture.targetID)

}
func checkCategoryReorderValidation(t *testing.T, fixture categoryCollisionFixture) {
	t.Helper()
	ctx, repo, localID, importedID := fixture.ctx, fixture.repo, fixture.localID, fixture.importedID
	{
		err := repo.ReorderKeyCategories(ctx, []string{"Target", "Unknown"})
		requireFixturef(t, err == nil, "partial/unknown category reorder must remain accepted: %v", err)
	}

	{
		err := repo.ReorderKeys(ctx, []int64{localID})
		requireFixturef(t, errors.Is(err, keymanagement.ErrInvalidKeyOrderCount), "partial key order error = %v", err)
	}

	{
		err := repo.ReorderKeys(ctx, []int64{localID, 999})
		requireFixturef(t, errors.Is(err, keymanagement.ErrUnknownKeyInOrder), "unknown key order error = %v", err)
	}

	{
		err := repo.ReorderKeys(ctx, []int64{localID, localID})
		requireFixturef(t, errors.Is(err, keymanagement.ErrDuplicateKeyInOrder), "duplicate key order error = %v", err)
	}

	{
		err := repo.ReorderKeys(ctx, []int64{importedID, localID})
		requireFixturef(t, err == nil, "complete key order: %v", err)
	}

}
func TestKeyRepository_CategoryDeletionModesIncludeSourceOwnedKeys(t *testing.T) {
	newFixture := func(t *testing.T, category string) (*Repository, int64, int64) {
		t.Helper()
		db := setupTestDB(t)
		t.Cleanup(func() { _ = db.Close() })
		repo := NewRepository(db, newTestKeyringForStorage(t))
		cat, err := repo.CreateKeyCategory(context.Background(), keymanagement.CreateCategoryParams{Name: category, Color: "#123456"})
		requireFixturef(t, err == nil, "create category: %v", err)

		id, err := repo.CreateLegacy(context.Background(), keymanagement.CreateLegacyKeyParams{
			Label: category, Status: "active", Kind: "real", Category: category,
			KeyURL: "vless://cccccccc-cccc-cccc-cccc-cccccccccccc@example.com:443#owned",
		})
		requireFixturef(t, err == nil, "create key: %v", err)

		{
			_, err := db.Exec(`UPDATE vless_keys SET external_source_id = 77 WHERE id = ?`, id)
			requireFixturef(t, err == nil, "mark source-owned: %v", err)
		}

		return repo, cat.ID, id
	}

	t.Run("keep_keys", func(t *testing.T) {
		repo, _, id := newFixture(t, "Keep")
		{
			err := repo.DeleteKeyCategory(context.Background(), keymanagement.DeleteCategoryParams{Name: "Keep", Mode: "keep_keys"})
			requireFixturef(t, err == nil, "delete category keep_keys: %v", err)
		}

		var categoryID *int64
		var category string
		{
			err := repo.db.QueryRow(`SELECT category_id, category FROM vless_keys WHERE id = ?`, id).Scan(&categoryID, &category)
			requireFixturef(t, err == nil, "source-owned key was deleted: %v", err)
		}

		requireFixturef(t, categoryID == nil, "category was not cleared: id=%v name=%q", categoryID, category)
		requireFixturef(t, category == "", "category was not cleared: id=%v name=%q", categoryID, category)

	})

	t.Run("delete_with_keys", func(t *testing.T) {
		repo, _, id := newFixture(t, "Delete")
		{
			err := repo.DeleteKeyCategory(context.Background(), keymanagement.DeleteCategoryParams{Name: "Delete", Mode: "delete_with_keys"})
			requireFixturef(t, err == nil, "delete category and keys: %v", err)
		}

		var count int
		{
			err := repo.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, id).Scan(&count)
			requireFixturef(t, err == nil, "source-owned key count = %d, err=%v", count, err)
			requireFixturef(t, count == 0, "source-owned key count = %d, err=%v", count, err)
		}

	})
}

func TestKeyRepository_BulkMutationsRollbackOnMissingKey(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	repo := NewRepository(db, newTestKeyringForStorage(t))
	ctx := context.Background()
	first, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "First", Status: "active", Kind: "real", KeyURL: "vless://dddddddd-dddd-dddd-dddd-dddddddddddd@example.com:443#first"})
	requireFixturef(t, err == nil, "create first: %v", err)

	second, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "Second", Status: "active", Kind: "real", KeyURL: "vless://eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee@example.com:443#second"})
	requireFixturef(t, err == nil, "create second: %v", err)

	err = repo.BulkUpdateKeys(ctx, keymanagement.BulkKeyUpdate{IDs: []int64{first, 999, second}, Status: "disabled"})
	var missing keymanagement.KeyNotFoundError
	requireFixturef(t, errors.As(err, &missing), "bulk update error = %v", err)
	requireFixturef(t, missing.ID == 999, "bulk update error = %v", err)

	var status string
	{
		err := db.QueryRow(`SELECT status FROM vless_keys WHERE id = ?`, first).Scan(&status)
		requireFixturef(t, err == nil, "bulk update did not roll back first row: status=%q err=%v", status, err)
		requireFixturef(t, status == "active", "bulk update did not roll back first row: status=%q err=%v", status, err)
	}

	err = repo.BulkDeleteKeys(ctx, []int64{first, 999, second})
	requireFixturef(t, errors.As(err, &missing), "bulk delete error = %v", err)
	requireFixturef(t, missing.ID == 999, "bulk delete error = %v", err)

	var count int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id IN (?, ?)`, first, second).Scan(&count)
		requireFixturef(t, err == nil, "bulk delete did not roll back: count=%d err=%v", count, err)
		requireFixturef(t, count == 2, "bulk delete did not roll back: count=%d err=%v", count, err)
	}

}
