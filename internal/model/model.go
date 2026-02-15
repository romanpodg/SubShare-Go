package model

import (
	"strings"
	"time"
)

// Status constants for users.
const (
	UserStatusActive  = "active"
	UserStatusPaused  = "paused"
	UserStatusBlocked = "blocked"
)

// Status constants for keys.
const (
	KeyStatusActive    = "active"
	KeyStatusNonActive = "non-active"
)

// Kind constants for keys.
const (
	KeyKindReal          = "real"
	KeyKindInformational = "informational"
)

// DefaultDeviceLimitMessage is the fallback message when the HWID device limit is exceeded.
const DefaultDeviceLimitMessage = "You have reached the maximum number of allowed devices for your subscription"

// AdminSession represents an authenticated admin session.
type AdminSession struct {
	ExpiresAt time.Time
	CSRFToken string
}

// User represents a subscription user.
type User struct {
	ID                       int64             `json:"id"`
	Name                     string            `json:"name"`
	Email                    string            `json:"email"`
	ActivationCode           string            `json:"activation_code"`
	SubscriptionID           string            `json:"subscription_id"`
	SubscriptionName         string            `json:"subscription_name"`
	SubscriptionRefreshHours int               `json:"subscription_refresh_hours"`
	SubscriptionInfoURL      string            `json:"subscription_info_url"`
	SubscriptionExtraURL     string            `json:"subscription_extra_url"`
	SubscriptionExtraStatus  string            `json:"subscription_extra_status"`
	ActivationUsedAt         string            `json:"activation_used_at"`
	Status                   string            `json:"status"`
	StartsAtInput            string            `json:"starts_at"`
	ExpiresAtInput           string            `json:"expires_at"`
	BlockedReason            string            `json:"blocked_reason"`
	AssignedKeyIDs           string            `json:"assigned_key_ids"`
	MaxDevices               int               `json:"max_devices"`
	ConnectedDeviceCount     int               `json:"connected_device_count"`
	ConnectedHWIDs           []string          `json:"connected_hwids"`
	ConnectedDevices         []ConnectedDevice `json:"connected_devices"`
	CreatedAt                time.Time         `json:"created_at"`
}

// ConnectedDevice stores device metadata collected from subscription client requests.
type ConnectedDevice struct {
	HWID           string `json:"hwid"`
	NormalizedHWID string `json:"normalized_hwid"`
	DeviceName     string `json:"device_name"`
	DeviceModel    string `json:"device_model"`
	DeviceBrand    string `json:"device_brand"`
	Platform       string `json:"platform"`
	OSVersion      string `json:"os_version"`
	AppName        string `json:"app_name"`
	AppVersion     string `json:"app_version"`
	ClientApp      string `json:"client_app"`
	ClientVersion  string `json:"client_version"`
	UserAgent      string `json:"user_agent"`
	CreatedAt      string `json:"created_at"`
	LastSeenAt     string `json:"last_seen_at"`
}

// VLESSKey represents a VLESS server key.
type VLESSKey struct {
	ID                int64     `json:"id"`
	Label             string    `json:"label"`
	URL               string    `json:"url"`
	Kind              string    `json:"kind"`
	TemplateText      string    `json:"template_text"`
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

// LoginRequest is the payload for POST /api/auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreateUserRequest is the payload for POST /api/admin/users.
type CreateUserRequest struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	ActivationCode string `json:"activation_code"`
	Status         string `json:"status"`
	IssueDays      int    `json:"issue_days"`
	BlockedReason  string `json:"blocked_reason"`
}

// UpdateUserKeysRequest is the payload for PUT /api/admin/users/{id}/keys.
type UpdateUserKeysRequest struct {
	KeyIDs []int64 `json:"key_ids"`
}

// UpdateSubscriptionRequest is the payload for PUT /api/admin/users/{id}/subscription.
type UpdateSubscriptionRequest struct {
	Status                   string `json:"status"`
	StartsAt                 string `json:"starts_at"`
	ExpiresAt                string `json:"expires_at"`
	BlockedReason            string `json:"blocked_reason"`
	SubscriptionName         string `json:"subscription_name"`
	SubscriptionRefreshHours int    `json:"subscription_refresh_hours"`
	SubscriptionInfoURL      string `json:"subscription_info_url"`
	SubscriptionExtraURL     string `json:"subscription_extra_url"`
	SubscriptionExtraStatus  string `json:"subscription_extra_status"`
}

