package storage

import (
	"context"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
)

func TestProfileCategoryRenameRollsBackKeyAndSourceReferences(t *testing.T) {
	f := newProfileContractFixture(t)
	f.makeSourceOwned(t)
	before := profileContractSnapshot(t, f.db)
	profileContractExec(t, f.db, `CREATE TRIGGER fail_source_category BEFORE UPDATE OF key_category_id ON external_subscription_sources BEGIN SELECT RAISE(ABORT, 'r06 source category failure'); END`)
	params := keymanagement.UpdateCategoryParams{OldName: "Original", NewName: "Renamed", Color: "#112233"}
	if _, err := f.repo.UpdateKeyCategory(context.Background(), params); err == nil {
		t.Fatal("rename ignored source reference failure")
	}
	assertProfileContractSnapshot(t, f.db, before)
	profileContractExec(t, f.db, `DROP TRIGGER fail_source_category`)
	category, err := f.repo.UpdateKeyCategory(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	var keyCategoryID, sourceCategoryID int64
	var keyName, sourceName string
	profileContractSuccess(t, f.db.QueryRow(`SELECT category_id, category FROM vless_keys WHERE id=?`, f.key.ID).Scan(&keyCategoryID, &keyName))
	profileContractSuccess(t, f.db.QueryRow(`SELECT key_category_id, key_category FROM external_subscription_sources WHERE id=77`).Scan(&sourceCategoryID, &sourceName))
	profileContractEqual(t, "new category identity", category.ID != f.key.CategoryID, true)
	profileContractEqual(t, "renamed key category ID", keyCategoryID, category.ID)
	profileContractEqual(t, "renamed source category ID", sourceCategoryID, category.ID)
	profileContractEqual(t, "renamed key category name", keyName, "Renamed")
	profileContractEqual(t, "renamed source category name", sourceName, "Renamed")
	profileContractEqual(t, "renamed color", category.Color, "#112233")
	profileContractEqual(t, "old category deleted", profileContractCount(t, f.db, `SELECT COUNT(*) FROM key_categories WHERE name='Original'`), 0)
	profileContractEqual(t, "unchanged category secret", profileContractSnapshot(t, f.db)["vless_key_secrets"], before["vless_key_secrets"])
}

func TestProfileCategoryRenameMergesExistingCategoryAndLegacyReferences(t *testing.T) {
	f := newProfileContractFixture(t)
	f.makeSourceOwned(t)
	profileContractExec(t, f.db, `INSERT INTO key_categories(name, color, sort_order) VALUES('Existing', '#abcdef', 9)`)
	var destinationID int64
	profileContractSuccess(t, f.db.QueryRow(`SELECT id FROM key_categories WHERE name='Existing'`).Scan(&destinationID))
	// Old databases can contain compatibility names without category IDs. Drop
	// only the fixture's synchronizing trigger to recreate that persisted state.
	profileContractExec(t, f.db, `DROP TRIGGER trg_vless_keys_category_id_update`)
	profileContractExec(t, f.db, `UPDATE vless_keys SET category_id=NULL, category='Original' WHERE id=?`, f.key.ID)
	profileContractExec(t, f.db, `UPDATE external_subscription_sources SET key_category_id=NULL, key_category='Original' WHERE id=77`)
	before := profileContractSnapshot(t, f.db)
	category, err := f.repo.UpdateKeyCategory(context.Background(), keymanagement.UpdateCategoryParams{OldName: "Original", NewName: "Existing", Color: "#112233"})
	profileContractSuccess(t, err)
	profileContractEqual(t, "merged category identity", category.ID, destinationID)
	var keyID, sourceID, sortOrder int64
	profileContractSuccess(t, f.db.QueryRow(`SELECT category_id FROM vless_keys WHERE id=?`, f.key.ID).Scan(&keyID))
	profileContractSuccess(t, f.db.QueryRow(`SELECT key_category_id FROM external_subscription_sources WHERE id=77`).Scan(&sourceID))
	profileContractSuccess(t, f.db.QueryRow(`SELECT sort_order FROM key_categories WHERE id=?`, destinationID).Scan(&sortOrder))
	profileContractEqual(t, "legacy key reference assigned", keyID, destinationID)
	profileContractEqual(t, "legacy source reference assigned", sourceID, destinationID)
	profileContractEqual(t, "existing destination order retained", sortOrder, int64(9))
	after := profileContractSnapshot(t, f.db)
	profileContractEqual(t, "one merged category", len(after["key_categories"]), 1)
	profileContractEqual(t, "merge preserved secrets", after["vless_key_secrets"], before["vless_key_secrets"])
	profileContractEqual(t, "merge preserved user assignments", after["user_keys"], before["user_keys"])
}

func TestProfileCategoryDeleteModesAreAtomic(t *testing.T) {
	for _, mode := range []string{"keep_keys", "delete_with_keys"} {
		t.Run(mode, func(t *testing.T) {
			f := newProfileContractFixture(t)
			before := profileContractSnapshot(t, f.db)
			profileContractExec(t, f.db, `CREATE TRIGGER fail_category_delete BEFORE DELETE ON key_categories BEGIN SELECT RAISE(ABORT, 'r06 category delete failure'); END`)
			params := keymanagement.DeleteCategoryParams{Name: "Original", Mode: mode}
			if err := f.repo.DeleteKeyCategory(context.Background(), params); err == nil {
				t.Fatal("category delete ignored injected failure")
			}
			assertProfileContractSnapshot(t, f.db, before)
			profileContractExec(t, f.db, `DROP TRIGGER fail_category_delete`)
			if err := f.repo.DeleteKeyCategory(context.Background(), params); err != nil {
				t.Fatal(err)
			}
			after := profileContractSnapshot(t, f.db)
			profileContractEqual(t, "category deleted", len(after["key_categories"]), 0)
			if mode == "delete_with_keys" {
				profileContractEqual(t, "deleted key cascade", len(after["vless_keys"])+len(after["vless_key_secrets"])+len(after["user_keys"]), 0)
				return
			}
			key, raw, err := f.repo.GetByID(context.Background(), f.key.ID)
			profileContractSuccess(t, err)
			profileContractEqual(t, "cleared category ID", key.CategoryID, int64(0))
			profileContractEqual(t, "cleared category name", key.Category, "")
			profileContractEqual(t, "keep-keys revision", key.ProfileRevision, int64(1))
			profileContractEqual(t, "keep-keys raw bytes", raw, contractProfileURI)
			profileContractEqual(t, "keep-keys ciphertext", after["vless_key_secrets"], before["vless_key_secrets"])
			profileContractEqual(t, "keep-keys assignments", after["user_keys"], before["user_keys"])
		})
	}
}

func TestProfileCategoryReorderRollsBackEarlierWrites(t *testing.T) {
	f := newProfileContractFixture(t)
	profileContractExec(t, f.db, `INSERT INTO key_categories(name, color, sort_order) VALUES('Second', '#223344', 2), ('Third', '#334455', 3)`)
	profileContractExec(t, f.db, `UPDATE key_categories SET sort_order=1 WHERE name='Original'`)
	before := profileContractSnapshot(t, f.db)
	profileContractExec(t, f.db, `CREATE TRIGGER fail_category_order BEFORE UPDATE OF sort_order ON key_categories WHEN NEW.name='Original' BEGIN SELECT RAISE(ABORT, 'r06 order failure'); END`)
	names := []string{"Third", "Original", "Second"}
	if err := f.repo.ReorderKeyCategories(context.Background(), names); err == nil {
		t.Fatal("reorder ignored second-write failure")
	}
	assertProfileContractSnapshot(t, f.db, before)
	profileContractExec(t, f.db, `DROP TRIGGER fail_category_order`)
	if err := f.repo.ReorderKeyCategories(context.Background(), names); err != nil {
		t.Fatal(err)
	}
	rows, err := f.db.Query(`SELECT name FROM key_categories ORDER BY sort_order`)
	if err != nil {
		t.Fatal(err)
	}
	profileContractEqual(t, "category order", profileContractRows(t, rows), [][]any{{"Third"}, {"Original"}, {"Second"}})
}

func TestProfileKeyReorderValidationAndAtomicity(t *testing.T) {
	f := newProfileContractFixture(t)
	other, _, err := runProfileContractCommand(f, "create")
	if err != nil {
		t.Fatal(err)
	}
	before := profileContractSnapshot(t, f.db)
	for _, tc := range []struct {
		name string
		ids  []int64
		want error
	}{
		{"missing", []int64{f.key.ID}, keymanagement.ErrInvalidKeyOrderCount},
		{"duplicate", []int64{f.key.ID, f.key.ID}, keymanagement.ErrDuplicateKeyInOrder},
		{"unknown", []int64{f.key.ID, 9999}, keymanagement.ErrUnknownKeyInOrder},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profileContractError(t, f.repo.ReorderKeys(context.Background(), tc.ids), tc.want)
			assertProfileContractSnapshot(t, f.db, before)
		})
	}
	profileContractExec(t, f.db, `CREATE TRIGGER fail_key_order BEFORE UPDATE OF sort_order ON vless_keys WHEN NEW.id=1 BEGIN SELECT RAISE(ABORT, 'r06 order failure'); END`)
	ids := []int64{other.ID, f.key.ID}
	if err := f.repo.ReorderKeys(context.Background(), ids); err == nil {
		t.Fatal("reorder ignored second-write failure")
	}
	assertProfileContractSnapshot(t, f.db, before)
	profileContractExec(t, f.db, `DROP TRIGGER fail_key_order`)
	if err := f.repo.ReorderKeys(context.Background(), ids); err != nil {
		t.Fatal(err)
	}
	list, err := f.repo.ListLegacy(context.Background())
	profileContractSuccess(t, err)
	profileContractEqual(t, "reordered list size", len(list), 2)
	profileContractEqual(t, "first reordered key", list[0].ID, other.ID)
	profileContractEqual(t, "second reordered key", list[1].ID, f.key.ID)
}
