package storage

import (
	"context"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"testing"
)

func TestStorage_LegacyKeyCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	{
		_, err := db.Exec(`INSERT INTO users(id, name, token, key_assignment_mode) VALUES(1, 'u1', 'tok-1', 'all'), (2, 'u2', 'tok-2', 'selected')`)
		requireFixturef(t, err == nil, "seed users: %v", err)
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
	requireFixturef(t, err == nil, "CreateLegacy failed: %v", err)

	requireFixturef(t, keyID > 0, "expected positive keyID, got %d", keyID)

	var parentLabel, urlBlindIndex string
	{
		err := db.QueryRow(`SELECT label, url_blind_index FROM vless_keys WHERE id = ?`, keyID).Scan(&parentLabel, &urlBlindIndex)
		requireFixturef(t, err == nil, "query parent row: %v", err)
	}

	requireFixturef(t, parentLabel == "Storage Legacy Key", "unexpected label: %q", parentLabel)

	requireFixturef(t, urlBlindIndex != "", "expected non-empty blind index")

	var encURL string
	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&encURL)
		requireFixturef(t, err == nil, "query secret row: %v", err)
	}

	dec, err := profilestorage.Decrypt(encURL, kr, keyID)
	requireFixturef(t, err == nil, "decrypt failed or mismatched URI: err=%v, uri=%q", err, dec.Reveal())
	requireFixturef(t, dec.Reveal() == vlessURI, "decrypt failed or mismatched URI: err=%v, uri=%q", err, dec.Reveal())

	var assignedUserCount int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE key_id = ?`, keyID).Scan(&assignedUserCount)
		requireFixturef(t, err == nil, "query user_keys: %v", err)
	}

	requireFixturef(t, assignedUserCount == 1, "expected 1 assigned user, got %d", assignedUserCount)

	// 2. List Legacy Keys
	keys, err := repo.ListLegacy(ctx)
	requireFixturef(t, err == nil, "ListLegacy failed: %v", err)

	requireFixturef(t, len(keys) == 1, "ListLegacy returned unexpected key: %#v", keys)
	requireFixturef(t, keys[0].ID == keyID, "ListLegacy returned unexpected key: %#v", keys)
	requireFixturef(t, keys[0].URL == vlessURI, "ListLegacy returned unexpected key: %#v", keys)

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
	requireFixturef(t, err == nil, "UpdateLegacy failed: %v", err)

	{
		err := db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&encURL)
		requireFixturef(t, err == nil, "query updated secret: %v", err)
	}

	decUpd, err := profilestorage.Decrypt(encURL, kr, keyID)
	requireFixturef(t, err == nil, "decryption of updated secret failed: err=%v, uri=%q", err, decUpd.Reveal())
	requireFixturef(t, decUpd.Reveal() == updatedURI, "decryption of updated secret failed: err=%v, uri=%q", err, decUpd.Reveal())

	// 4. Delete Legacy Key
	err = repo.DeleteLegacy(ctx, keyID)
	requireFixturef(t, err == nil, "DeleteLegacy failed: %v", err)

	var keyCount, secretCount, assignmentCount int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, keyID).Scan(&keyCount)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT COUNT(*) FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&secretCount)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE key_id = ?`, keyID).Scan(&assignmentCount)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	requireFixturef(t, keyCount == 0, "cleanup failed after delete: keys=%d, secrets=%d, assignments=%d", keyCount, secretCount, assignmentCount)
	requireFixturef(t, secretCount == 0, "cleanup failed after delete: keys=%d, secrets=%d, assignments=%d", keyCount, secretCount, assignmentCount)
	requireFixturef(t, assignmentCount == 0, "cleanup failed after delete: keys=%d, secrets=%d, assignments=%d", keyCount, secretCount, assignmentCount)

}

func TestStorage_LegacyKeyUpdateOrdering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForStorage(t)
	repo := NewRepository(db, kr)
	ctx := context.Background()

	id1, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "K1", Status: "active", Kind: "real", KeyURL: "vless://11111111-1111-1111-1111-111111111111@e.com:443#k1"})
	requireFixturef(t, err == nil, "fixture CreateLegacy: %v", err)

	id2, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "K2", Status: "active", Kind: "real", KeyURL: "vless://22222222-2222-2222-2222-222222222222@e.com:443#k2"})
	requireFixturef(t, err == nil, "fixture CreateLegacy: %v", err)

	id3, err := repo.CreateLegacy(ctx, keymanagement.CreateLegacyKeyParams{Label: "K3", Status: "active", Kind: "real", KeyURL: "vless://33333333-3333-3333-3333-333333333333@e.com:443#k3"})
	requireFixturef(t, err == nil, "fixture CreateLegacy: %v", err)

	var s1, s2, s3 int64
	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id1).Scan(&s1)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id2).Scan(&s2)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id3).Scan(&s3)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	requireFixturef(t, s1 == 1, "unexpected initial sort orders: s1=%d, s2=%d, s3=%d", s1, s2, s3)
	requireFixturef(t, s2 == 2, "unexpected initial sort orders: s1=%d, s2=%d, s3=%d", s1, s2, s3)
	requireFixturef(t, s3 == 3, "unexpected initial sort orders: s1=%d, s2=%d, s3=%d", s1, s2, s3)

	// Update K2 without changing kind -> sort_order MUST stay 2
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{
		ID:       id2,
		Label:    "K2 Updated",
		Status:   "active",
		Kind:     "real",
		BuiltURL: "vless://22222222-2222-2222-2222-222222222222@e.com:443#k2_upd",
	})
	requireFixturef(t, err == nil, "UpdateLegacy failed: %v", err)

	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id1).Scan(&s1)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id2).Scan(&s2)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id3).Scan(&s3)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	requireFixturef(t, s1 == 1, "updating K2 altered sort orders unexpectedly: s1=%d, s2=%d, s3=%d", s1, s2, s3)
	requireFixturef(t, s2 == 2, "updating K2 altered sort orders unexpectedly: s1=%d, s2=%d, s3=%d", s1, s2, s3)
	requireFixturef(t, s3 == 3, "updating K2 altered sort orders unexpectedly: s1=%d, s2=%d, s3=%d", s1, s2, s3)

	// Update K2 changing kind from 'real' to 'informational' -> sort_order MUST become MAX + 1 = 4
	err = repo.UpdateLegacy(ctx, keymanagement.UpdateLegacyKeyParams{
		ID:           id2,
		Label:        "K2 Info",
		Status:       "active",
		Kind:         "informational",
		TemplateText: "Info Text",
		BuiltURL:     "info://abcdef123456",
	})
	requireFixturef(t, err == nil, "UpdateLegacy kind change failed: %v", err)

	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id1).Scan(&s1)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id2).Scan(&s2)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	{
		err := db.QueryRow(`SELECT sort_order FROM vless_keys WHERE id = ?`, id3).Scan(&s3)
		requireFixturef(t, err == nil, "fixture Scan: %v", err)
	}

	requireFixturef(t, s1 == 1, "updating K2 kind failed to assign MAX+1: s1=%d, s2=%d (expected 4), s3=%d", s1, s2, s3)
	requireFixturef(t, s2 == 4, "updating K2 kind failed to assign MAX+1: s1=%d, s2=%d (expected 4), s3=%d", s1, s2, s3)
	requireFixturef(t, s3 == 3, "updating K2 kind failed to assign MAX+1: s1=%d, s2=%d (expected 4), s3=%d", s1, s2, s3)

}