// UpdateHWIDRequest is the payload for PUT /api/admin/users/{id}/hwid.
type UpdateHWIDRequest struct {
	MaxDevices int `json:"max_devices"`
}

// CreateKeyRequest is the payload for POST /api/admin/keys.
type CreateKeyRequest struct {
	Label        string `json:"label"`
	URL          string `json:"url"`
	Status       string `json:"status"`
	Kind         string `json:"kind"`
	TemplateText string `json:"template_text"`
}

// UpdateKeyRequest is the payload for PUT /api/admin/keys/{id}.
type UpdateKeyRequest struct {
	Label        string `json:"label"`
	Status       string `json:"status"`
	UUID         string `json:"uuid"`
	Host         string `json:"host"`
	Port         string `json:"port"`
	Query        string `json:"query"`
	Fragment     string `json:"fragment"`
	Kind         string `json:"kind"`
	TemplateText string `json:"template_text"`
}

// ReorderKeysRequest is the payload for PUT /api/admin/keys/reorder.
type ReorderKeysRequest struct {
	IDs []int64 `json:"ids"`
}

// ActivateRequest is the payload for POST /api/subscription/activate.
type ActivateRequest struct {
	ActivationCode string `json:"activation_code"`
}

// SubscriptionSettings contains global subscription metadata shown to clients.
type SubscriptionSettings struct {
	Title        string `json:"title"`
	RefreshHours int    `json:"refresh_hours"`
	InfoURL      string `json:"info_url"`
	ExtraURL     string `json:"extra_url"`
	ExtraStatus  string `json:"extra_status"`
}

// UpdateSubscriptionSettingsRequest is the payload for PUT /api/admin/subscription-settings.
type UpdateSubscriptionSettingsRequest struct {
	Title        string `json:"title"`
	RefreshHours int    `json:"refresh_hours"`
	InfoURL      string `json:"info_url"`
	ExtraURL     string `json:"extra_url"`
	ExtraStatus  string `json:"extra_status"`
}

// NormalizeUserStatus validates and normalizes a user status string.
// It returns the normalized status and true if valid, or empty string and false otherwise.
func NormalizeUserStatus(raw string) (string, bool) {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		status = UserStatusActive
	}
	switch status {
	case UserStatusActive, UserStatusPaused, UserStatusBlocked:
		return status, true
	default:
		return "", false
	}
}

// NormalizeStoredStatus normalizes a status from the database, defaulting to active.
func NormalizeStoredStatus(raw string) string {
	status, ok := NormalizeUserStatus(raw)
	if !ok {
		return UserStatusActive
	}
	return status
}

// NormalizeKeyStatus validates and normalizes a key status string.
func NormalizeKeyStatus(raw string) (string, bool) {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		status = KeyStatusActive
	}
	if status == "blocked" {
		status = KeyStatusNonActive
	}
	switch status {
	case KeyStatusActive, KeyStatusNonActive:
		return status, true
	default:
		return "", false
	}
}

// KeyStatusLabel returns a human-readable label for a key status.
func KeyStatusLabel(status string) string {
	switch status {
	case KeyStatusActive:
		return "active"
	case KeyStatusNonActive:
		return "non-active"
	default:
		return "non-active"
	}
}

// NormalizeCheckStatus normalizes a health check status string.
func NormalizeCheckStatus(raw string) string {
	status := strings.ToLower(strings.TrimSpace(raw))
	switch status {
	case "up", "down", "unknown":
		return status
	default:
		return "unknown"
	}
}

// NormalizeKeyKind validates and normalizes key kind.
func NormalizeKeyKind(raw string) (string, bool) {
	kind := strings.ToLower(strings.TrimSpace(raw))
	if kind == "" {
		kind = KeyKindReal
	}
	switch kind {
	case KeyKindReal, KeyKindInformational:
		return kind, true
	default:
		return "", false
	}
}

// CheckStatusLabel returns a human-readable label for a check status (in Russian).
func CheckStatusLabel(status string) string {
	switch status {
	case "up":
		return "Доступен"
	case "down":
		return "Недоступен"
	default:
		return "Не проверен"
	}
}
