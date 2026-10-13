package storage

import (
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"testing"
)

func newTestKeyringForStorage(t *testing.T) *profilestorage.Keyring {
	t.Helper()
	data, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	requireFixturef(t, err == nil, "generate keyring: %v", err)

	kr, err := profilestorage.LoadKeyringJSON(data)
	requireFixturef(t, err == nil, "load keyring: %v", err)

	return kr
}
