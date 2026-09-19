package main

import (
	"database/sql"
	"sync"

	adminpassword "github.com/romanpodg/SubShare-Go/internal/security/password"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

type App struct {
	db                        *sql.DB
	dbPath                    string
	backupPath                string
	deviceLimitMessage        string
	baseURL                   string
	happCryptoAPIURL          string
	subscriptionBodyEncoding  string
	adminPasswordHasher       *adminpassword.Hasher
	profileFingerprintKey     []byte
	profileFingerprintOldKeys [][]byte
	profileKeyring            *profilestorage.Keyring
	mu                        sync.RWMutex
	keysOnce                  sync.Once
	keyHandlers               *keyHandlers
}

func cloneByteSlices(values [][]byte) [][]byte {
	cloned := make([][]byte, len(values))
	for index := range values {
		cloned[index] = append([]byte(nil), values[index]...)
	}
	return cloned
}
