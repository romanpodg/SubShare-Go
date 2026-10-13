package storage

import (
	"context"
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"testing"
)

func TestProfileRepository_Create_Update_Clone(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForRepo(t)
	repo := NewRepository(db, kr)
	{
		_, err := db.Exec(`INSERT INTO users(name, token, key_assignment_mode) VALUES('u1', 'tok-1', 'all'), ('u2', 'tok-2', 'selected')`)
		requireFixturef(t, err == nil, "seed key assignment modes: %v", err)
	}

	// 1. Create Local Profile
	vlessURI := "vless://user1@example.com:443?encryption=none#Node1"
	createdKey, decryptedURI, err := repo.CreateLocal(ctx, keymanagement.CreateProfileParams{
		Label:    "Key One",
		Status:   "active",
		Kind:     "real",
		Category: "General",
		Protocol: "vless",
		BuiltURI: vlessURI,
	})
	requireFixturef(t, err == nil, "CreateLocal: %v", err)

	requireFixturef(t, createdKey.ID > 0, "unexpected created key metadata: %#v", createdKey)
	requireFixturef(t, createdKey.Label == "Key One", "unexpected created key metadata: %#v", createdKey)
	requireFixturef(t, createdKey.ProfileRevision == 1, "unexpected created key metadata: %#v", createdKey)

	requireFixturef(t, decryptedURI == vlessURI, "decryptedURI = %q, want %q", decryptedURI, vlessURI)

	// Verify plaintext URI is absent from parent storage table
	var parentBlindIndex string
	{
		err := db.QueryRow(`SELECT url_blind_index FROM vless_keys WHERE id = ?`, createdKey.ID).Scan(&parentBlindIndex)
		requireFixturef(t, err == nil, "query blind index: %v", err)
	}

	requireFixturef(t, parentBlindIndex != "", "invalid parent blind index: %q", parentBlindIndex)
	requireFixturef(t, parentBlindIndex != vlessURI, "invalid parent blind index: %q", parentBlindIndex)

	// 2. Update Local Profile with correct revision
	newURI := "vless://user1-updated@example.com:443?encryption=none#Node1Updated"
	updatedKey, updatedURI, err := repo.UpdateLocal(ctx, keymanagement.UpdateProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 1,
		Label:            "Key One Updated",
		Status:           "active",
		Kind:             "real",
		Category:         "General",
		Protocol:         "vless",
		NewURI:           newURI,
	})
	requireFixturef(t, err == nil, "UpdateLocal: %v", err)

	requireFixturef(t, updatedKey.ProfileRevision == 2, "unexpected updated key metadata: %#v", updatedKey)
	requireFixturef(t, updatedKey.Label == "Key One Updated", "unexpected updated key metadata: %#v", updatedKey)

	requireFixturef(t, updatedURI == newURI, "updatedURI = %q, want %q", updatedURI, newURI)

	// 3. Stale revision update conflict
	_, _, err = repo.UpdateLocal(ctx, keymanagement.UpdateProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 1, // stale revision!
		Label:            "Stale Edit",
		Status:           "active",
		Kind:             "real",
		Protocol:         "vless",
		NewURI:           newURI,
	})
	requireFixturef(t, errors.Is(err, keymanagement.ErrProfileRevisionConflict), "expected ErrProfileRevisionConflict, got %v", err)

	// 4. Source-owned read-only rejection
	{
		_, err := db.Exec(`UPDATE vless_keys SET external_source_id = 99 WHERE id = ?`, createdKey.ID)
		requireFixturef(t, err == nil, "set external_source_id: %v", err)
	}

	_, _, err = repo.UpdateLocal(ctx, keymanagement.UpdateProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 2,
		Label:            "Forbidden Edit",
		Status:           "active",
		Kind:             "real",
		Protocol:         "vless",
		NewURI:           newURI,
	})
	requireFixturef(t, errors.Is(err, keymanagement.ErrSourceOwnedReadOnly), "expected ErrSourceOwnedProfile, got %v", err)

	// Reset source ID for clone test
	{
		_, err := db.Exec(`UPDATE vless_keys SET external_source_id = NULL WHERE id = ?`, createdKey.ID)
		requireFixturef(t, err == nil, "clear external_source_id: %v", err)
	}

	// 5. Clone Local Profile
	clonedKey, clonedURI, err := repo.CloneLocal(ctx, keymanagement.CloneProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 2,
		NewLabel:         "Key One Copy",
	})
	requireFixturef(t, err == nil, "CloneLocal: %v", err)

	requireFixturef(t, clonedKey.ID != createdKey.ID, "unexpected cloned key metadata: %#v", clonedKey)
	requireFixturef(t, clonedKey.Label == "Key One Copy", "unexpected cloned key metadata: %#v", clonedKey)
	requireFixturef(t, clonedKey.ProfileRevision == 1, "unexpected cloned key metadata: %#v", clonedKey)

	requireFixturef(t, clonedKey.ExternalSourceID == 0, "cloned key external_source_id = %d, want 0", clonedKey.ExternalSourceID)

	requireFixturef(t, clonedURI == newURI, "clonedURI = %q, want %q", clonedURI, newURI)

	requireFixturef(t, clonedKey.ClientDisplayName == "Node1Updated", "local clone raw-name fallback changed: %#v", clonedKey)
	requireFixturef(t, !(clonedKey.ClientDisplayNameOverridden), "local clone raw-name fallback changed: %#v", clonedKey)

	var allAssignments, selectedAssignments int
	{
		err := db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = 1`).Scan(&allAssignments)
		requireFixturef(t, err == nil, "count all-mode assignments: %v", err)
	}

	{
		err := db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = 2`).Scan(&selectedAssignments)
		requireFixturef(t, err == nil, "count selected-mode assignments: %v", err)
	}

	requireFixturef(t, allAssignments == 2, "clone assignment mismatch: all=%d selected=%d, want 2 and 0", allAssignments, selectedAssignments)
	requireFixturef(t, selectedAssignments == 0, "clone assignment mismatch: all=%d selected=%d, want 2 and 0", allAssignments, selectedAssignments)

	// 6. Metadata-only updates to a clone must retain its intentional
	// non-canonical blind index instead of colliding with the source row.
	updatedClone, updatedCloneURI, err := repo.UpdateLocal(ctx, keymanagement.UpdateProfileParams{
		ID:               clonedKey.ID,
		ExpectedRevision: 1,
		Label:            "Key One Copy Renamed",
		Status:           "active",
		Kind:             "real",
		Category:         "General",
		Protocol:         "vless",
		NewURI:           clonedURI,
	})
	requireFixturef(t, err == nil, "UpdateLocal clone metadata: %v", err)

	requireFixturef(t, updatedClone.ProfileRevision == 2, "unexpected updated clone metadata: %#v", updatedClone)
	requireFixturef(t, updatedClone.Label == "Key One Copy Renamed", "unexpected updated clone metadata: %#v", updatedClone)

	requireFixturef(t, updatedCloneURI == clonedURI, "updated clone URI = %q, want %q", updatedCloneURI, clonedURI)

}

