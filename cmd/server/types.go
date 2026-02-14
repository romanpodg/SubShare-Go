package main

import (
	"database/sql"
	"sync"
	"time"
)

type App struct {
	db                 *sql.DB
	adminUser          string
	adminPassHash      []byte
	deviceLimitMessage string
	baseURL            string
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
	ID                   int64    `json:"id"`
	Name                 string   `json:"name"`
	Email                string   `json:"email"`
	ActivationCode       string   `json:"activation_code"`
	SubscriptionID       string   `json:"subscription_id"`
	ActivationUsedAt     string   `json:"activation_used_at"`
	Status               string   `json:"status"`
	StartsAtInput        string   `json:"starts_at"`
	ExpiresAtInput       string   `json:"expires_at"`
	BlockedReason        string   `json:"blocked_reason"`
	AssignedKeyIDs       string   `json:"assigned_key_ids"`
	MaxDevices           int      `json:"max_devices"`
	ConnectedDeviceCount int      `json:"connected_device_count"`
	ConnectedHWIDs       []string `json:"connected_hwids"`
	CreatedAt            time.Time `json:"created_at"`
}

type VLESSKey struct {
	ID                int64     `json:"id"`
	Label             string    `json:"label"`
	URL               string    `json:"url"`
	URLShort          string    `json:"url_short"`
	Status            string    `json:"status"`
	StatusLabel       string    `json:"status_label"`
	CheckStatus       string    `json:"check_status"`
	CheckStatusLabel  string    `json:"check_status_label"`
	CheckError        string    `json:"check_error"`
	LastLatencyMS     int64     `json:"last_latency_ms"`
	LastCheckedAtText string    `json:"last_checked_at"`
	EditUUID          string    `json:"edit_uuid"`
	EditHost          string    `json:"edit_host"`
	EditPort          string    `json:"edit_port"`
	EditQuery         string    `json:"edit_query"`
	EditFragment      string    `json:"edit_fragment"`
	CreatedAt         time.Time `json:"created_at"`
}

// API request types

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateUserRequest struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	ActivationCode string `json:"activation_code"`
	Status         string `json:"status"`
	IssueDays      int    `json:"issue_days"`
	BlockedReason  string `json:"blocked_reason"`
}

type UpdateUserKeysRequest struct {
	KeyIDs []int64 `json:"key_ids"`
}

type UpdateSubscriptionRequest struct {
	Status        string `json:"status"`
	StartsAt      string `json:"starts_at"`
	ExpiresAt     string `json:"expires_at"`
	BlockedReason string `json:"blocked_reason"`
}

type UpdateHWIDRequest struct {
	MaxDevices int `json:"max_devices"`
}

type CreateKeyRequest struct {
	Label  string `json:"label"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

type UpdateKeyRequest struct {
	Label    string `json:"label"`
	Status   string `json:"status"`
	UUID     string `json:"uuid"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Query    string `json:"query"`
	Fragment string `json:"fragment"`
}

type ActivateRequest struct {
	ActivationCode string `json:"activation_code"`
}
