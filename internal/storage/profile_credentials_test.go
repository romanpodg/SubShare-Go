package storage

import (
	"context"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"testing"
)

func TestProfileRepository_BlindIndexKeySeparation(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	krBase, krDiffBIK, krDiffEnc := newTwoKeyKeyring(t)
	repoBase := NewRepository(db, krBase)
	repoDiffBIK := NewRepository(db, krDiffBIK)
	repoDiffEnc := NewRepository(db, krDiffEnc)

	uri := "vless://secret-user@example.com:443?encryption=none#TestBlindIndex"

	// Create key with base keyring (encKey: key-1, BIK: bik-1)
	key1, _, err := repoBase.CreateLocal(ctx, keymanagement.CreateProfileParams{
		Label:    "Key 1",
		Status:   "active",
		Kind:     "real",
		Protocol: "vless",
		BuiltURI: uri,
	})
	requireFixturef(t, err == nil, "CreateLocal 1: %v", err)

	var blindIndex1, cipherText1 string
	{
		err := db.QueryRow(`SELECT k.url_blind_index, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = ?`, key1.ID).Scan(&blindIndex1, &cipherText1)
		requireFixturef(t, err == nil, "query key1: %v", err)
	}

	// 1. Plaintext URI is NOT stored in parent row or blind index
	requireFixturef(t, blindIndex1 != uri, "url_blind_index matches raw URI")

	// 2. Create key with diff BIK keyring (encKey: key-1, BIK: bik-2)
	key2, _, err := repoDiffBIK.CreateLocal(ctx, keymanagement.CreateProfileParams{
		Label:    "Key 2",
		Status:   "active",
		Kind:     "real",
		Protocol: "vless",
		BuiltURI: uri,
	})
	requireFixturef(t, err == nil, "CreateLocal 2: %v", err)

	var blindIndex2 string
	{
		err := db.QueryRow(`SELECT url_blind_index FROM vless_keys WHERE id = ?`, key2.ID).Scan(&blindIndex2)
		requireFixturef(t, err == nil, "query key2: %v", err)
	}

	// Changing ONLY blind index key MUST change url_blind_index
	requireFixturef(t, blindIndex1 != blindIndex2, "expected different blind indexes when BIK changes, got equal: %q", blindIndex1)

	// Production rejects duplicate deterministic blind indexes. Remove the
	// first row after capturing its ciphertext so the next assertion can
	// isolate encryption-key rotation from duplicate detection.
	{
		_, err := db.Exec(`DELETE FROM vless_keys WHERE id = ?`, key1.ID)
		requireFixturef(t, err == nil, "delete key1 before encryption-key comparison: %v", err)
	}

	// 3. Create key with diff Enc keyring (encKey: key-2, BIK: bik-1)
	key3, _, err := repoDiffEnc.CreateLocal(ctx, keymanagement.CreateProfileParams{
		Label:    "Key 3",
		Status:   "active",
		Kind:     "real",
		Protocol: "vless",
		BuiltURI: uri,
	})
	requireFixturef(t, err == nil, "CreateLocal 3: %v", err)

	var blindIndex3, cipherText3 string
	{
		err := db.QueryRow(`SELECT k.url_blind_index, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = ?`, key3.ID).Scan(&blindIndex3, &cipherText3)
		requireFixturef(t, err == nil, "query key3: %v", err)
	}

	// With SAME raw URI and SAME BIK (bik-1), different encryption keys MUST produce EXACT SAME url_blind_index
	requireFixturef(t, blindIndex1 == blindIndex3, "expected identical blind index when BIK is identical, got %q vs %q", blindIndex1, blindIndex3)

	// Proven decryption test: cipherText1 decrypts with krBase (key-1) but FAILS with krDiffEnc (key-2)
	dec1, decErr1 := profilestorage.Decrypt(cipherText1, krBase, key1.ID)
	requireFixturef(t, decErr1 == nil, "failed to decrypt cipherText1 with base keyring: %v", decErr1)
	requireFixturef(t, dec1.Reveal() == uri, "failed to decrypt cipherText1 with base keyring: %v", decErr1)

	_, decErr1WrongKey := profilestorage.Decrypt(cipherText1, krDiffEnc, key1.ID)
	requireFixturef(t, decErr1WrongKey != nil, "expected decryption failure when decrypting cipherText1 with wrong encryption key (krDiffEnc)")

	// cipherText3 decrypts with krDiffEnc (key-2) but FAILS with krBase (key-1)
	dec3, decErr3 := profilestorage.Decrypt(cipherText3, krDiffEnc, key3.ID)
	requireFixturef(t, decErr3 == nil, "failed to decrypt cipherText3 with diffEnc keyring: %v", decErr3)
	requireFixturef(t, dec3.Reveal() == uri, "failed to decrypt cipherText3 with diffEnc keyring: %v", decErr3)

	_, decErr3WrongKey := profilestorage.Decrypt(cipherText3, krBase, key3.ID)
	requireFixturef(t, decErr3WrongKey != nil, "expected decryption failure when decrypting cipherText3 with wrong encryption key (krBase)")

}
