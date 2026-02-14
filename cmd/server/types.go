package main

import (
	"database/sql"
	"html/template"
	"sync"
	"time"
)

type App struct {
	db                 *sql.DB
	templates          *template.Template
	adminUser          string
	adminPass          string
	deviceLimitMessage string
	sessions           map[string]AdminSession
	mu                 sync.RWMutex
}

type AdminSession struct {
	ExpiresAt time.Time
	CSRFToken string
}

const (
	userStatusActive  = "active"
	userStatusPaused  = "paused"
	userStatusBlocked = "blocked"

	keyStatusActive    = "active"
	keyStatusNonActive = "non-active"
)

type User struct {
	ID                   int64
	Name                 string
	Email                string
	ActivationCode       string
	SubscriptionID       string
	ActivationUsedAt     string
	Status               string
	StartsAtInput        string
	ExpiresAtInput       string
	BlockedReason        string
	AssignedKeyIDs       string
	MaxDevices           int
	ConnectedDeviceCount int
	ConnectedHWIDs       []string
	CreatedAt            time.Time
}

type VLESSKey struct {
	ID                int64
	Label             string
	URL               string
	URLShort          string
	Status            string
	StatusLabel       string
	CheckStatus       string
	CheckStatusLabel  string
	CheckError        string
	LastLatencyMS     int64
	LastCheckedAtText string
	EditUUID          string
	EditHost          string
	EditPort          string
	EditQuery         string
	EditFragment      string
	CreatedAt         time.Time
}

type AdminPageData struct {
	Users          []User
	Keys           []VLESSKey
	AssignableKeys []VLESSKey
	BaseURL        string
	CSRFToken      string
	Message        string
	Error          string
}

type LoginPageData struct {
	Error string
}

type SubscriptionPageData struct {
	Token           string
	SubscriptionURL string
	Message         string
	Error           string
}