func TestProfileRepository_ErrorClassifications(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForRepo(t)
	repo := NewRepository(db, kr)

	// 1. Get non-existent key -> ErrProfileNotFound
	_, _, err := repo.GetByID(ctx, 999)
	requireFixturef(t, errors.Is(err, keymanagement.ErrKeyNotFound), "expected ErrProfileNotFound on GetByID(999), got %v", err)

	// 2. Update non-existent key -> ErrProfileNotFound
	_, _, err = repo.UpdateLocal(ctx, keymanagement.UpdateProfileParams{
		ID:               999,
		ExpectedRevision: 1,
		Label:            "Non-existent",
		Status:           "active",
		Kind:             "real",
		Protocol:         "vless",
		NewURI:           "vless://a@b.com:443#a",
	})
	requireFixturef(t, errors.Is(err, keymanagement.ErrKeyNotFound), "expected ErrProfileNotFound on UpdateLocal(999), got %v", err)

	// 3. Clone non-existent key -> ErrProfileNotFound
	_, _, err = repo.CloneLocal(ctx, keymanagement.CloneProfileParams{
		ID:               999,
		ExpectedRevision: 1,
		NewLabel:         "Clone non-existent",
	})
	requireFixturef(t, errors.Is(err, keymanagement.ErrKeyNotFound), "expected ErrProfileNotFound on CloneLocal(999), got %v", err)

	// Create valid base key
	createdKey, _, err := repo.CreateLocal(ctx, keymanagement.CreateProfileParams{
		Label:    "Key Test",
		Status:   "active",
		Kind:     "real",
		Protocol: "vless",
		BuiltURI: "vless://test@example.com:443#Test",
	})
	requireFixturef(t, err == nil, "CreateLocal: %v", err)

	// 4. Update with stale revision -> ErrProfileRevisionConflict
	_, _, err = repo.UpdateLocal(ctx, keymanagement.UpdateProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 99,
		Label:            "Stale Revision",
		Status:           "active",
		Kind:             "real",
		Protocol:         "vless",
		NewURI:           "vless://test@example.com:443#Stale",
	})
	requireFixturef(t, errors.Is(err, keymanagement.ErrProfileRevisionConflict), "expected ErrProfileRevisionConflict on UpdateLocal stale rev, got %v", err)

	// Verify failed update leaves parent and child unchanged
	var uneditedLabel string
	{
		err := db.QueryRow(`SELECT label FROM vless_keys WHERE id = ?`, createdKey.ID).Scan(&uneditedLabel)
		requireFixturef(t, err == nil, "parent row modified after failed update: %q", uneditedLabel)
		requireFixturef(t, uneditedLabel == "Key Test", "parent row modified after failed update: %q", uneditedLabel)
	}

	// 5. Clone with stale revision -> ErrProfileRevisionConflict
	_, _, err = repo.CloneLocal(ctx, keymanagement.CloneProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 99,
		NewLabel:         "Stale Clone",
	})
	requireFixturef(t, errors.Is(err, keymanagement.ErrProfileRevisionConflict), "expected ErrProfileRevisionConflict on CloneLocal stale rev, got %v", err)

	// 6. Missing child secret during clone -> ErrStorageIntegrity
	{
		_, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, createdKey.ID)
		requireFixturef(t, err == nil, "delete secret row: %v", err)
	}

	{
		_, _, err = repo.GetByID(ctx, createdKey.ID)
		requireFixturef(t, errors.Is(err, keymanagement.ErrStorageIntegrity), "expected ErrStorageIntegrity on GetByID missing secret, got %v", err)
	}

	_, _, err = repo.CloneLocal(ctx, keymanagement.CloneProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 1,
		NewLabel:         "Missing Secret Clone",
	})
	requireFixturef(t, errors.Is(err, keymanagement.ErrStorageIntegrity), "expected ErrStorageIntegrity on CloneLocal missing secret, got %v", err)

	// 7. Corrupt ciphertext during clone -> ErrStorageIntegrity
	{
		_, err := db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, 'v1:key-1:corrupt_junk')`, createdKey.ID)
		requireFixturef(t, err == nil, "insert corrupt secret: %v", err)
	}

	{
		_, _, err = repo.GetByID(ctx, createdKey.ID)
		requireFixturef(t, errors.Is(err, keymanagement.ErrStorageIntegrity), "expected ErrStorageIntegrity on GetByID corrupt ciphertext, got %v", err)
	}

	_, _, err = repo.CloneLocal(ctx, keymanagement.CloneProfileParams{
		ID:               createdKey.ID,
		ExpectedRevision: 1,
		NewLabel:         "Corrupt Clone",
	})
	requireFixturef(t, errors.Is(err, keymanagement.ErrStorageIntegrity), "expected ErrStorageIntegrity on CloneLocal corrupt ciphertext, got %v", err)

}
