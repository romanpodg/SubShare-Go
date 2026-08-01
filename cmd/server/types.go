package main

import (
	"database/sql"
	"sync"

	adminpassword "github.com/romanpodg/SubShare-Go/internal/security/password"
)

type App struct {
	db                       *sql.DB
	dbPath                   string
	backupPath               string
	deviceLimitMessage       string
	baseURL                  string
	happCryptoAPIURL         string
	subscriptionBodyEncoding string
	adminPasswordHasher      *adminpassword.Hasher
	mu                       sync.RWMutex
}
