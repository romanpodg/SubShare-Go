package main

import (
	"database/sql"
	"github.com/romanpodg/SubShare-Go/internal/storage"
	"net/http"
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
	storeOnce                 sync.Once
	keyStore                  *storage.Repository
	keysOnce                  sync.Once
	sourceClientOnce          sync.Once
	sourceHTTPClient          *http.Client
	keyHandlers               *keyHandlers
}

func cloneByteSlices(values [][]byte) [][]byte {
	cloned := make([][]byte, len(values))
	for index := range values {
		cloned[index] = append([]byte(nil), values[index]...)
	}
	return cloned
}

// store is the Profile store: the only module that reads or writes vless_keys
// and vless_key_secrets, and the only one that touches the keyring.
func (a *App) store() *storage.Repository {
	a.storeOnce.Do(func() {
		a.keyStore = storage.NewRepository(a.db, a.profileKeyring)
	})
	return a.keyStore
}
