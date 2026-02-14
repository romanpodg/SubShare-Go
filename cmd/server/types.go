package main

import (
	"database/sql"
	"sync"

	"xary-sub/internal/model"
)

type App struct {
	db                 *sql.DB
	adminUser          string
	adminPassHash      []byte
	deviceLimitMessage string
	baseURL            string
	sessions           map[string]model.AdminSession
	mu                 sync.RWMutex
}
