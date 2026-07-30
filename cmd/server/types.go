package main

import (
	"database/sql"
	"sync"
)

type App struct {
	db                       *sql.DB
	dbPath                   string
	deviceLimitMessage       string
	baseURL                  string
	happCryptoAPIURL         string
	subscriptionBodyEncoding string
	mu                       sync.RWMutex
}
