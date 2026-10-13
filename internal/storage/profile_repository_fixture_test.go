package storage

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"path/filepath"
	"testing"
)

func newTestKeyringForRepo(t *testing.T) *profilestorage.Keyring {
	t.Helper()
	data, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	requireFixturef(t, err == nil, "generate keyring: %v", err)

	kr, err := profilestorage.LoadKeyringJSON(data)
	requireFixturef(t, err == nil, "load keyring: %v", err)

	return kr
}

type fixtureKeyring struct {
	keyID, keySecret, bikID, bikSecret string
}

func generateTestKeyring(t *testing.T, input fixtureKeyring) *profilestorage.Keyring {
	t.Helper()
	keyID, keySecret, bikID, bikSecret := input.keyID, input.keySecret, input.bikID, input.bikSecret
	keyB64 := base64.RawURLEncoding.EncodeToString([]byte(keySecret))
	bikB64 := base64.RawURLEncoding.EncodeToString([]byte(bikSecret))
	jsonStr := fmt.Sprintf(`{
		"active_key_id": %q,
		"keys": {%q: %q},
		"active_blind_index_key_id": %q,
		"blind_index_keys": {%q: %q}
	}`, keyID, keyID, keyB64, bikID, bikID, bikB64)
	kr, err := profilestorage.LoadKeyringJSON([]byte(jsonStr))
	requireFixturef(t, err == nil, "load keyring (%s, %s): %v", keyID, bikID, err)

	return kr
}

func newTwoKeyKeyring(t *testing.T) (*profilestorage.Keyring, *profilestorage.Keyring, *profilestorage.Keyring) {
	t.Helper()
	keySecret1 := "01234567890123456789012345678901"
	bikSecret1 := "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	requireFixture(t, keySecret1 != bikSecret1, "fixture error: encryption and BIK secrets must not be equal")

	krBase := generateTestKeyring(t, fixtureKeyring{"key-1", keySecret1, "bik-1", bikSecret1})
	krDiffBIK := generateTestKeyring(t, fixtureKeyring{"key-1", keySecret1, "bik-2", "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"})
	krDiffEnc := generateTestKeyring(t, fixtureKeyring{"key-2", "98765432109876543210987654321098", "bik-1", bikSecret1})

	return krBase, krDiffBIK, krDiffEnc
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
	requireFixturef(t, err == nil, "open db: %v", err)

	{
		_, err := db.Exec(`PRAGMA foreign_keys = ON`)
		requireFixturef(t, err == nil, "enable foreign keys: %v", err)
	}

	{
		err := MigrateWithKeyring(db, newTestKeyringForRepo(t))
		requireFixturef(t, err == nil, "migrate: %v", err)
	}

	return db
}
