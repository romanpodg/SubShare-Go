# Next.js Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate Xray Sub frontend from Go server-rendered templates to a Next.js SPA with Tailwind CSS dark theme, converting Go to a pure JSON REST API.

**Architecture:** Go backend becomes a JSON API under `/api/`. Next.js App Router handles all UI in `frontend/` directory. Next.js proxies API calls to Go in development. In production, Go serves the exported Next.js static files.

**Tech Stack:** Go 1.22+ stdlib (method-based routing), Next.js 14+ (App Router), React 18, Tailwind CSS, TypeScript

---

## Phase 1: Go API Conversion

### Task 1: Update types.go — Add JSON tags and request types

**Files:**
- Modify: `cmd/server/types.go`

**Step 1: Rewrite types.go with JSON tags and API request/response types**

```go
package main

import (
	"database/sql"
	"sync"
	"time"
)

type App struct {
	db                 *sql.DB
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
```

**Step 2: Verify it compiles**

Run: `cd cmd/server && go build ./...` (will fail until handlers are updated — that's expected)

**Step 3: Commit**

```bash
git add cmd/server/types.go
git commit -m "refactor: add JSON tags and API request types to types.go"
```

---

### Task 2: Update auth.go — Header-based CSRF

**Files:**
- Modify: `cmd/server/auth.go`

**Step 1: Update requireAdmin to read CSRF from header and JSON body**

The key change: CSRF token is read from `X-CSRF-Token` header instead of form field. Also, since we're sending JSON bodies, we should NOT call `r.ParseForm()` in the middleware (that would consume the body). Instead, just read the header.

```go
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

func (a *App) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _, ok := a.adminSessionFromRequest(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		if isUnsafeHTTPMethod(r.Method) {
			csrfToken := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
			if !secureEqual(csrfToken, session.CSRFToken) {
				writeJSON(w, http.StatusForbidden, map[string]any{"error": "invalid csrf token"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) isAdminAuthenticated(r *http.Request) bool {
	_, _, ok := a.adminSessionFromRequest(r)
	return ok
}

func (a *App) adminSessionFromRequest(r *http.Request) (AdminSession, string, bool) {
	cookie, err := r.Cookie("xary_admin_session")
	if err != nil || cookie.Value == "" {
		return AdminSession{}, "", false
	}

	a.mu.RLock()
	session, ok := a.sessions[cookie.Value]
	a.mu.RUnlock()
	if !ok {
		return AdminSession{}, "", false
	}
	if time.Now().After(session.ExpiresAt) {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
		return AdminSession{}, "", false
	}
	return session, cookie.Value, true
}

func (a *App) mustCSRFToken(r *http.Request) string {
	session, _, ok := a.adminSessionFromRequest(r)
	if !ok {
		return ""
	}
	return session.CSRFToken
}

func isUnsafeHTTPMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

func secureEqual(left, right string) bool {
	leftHash := sha256.Sum256([]byte(left))
	rightHash := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftHash[:], rightHash[:]) == 1
}

func generateToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
```

**Step 2: Commit**

```bash
git add cmd/server/auth.go
git commit -m "refactor: switch CSRF validation to X-CSRF-Token header"
```

---

### Task 3: Rewrite handlers.go — Complete JSON API

**Files:**
- Modify: `cmd/server/handlers.go`

**Step 1: Rewrite handlers.go with all JSON API handlers**

This is the largest change. Replace the entire file. The `handleSubscription` handler (for `/sub/{id}`) stays mostly the same. Everything else becomes JSON API.

```go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func writeMessage(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusOK, map[string]any{"message": msg})
}

// pathID extracts an int64 from a path value. Returns 0 and writes error if invalid.
func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

// --- Auth API ---

func (a *App) apiLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if !secureEqual(username, a.adminUser) || !secureEqual(password, a.adminPass) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	sessionID, err := generateToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	csrfToken, err := generateToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	a.mu.Lock()
	a.sessions[sessionID] = AdminSession{
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CSRFToken: csrfToken,
	}
	a.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "xary_admin_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((24 * time.Hour).Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{"csrf_token": csrfToken})
}

func (a *App) apiLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("xary_admin_session")
	if err == nil && cookie.Value != "" {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "xary_admin_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	writeMessage(w, "logged out")
}

func (a *App) apiMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"csrf_token":    a.mustCSRFToken(r),
	})
}

// --- Users API ---

func (a *App) apiListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.listUsers()
	if err != nil {
		log.Printf("apiListUsers: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []User{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (a *App) apiCreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	activationCode := strings.TrimSpace(req.ActivationCode)
	status, ok := normalizeUserStatus(req.Status)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid subscription status")
		return
	}

	issueDays := req.IssueDays
	if issueDays <= 0 {
		issueDays = 30
	}
	if issueDays > 3650 {
		writeError(w, http.StatusBadRequest, "issue days must be between 1 and 3650")
		return
	}

	blockedReason := strings.TrimSpace(req.BlockedReason)
	if status != userStatusBlocked {
		blockedReason = ""
	}

	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if activationCode == "" || strings.Contains(activationCode, "/") {
		writeError(w, http.StatusBadRequest, "activation code is required")
		return
	}

	legacyToken, err := generateToken(24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate user token")
		return
	}

	now := time.Now().UTC()
	expiresAt := now.AddDate(0, 0, issueDays)

	_, err = a.db.Exec(
		`INSERT INTO users(name, email, token, activation_code, status, starts_at, expires_at, blocked_reason, max_devices) VALUES(?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		name, email, legacyToken, activationCode, status, now, expiresAt, blockedReason,
	)
	if err != nil {
		writeError(w, http.StatusConflict, "failed to create user (check activation code uniqueness)")
		return
	}
	writeMessage(w, "user created")
}

func (a *App) apiDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := a.db.Exec(`DELETE FROM users WHERE id = ?`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	writeMessage(w, "user deleted")
}

func (a *App) apiUpdateUserKeys(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req UpdateUserKeysRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user keys")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM user_keys WHERE user_id = ?`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user keys")
		return
	}

	seen := make(map[int64]struct{}, len(req.KeyIDs))
	for _, keyID := range req.KeyIDs {
		if keyID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid key id")
			return
		}
		if _, dup := seen[keyID]; dup {
			continue
		}
		seen[keyID] = struct{}{}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO user_keys(user_id, key_id)
			 SELECT ?, id FROM vless_keys WHERE id = ? AND LOWER(COALESCE(NULLIF(TRIM(status), ''), 'active')) = 'active'`,
			id, keyID,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update user keys")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user keys")
		return
	}
	writeMessage(w, "subscription keys updated")
}

func (a *App) apiUpdateUserSubscription(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req UpdateSubscriptionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	status, ok := normalizeUserStatus(req.Status)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid subscription status")
		return
	}

	startsAt, err := parseOptionalDateTimeLocal(req.StartsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid starts_at datetime")
		return
	}
	expiresAt, err := parseOptionalDateTimeLocal(req.ExpiresAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid expires_at datetime")
		return
	}

	if startsAt.Valid && expiresAt.Valid && startsAt.Time.After(expiresAt.Time) {
		writeError(w, http.StatusBadRequest, "starts_at must be before expires_at")
		return
	}

	blockedReason := strings.TrimSpace(req.BlockedReason)
	if status != userStatusBlocked {
		blockedReason = ""
	}

	_, err = a.db.Exec(
		`UPDATE users SET status = ?, starts_at = ?, expires_at = ?, blocked_reason = ? WHERE id = ?`,
		status, nullTimeValue(startsAt), nullTimeValue(expiresAt), nullStringValue(blockedReason), id,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}
	writeMessage(w, "subscription updated")
}

func (a *App) apiUpdateUserHWID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req UpdateHWIDRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MaxDevices <= 0 || req.MaxDevices > 32 {
		writeError(w, http.StatusBadRequest, "max_devices must be between 1 and 32")
		return
	}

	if _, err := a.db.Exec(`UPDATE users SET max_devices = ? WHERE id = ?`, req.MaxDevices, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update hwid settings")
		return
	}
	writeMessage(w, "hwid settings updated")
}

func (a *App) apiDeleteUserHWID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	hwid := r.PathValue("hwid")
	if strings.TrimSpace(hwid) == "" {
		writeError(w, http.StatusBadRequest, "hwid is required")
		return
	}

	if _, err := a.db.Exec(`DELETE FROM user_devices WHERE user_id = ? AND hwid = ?`, id, hwid); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete hwid")
		return
	}
	writeMessage(w, "hwid removed")
}

// --- Keys API ---

func (a *App) apiListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := a.listKeys()
	if err != nil {
		log.Printf("apiListKeys: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list keys")
		return
	}
	if keys == nil {
		keys = []VLESSKey{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (a *App) apiCreateKey(w http.ResponseWriter, r *http.Request) {
	var req CreateKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	label := strings.TrimSpace(req.Label)
	keyURL := strings.TrimSpace(req.URL)
	status, ok := normalizeKeyStatus(req.Status)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid key status")
		return
	}
	if label == "" || keyURL == "" {
		writeError(w, http.StatusBadRequest, "label and url are required")
		return
	}
	if !strings.HasPrefix(strings.ToLower(keyURL), "vless://") {
		writeError(w, http.StatusBadRequest, "url must start with vless://")
		return
	}

	if _, err := a.db.Exec(
		`INSERT INTO vless_keys(label, url, status, check_status) VALUES(?, ?, ?, 'unknown')`,
		label, keyURL, status,
	); err != nil {
		writeError(w, http.StatusConflict, "failed to add key (maybe duplicate)")
		return
	}
	writeMessage(w, "key added")
}

func (a *App) apiUpdateKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req UpdateKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	label := strings.TrimSpace(req.Label)
	status, ok := normalizeKeyStatus(req.Status)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid key status")
		return
	}
	if label == "" {
		writeError(w, http.StatusBadRequest, "label is required")
		return
	}

	builtURL, err := buildVLESSURL(req.UUID, req.Host, req.Port, req.Query, req.Fragment)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := a.db.Exec(
		`UPDATE vless_keys SET label = ?, url = ?, status = ? WHERE id = ?`,
		label, builtURL, status, id,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update key")
		return
	}
	writeMessage(w, "key updated")
}

func (a *App) apiDeleteKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := a.db.Exec(`DELETE FROM vless_keys WHERE id = ?`, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete key")
		return
	}
	writeMessage(w, "key deleted")
}

func (a *App) apiCheckKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var rawURL string
	if err := a.db.QueryRow(`SELECT url FROM vless_keys WHERE id = ?`, id).Scan(&rawURL); err != nil {
		writeError(w, http.StatusNotFound, "key not found")
		return
	}

	if err := a.checkAndPersistKey(id, rawURL); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check key")
		return
	}
	a.respondJSONKeyCheck(w, id)
}

func (a *App) apiCheckAllKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT id, url FROM vless_keys ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load keys")
		return
	}
	defer rows.Close()

	type keyRow struct {
		id  int64
		url string
	}
	keys := make([]keyRow, 0)
	for rows.Next() {
		var row keyRow
		if err := rows.Scan(&row.id, &row.url); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read keys")
			return
		}
		keys = append(keys, row)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read keys")
		return
	}

	checked := 0
	for _, key := range keys {
		if err := a.checkAndPersistKey(key.id, key.url); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check keys")
			return
		}
		checked++
	}

	updatedRows, err := a.db.Query(`
		SELECT id, check_status, check_error, last_checked_at, last_latency_ms
		FROM vless_keys ORDER BY id
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load key checks")
		return
	}
	defer updatedRows.Close()

	results := make([]map[string]any, 0, checked)
	for updatedRows.Next() {
		var id int64
		var checkStatus, checkError sql.NullString
		var lastCheckedAt sql.NullTime
		var latency sql.NullInt64
		if err := updatedRows.Scan(&id, &checkStatus, &checkError, &lastCheckedAt, &latency); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load key checks")
			return
		}
		payload := map[string]any{
			"id":                 id,
			"check_status":       normalizeCheckStatus(checkStatus.String),
			"check_status_label": checkStatusLabel(normalizeCheckStatus(checkStatus.String)),
			"check_error":        strings.TrimSpace(checkError.String),
			"last_checked_at":    "",
			"last_latency_ms":    int64(0),
		}
		if lastCheckedAt.Valid {
			payload["last_checked_at"] = lastCheckedAt.Time.Local().Format("2006-01-02 15:04:05")
		}
		if latency.Valid {
			payload["last_latency_ms"] = latency.Int64
		}
		results = append(results, payload)
	}
	if err := updatedRows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load key checks")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checked": checked, "keys": results})
}

// --- Subscription API ---

func (a *App) apiActivateSubscription(w http.ResponseWriter, r *http.Request) {
	var req ActivateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	activationCode := strings.TrimSpace(req.ActivationCode)
	if activationCode == "" || strings.Contains(activationCode, "/") {
		writeError(w, http.StatusBadRequest, "Введите корректный ключ активации")
		return
	}

	subscriptionID, code, reason, err := a.redeemActivationCode(activationCode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to activate subscription")
		return
	}
	if code != http.StatusOK {
		if strings.TrimSpace(reason) == "" {
			reason = "Не удалось активировать подписку"
		}
		writeError(w, code, reason)
		return
	}

	subscriptionURL := fmt.Sprintf("%s/sub/%s", resolveBaseURL(r), subscriptionID)
	writeJSON(w, http.StatusOK, map[string]any{
		"subscription_url": subscriptionURL,
		"message":          "Ключ активирован. Ссылка готова — скопируйте и вставьте её в VPN-клиент",
	})
}

// --- Subscription delivery (unchanged — serves plaintext for VPN clients) ---

func (a *App) handleSubscription(w http.ResponseWriter, r *http.Request) {
	subscriptionID := r.PathValue("subscription_id")
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}

	allowed, userID, code, reason, err := a.subscriptionAccessAllowed(subscriptionID)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, reason, code)
		return
	}

	hwid := strings.TrimSpace(r.URL.Query().Get("hwid"))
	if hwid == "" {
		hwid = strings.TrimSpace(r.Header.Get("X-HWID"))
	}
	if hwid == "" {
		hwid = strings.TrimSpace(r.Header.Get("X-Device-ID"))
	}
	if hwid != "" {
		allowedDevice, err := a.registerHWID(userID, hwid)
		if err != nil {
			http.Error(w, "failed to validate hwid", http.StatusInternalServerError)
			return
		}
		if !allowedDevice {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			message := strings.TrimSpace(a.deviceLimitMessage)
			if message == "" {
				message = defaultDeviceLimitMessage
			}
			headerMessage := strings.ReplaceAll(strings.ReplaceAll(message, "\r", " "), "\n", " ")
			if headerMessage == "" {
				headerMessage = defaultDeviceLimitMessage
			}
			w.Header().Set("X-Device-Limit-Message", headerMessage)
			_, _ = w.Write([]byte("# " + message + "\n" + message + "\n"))
			return
		}
	}

	rows, err := a.db.Query(`
		SELECT k.url
		FROM users u
		JOIN user_keys uk ON uk.user_id = u.id
		JOIN vless_keys k ON k.id = uk.key_id
		WHERE u.subscription_id = ?
		  AND LOWER(COALESCE(NULLIF(TRIM(k.status), ''), 'active')) = 'active'
		  AND k.check_status != 'down'
		ORDER BY k.id
	`, subscriptionID)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var keyURL string
		if err := rows.Scan(&keyURL); err != nil {
			http.Error(w, "failed to read subscription", http.StatusInternalServerError)
			return
		}
		lines = append(lines, keyURL)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "failed to read subscription", http.StatusInternalServerError)
		return
	}
	if len(lines) == 0 {
		http.Error(w, "subscription has no available keys", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(strings.Join(lines, "\n")))
}

// --- JSON key check response helper ---

func (a *App) respondJSONKeyCheck(w http.ResponseWriter, keyID int64) {
	var checkStatus, checkError sql.NullString
	var lastCheckedAt sql.NullTime
	var latency sql.NullInt64
	if err := a.db.QueryRow(
		`SELECT check_status, check_error, last_checked_at, last_latency_ms FROM vless_keys WHERE id = ?`,
		keyID,
	).Scan(&checkStatus, &checkError, &lastCheckedAt, &latency); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load key check")
		return
	}
	status := normalizeCheckStatus(checkStatus.String)
	payload := map[string]any{
		"id":                 keyID,
		"check_status":       status,
		"check_status_label": checkStatusLabel(status),
		"check_error":        strings.TrimSpace(checkError.String),
		"last_checked_at":    "",
		"last_latency_ms":    int64(0),
	}
	if lastCheckedAt.Valid {
		payload["last_checked_at"] = lastCheckedAt.Time.Local().Format("2006-01-02 15:04:05")
	}
	if latency.Valid {
		payload["last_latency_ms"] = latency.Int64
	}
	writeJSON(w, http.StatusOK, payload)
}
```

**Step 2: Commit**

```bash
git add cmd/server/handlers.go
git commit -m "refactor: convert all handlers to JSON API endpoints"
```

---

### Task 4: Rewrite main.go — New routes, remove templates

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Rewrite main.go**

Remove template loading and static file serving. Add new API routes using Go 1.22 method-based patterns. Add optional static file serving for production (serves `frontend/out/`).

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := os.MkdirAll("data", 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", filepath.ToSlash("data/app.db"))
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("set foreign_keys pragma: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		return fmt.Errorf("set busy_timeout pragma: %w", err)
	}

	if err := migrate(db); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}

	adminUser := strings.TrimSpace(os.Getenv("ADMIN_USER"))
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPass := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	if adminPass == "" {
		log.Fatal("ADMIN_PASSWORD environment variable is required but not set")
	}

	deviceLimitMessage := strings.TrimSpace(os.Getenv("DEVICE_LIMIT_MESSAGE"))
	if deviceLimitMessage == "" {
		deviceLimitMessage = defaultDeviceLimitMessage
	}

	app := &App{
		db:                 db,
		adminUser:          adminUser,
		adminPass:          adminPass,
		deviceLimitMessage: deviceLimitMessage,
		sessions:           make(map[string]AdminSession),
	}

	mux := http.NewServeMux()

	// Auth API
	mux.HandleFunc("POST /api/auth/login", app.apiLogin)
	mux.Handle("POST /api/auth/logout", app.requireAdmin(http.HandlerFunc(app.apiLogout)))
	mux.Handle("GET /api/auth/me", app.requireAdmin(http.HandlerFunc(app.apiMe)))

	// Users API
	mux.Handle("GET /api/admin/users", app.requireAdmin(http.HandlerFunc(app.apiListUsers)))
	mux.Handle("POST /api/admin/users", app.requireAdmin(http.HandlerFunc(app.apiCreateUser)))
	mux.Handle("DELETE /api/admin/users/{id}", app.requireAdmin(http.HandlerFunc(app.apiDeleteUser)))
	mux.Handle("PUT /api/admin/users/{id}/keys", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserKeys)))
	mux.Handle("PUT /api/admin/users/{id}/subscription", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserSubscription)))
	mux.Handle("PUT /api/admin/users/{id}/hwid", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserHWID)))
	mux.Handle("DELETE /api/admin/users/{id}/hwid/{hwid}", app.requireAdmin(http.HandlerFunc(app.apiDeleteUserHWID)))

	// Keys API
	mux.Handle("GET /api/admin/keys", app.requireAdmin(http.HandlerFunc(app.apiListKeys)))
	mux.Handle("POST /api/admin/keys", app.requireAdmin(http.HandlerFunc(app.apiCreateKey)))
	mux.Handle("PUT /api/admin/keys/{id}", app.requireAdmin(http.HandlerFunc(app.apiUpdateKey)))
	mux.Handle("DELETE /api/admin/keys/{id}", app.requireAdmin(http.HandlerFunc(app.apiDeleteKey)))
	mux.Handle("POST /api/admin/keys/{id}/check", app.requireAdmin(http.HandlerFunc(app.apiCheckKey)))
	mux.Handle("POST /api/admin/keys/check-all", app.requireAdmin(http.HandlerFunc(app.apiCheckAllKeys)))

	// Subscription API
	mux.HandleFunc("POST /api/subscription/activate", app.apiActivateSubscription)

	// Subscription delivery (VPN clients hit this directly — unchanged)
	mux.HandleFunc("GET /sub/{subscription_id}", app.handleSubscription)

	// Serve frontend static files in production (if frontend/out exists)
	frontendDir := "frontend/out"
	if info, err := os.Stat(frontendDir); err == nil && info.IsDir() {
		log.Printf("Serving frontend from %s", frontendDir)
		mux.Handle("/", spaFileServer(os.DirFS(frontendDir)))
	}

	addr := ":8080"

	srv := &http.Server{
		Addr:              addr,
		Handler:           logRequest(mux),
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}()

	log.Printf("Server listening on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

// spaFileServer serves static files and falls back to index.html for client-side routing.
func spaFileServer(root fs.FS) http.Handler {
	fileServer := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		// Try to open the file
		f, err := root.Open(path)
		if err != nil {
			// File not found — serve index.html for SPA routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
```

**Step 2: Verify it compiles**

Run: `cd E:/LeetCode/vless && go build -o server ./cmd/server`

Expected: Successful build with no errors.

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "refactor: update routing to JSON API, add SPA file server"
```

---

### Task 5: Verify Go API compiles and runs

**Files:** None (verification only)

**Step 1: Build**

```bash
cd E:/LeetCode/vless && go build -o server ./cmd/server
```

**Step 2: Quick smoke test**

```bash
ADMIN_PASSWORD=admin ./server &
# Test login
curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' -c cookies.txt
# Test auth check
curl -s http://localhost:8080/api/auth/me -b cookies.txt
# Kill server
kill %1 && rm -f cookies.txt server
```

Expected: Login returns `{"csrf_token":"..."}`, me returns `{"authenticated":true,"csrf_token":"..."}`.

**Step 3: Commit (if any fixes needed)**

---

## Phase 2: Next.js Project Setup

### Task 6: Initialize Next.js project with Tailwind CSS

**Files:**
- Create: `frontend/` directory with Next.js scaffolding

**Step 1: Initialize the project**

```bash
cd E:/LeetCode/vless
npx create-next-app@latest frontend --typescript --tailwind --eslint --app --src-dir --no-import-alias --use-npm
```

Answer prompts: No to Turbopack (default).

**Step 2: Clean up generated boilerplate**

Remove default page content, global styles placeholder, and favicon references. Keep the generated structure intact.

**Step 3: Commit**

```bash
git add frontend/
git commit -m "feat: initialize Next.js project with Tailwind CSS"
```

---

### Task 7: Configure Tailwind theme and Next.js proxy

**Files:**
- Modify: `frontend/tailwind.config.ts`
- Modify: `frontend/next.config.ts`
- Modify: `frontend/src/app/globals.css`

**Step 1: Configure Tailwind dark theme colors**

`frontend/tailwind.config.ts`:

```ts
import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./src/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        bg: "#0a0a0f",
        surface: {
          1: "#13131a",
          2: "#1a1a24",
        },
        border: "rgba(255, 255, 255, 0.04)",
        accent: {
          DEFAULT: "#6366f1",
          hover: "#818cf8",
        },
        success: "#22c55e",
        danger: "#ef4444",
        warning: "#f59e0b",
      },
    },
  },
  plugins: [],
};
export default config;
```

**Step 2: Configure Next.js API proxy and static export**

`frontend/next.config.ts`:

```ts
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8080/api/:path*",
      },
      {
        source: "/sub/:path*",
        destination: "http://localhost:8080/sub/:path*",
      },
    ];
  },
};

export default nextConfig;
```

**Important note for the implementer:** `output: "export"` and `rewrites` are incompatible in Next.js. Use TWO configs:
- For development: remove `output: "export"`, keep `rewrites`
- For production build: use `output: "export"`, remove `rewrites`

Simplest approach: use an environment variable:

```ts
import type { NextConfig } from "next";

const isProd = process.env.NODE_ENV === "production";

const nextConfig: NextConfig = {
  ...(isProd ? { output: "export" } : {}),
  ...(!isProd
    ? {
        async rewrites() {
          return [
            { source: "/api/:path*", destination: "http://localhost:8080/api/:path*" },
            { source: "/sub/:path*", destination: "http://localhost:8080/sub/:path*" },
          ];
        },
      }
    : {}),
};

export default nextConfig;
```

**Step 3: Set up global CSS**

`frontend/src/app/globals.css`:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

body {
  background-color: #0a0a0f;
  color: #e4e4e7;
}
```

**Step 4: Commit**

```bash
git add frontend/tailwind.config.ts frontend/next.config.ts frontend/src/app/globals.css
git commit -m "feat: configure Tailwind dark theme and API proxy"
```

---

### Task 8: Create API client, TypeScript types, and auth hook

**Files:**
- Create: `frontend/src/lib/types.ts`
- Create: `frontend/src/lib/api.ts`
- Create: `frontend/src/hooks/useAuth.ts`

**Step 1: TypeScript types**

`frontend/src/lib/types.ts`:

```ts
export interface User {
  id: number;
  name: string;
  email: string;
  activation_code: string;
  subscription_id: string;
  activation_used_at: string;
  status: "active" | "paused" | "blocked";
  starts_at: string;
  expires_at: string;
  blocked_reason: string;
  assigned_key_ids: string;
  max_devices: number;
  connected_device_count: number;
  connected_hwids: string[];
  created_at: string;
}

export interface VLESSKey {
  id: number;
  label: string;
  url: string;
  url_short: string;
  status: "active" | "non-active";
  status_label: string;
  check_status: "up" | "down" | "unknown";
  check_status_label: string;
  check_error: string;
  last_latency_ms: number;
  last_checked_at: string;
  edit_uuid: string;
  edit_host: string;
  edit_port: string;
  edit_query: string;
  edit_fragment: string;
  created_at: string;
}

export interface KeyCheckResult {
  id: number;
  check_status: string;
  check_status_label: string;
  check_error: string;
  last_checked_at: string;
  last_latency_ms: number;
}
```

**Step 2: API client**

`frontend/src/lib/api.ts`:

```ts
let csrfToken: string | null = null;

export function setCsrfToken(token: string) {
  csrfToken = token;
}

export function getCsrfToken(): string | null {
  return csrfToken;
}

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};

  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (csrfToken && method !== "GET") {
    headers["X-CSRF-Token"] = csrfToken;
  }

  const res = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, data.error || res.statusText);
  }

  return res.json();
}

// Auth
export const auth = {
  login: (username: string, password: string) =>
    request<{ csrf_token: string }>("POST", "/api/auth/login", { username, password }),
  logout: () => request<{ message: string }>("POST", "/api/auth/logout"),
  me: () => request<{ authenticated: boolean; csrf_token: string }>("GET", "/api/auth/me"),
};

// Users
export const users = {
  list: () => request<{ users: import("./types").User[] }>("GET", "/api/admin/users"),
  create: (data: {
    name: string;
    email: string;
    activation_code: string;
    status: string;
    issue_days: number;
    blocked_reason?: string;
  }) => request<{ message: string }>("POST", "/api/admin/users", data),
  delete: (id: number) => request<{ message: string }>("DELETE", `/api/admin/users/${id}`),
  updateKeys: (id: number, key_ids: number[]) =>
    request<{ message: string }>("PUT", `/api/admin/users/${id}/keys`, { key_ids }),
  updateSubscription: (id: number, data: {
    status: string;
    starts_at: string;
    expires_at: string;
    blocked_reason?: string;
  }) => request<{ message: string }>("PUT", `/api/admin/users/${id}/subscription`, data),
  updateHwid: (id: number, max_devices: number) =>
    request<{ message: string }>("PUT", `/api/admin/users/${id}/hwid`, { max_devices }),
  deleteHwid: (id: number, hwid: string) =>
    request<{ message: string }>("DELETE", `/api/admin/users/${id}/hwid/${encodeURIComponent(hwid)}`),
};

// Keys
export const keys = {
  list: () => request<{ keys: import("./types").VLESSKey[] }>("GET", "/api/admin/keys"),
  create: (data: { label: string; url: string; status: string }) =>
    request<{ message: string }>("POST", "/api/admin/keys", data),
  update: (id: number, data: {
    label: string;
    status: string;
    uuid: string;
    host: string;
    port: string;
    query: string;
    fragment: string;
  }) => request<{ message: string }>("PUT", `/api/admin/keys/${id}`, data),
  delete: (id: number) => request<{ message: string }>("DELETE", `/api/admin/keys/${id}`),
  check: (id: number) =>
    request<import("./types").KeyCheckResult>("POST", `/api/admin/keys/${id}/check`),
  checkAll: () =>
    request<{ checked: number; keys: import("./types").KeyCheckResult[] }>("POST", "/api/admin/keys/check-all"),
};

// Subscription
export const subscription = {
  activate: (activation_code: string) =>
    request<{ subscription_url: string; message: string }>("POST", "/api/subscription/activate", { activation_code }),
};
```

**Step 3: Auth hook**

`frontend/src/hooks/useAuth.ts`:

```ts
"use client";

import { useState, useEffect, useCallback } from "react";
import { auth, setCsrfToken } from "@/lib/api";

export function useAuth() {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(true);

  const checkAuth = useCallback(async () => {
    try {
      const data = await auth.me();
      setCsrfToken(data.csrf_token);
      setAuthenticated(true);
    } catch {
      setAuthenticated(false);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  const login = async (username: string, password: string) => {
    const data = await auth.login(username, password);
    setCsrfToken(data.csrf_token);
    setAuthenticated(true);
  };

  const logout = async () => {
    await auth.logout();
    setAuthenticated(false);
  };

  return { authenticated, loading, login, logout, checkAuth };
}
```

**Step 4: Commit**

```bash
git add frontend/src/lib/ frontend/src/hooks/
git commit -m "feat: add API client, TypeScript types, and auth hook"
```

---

## Phase 3: Frontend UI Components

### Task 9: Create base UI components

**Files:**
- Create: `frontend/src/components/ui/Button.tsx`
- Create: `frontend/src/components/ui/Input.tsx`
- Create: `frontend/src/components/ui/Card.tsx`
- Create: `frontend/src/components/ui/Modal.tsx`
- Create: `frontend/src/components/ui/Toast.tsx`

**Step 1: Button component**

`frontend/src/components/ui/Button.tsx`:

```tsx
"use client";

import { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "ghost" | "danger";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  loading?: boolean;
}

const variants: Record<Variant, string> = {
  primary: "bg-accent hover:bg-accent-hover text-white",
  ghost: "bg-transparent hover:bg-surface-2 text-zinc-300",
  danger: "bg-red-500/10 hover:bg-red-500/20 text-red-400",
};

export function Button({ variant = "primary", loading, className = "", children, disabled, ...props }: ButtonProps) {
  return (
    <button
      className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${variants[variant]} ${className}`}
      disabled={disabled || loading}
      {...props}
    >
      {loading ? "..." : children}
    </button>
  );
}
```

**Step 2: Input component**

`frontend/src/components/ui/Input.tsx`:

```tsx
"use client";

import { InputHTMLAttributes } from "react";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

export function Input({ label, error, className = "", id, ...props }: InputProps) {
  const inputId = id || label?.toLowerCase().replace(/\s+/g, "-");
  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label htmlFor={inputId} className="text-sm text-zinc-400">
          {label}
        </label>
      )}
      <input
        id={inputId}
        className={`bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-600 focus:outline-none focus:ring-1 focus:ring-accent ${error ? "ring-1 ring-red-500" : ""} ${className}`}
        {...props}
      />
      {error && <p className="text-xs text-red-400">{error}</p>}
    </div>
  );
}
```

**Step 3: Card component**

`frontend/src/components/ui/Card.tsx`:

```tsx
import { ReactNode } from "react";

interface CardProps {
  children: ReactNode;
  className?: string;
}

export function Card({ children, className = "" }: CardProps) {
  return (
    <div className={`bg-surface-1 border border-border rounded-xl p-6 ${className}`}>
      {children}
    </div>
  );
}
```

**Step 4: Modal component**

`frontend/src/components/ui/Modal.tsx`:

```tsx
"use client";

import { ReactNode, useEffect, useRef } from "react";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
}

export function Modal({ open, onClose, title, children }: ModalProps) {
  const overlayRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handleEsc = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleEsc);
    return () => document.removeEventListener("keydown", handleEsc);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      onClick={(e) => e.target === overlayRef.current && onClose()}
    >
      <div className="bg-surface-1 border border-border rounded-xl p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-zinc-200">{title}</h2>
          <button onClick={onClose} className="text-zinc-500 hover:text-zinc-300 text-xl leading-none">
            &times;
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}
```

**Step 5: Toast component**

`frontend/src/components/ui/Toast.tsx`:

```tsx
"use client";

import { createContext, useContext, useState, useCallback, ReactNode } from "react";

type ToastType = "success" | "error" | "info";

interface Toast {
  id: number;
  message: string;
  type: ToastType;
}

interface ToastContextType {
  toast: (message: string, type?: ToastType) => void;
}

const ToastContext = createContext<ToastContextType>({ toast: () => {} });

export function useToast() {
  return useContext(ToastContext);
}

let nextId = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const toast = useCallback((message: string, type: ToastType = "info") => {
    const id = nextId++;
    setToasts((prev) => [...prev, { id, message, type }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 4000);
  }, []);

  const colors: Record<ToastType, string> = {
    success: "bg-green-500/10 border-green-500/20 text-green-400",
    error: "bg-red-500/10 border-red-500/20 text-red-400",
    info: "bg-accent/10 border-accent/20 text-indigo-300",
  };

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`px-4 py-3 rounded-lg border text-sm animate-fade-in ${colors[t.type]}`}
          >
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
```

Add the fade-in animation to `tailwind.config.ts` under `extend`:

```ts
keyframes: {
  "fade-in": {
    "0%": { opacity: "0", transform: "translateY(8px)" },
    "100%": { opacity: "1", transform: "translateY(0)" },
  },
},
animation: {
  "fade-in": "fade-in 0.2s ease-out",
},
```

**Step 6: Commit**

```bash
git add frontend/src/components/ui/ frontend/tailwind.config.ts
git commit -m "feat: add base UI components (Button, Input, Card, Modal, Toast)"
```

---

### Task 10: Create root layout

**Files:**
- Modify: `frontend/src/app/layout.tsx`
- Modify: `frontend/src/app/page.tsx`

**Step 1: Root layout with ToastProvider**

`frontend/src/app/layout.tsx`:

```tsx
import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { ToastProvider } from "@/components/ui/Toast";
import "./globals.css";

const inter = Inter({ subsets: ["latin", "cyrillic"] });

export const metadata: Metadata = {
  title: "Xray Sub",
  description: "VLESS subscription management",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ru" className="dark">
      <body className={`${inter.className} bg-bg text-zinc-200 min-h-screen`}>
        <ToastProvider>{children}</ToastProvider>
      </body>
    </html>
  );
}
```

**Step 2: Root page — redirect to /subscription**

`frontend/src/app/page.tsx`:

```tsx
import { redirect } from "next/navigation";

export default function Home() {
  redirect("/subscription");
}
```

**Step 3: Commit**

```bash
git add frontend/src/app/layout.tsx frontend/src/app/page.tsx
git commit -m "feat: add root layout with dark theme and toast provider"
```

---

## Phase 4: Frontend Pages

### Task 11: Build login page

**Files:**
- Create: `frontend/src/app/admin/login/page.tsx`

**Step 1: Login page**

```tsx
"use client";

import { useState, FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { auth, setCsrfToken } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const data = await auth.login(username, password);
      setCsrfToken(data.csrf_token);
      router.push("/admin");
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <h1 className="text-xl font-semibold mb-6 text-center">Xray Sub</h1>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Input
            label="Логин"
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus
            required
          />
          <Input
            label="Пароль"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
          {error && <p className="text-sm text-red-400">{error}</p>}
          <Button type="submit" loading={loading}>
            Войти
          </Button>
        </form>
      </Card>
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add frontend/src/app/admin/login/
git commit -m "feat: add admin login page"
```

---

### Task 12: Build subscription activation page

**Files:**
- Create: `frontend/src/app/subscription/page.tsx`

**Step 1: Subscription page**

```tsx
"use client";

import { useState, FormEvent } from "react";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { subscription } from "@/lib/api";

export default function SubscriptionPage() {
  const [code, setCode] = useState("");
  const [subscriptionUrl, setSubscriptionUrl] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setMessage("");
    setSubscriptionUrl("");
    setLoading(true);
    try {
      const data = await subscription.activate(code);
      setSubscriptionUrl(data.subscription_url);
      setMessage(data.message);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Activation failed");
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = async () => {
    await navigator.clipboard.writeText(subscriptionUrl);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <h1 className="text-xl font-semibold mb-6 text-center">VPN-подписка</h1>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Input
            label="Ключ активации"
            type="text"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder="Введите ключ активации"
            autoFocus
            required
          />
          {error && <p className="text-sm text-red-400">{error}</p>}
          <Button type="submit" loading={loading}>
            Активировать и получить ссылку
          </Button>
        </form>

        {subscriptionUrl && (
          <div className="mt-6">
            <p className="text-sm text-green-400 mb-3">{message}</p>
            <div className="bg-surface-2 rounded-lg p-3 flex items-center gap-2">
              <code className="text-xs text-zinc-300 flex-1 break-all font-mono">
                {subscriptionUrl}
              </code>
              <Button variant="ghost" onClick={handleCopy} className="shrink-0 text-xs">
                {copied ? "Скопировано" : "Копировать"}
              </Button>
            </div>
          </div>
        )}

        <div className="mt-6 text-center">
          <a href="/admin/login" className="text-sm text-zinc-500 hover:text-zinc-300 transition-colors">
            Панель управления
          </a>
        </div>
      </Card>
    </div>
  );
}
```

**Step 2: Commit**

```bash
git add frontend/src/app/subscription/
git commit -m "feat: add subscription activation page"
```

---

### Task 13: Build admin page layout and users section

**Files:**
- Create: `frontend/src/app/admin/page.tsx`
- Create: `frontend/src/components/admin/UsersSection.tsx`
- Create: `frontend/src/components/admin/AddUserModal.tsx`
- Create: `frontend/src/components/admin/EditSubscriptionModal.tsx`
- Create: `frontend/src/components/admin/HwidManager.tsx`

**Step 1: Admin page — shell with auth guard, data fetching, section layout**

`frontend/src/app/admin/page.tsx`:

```tsx
"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/useAuth";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi, keys as keysApi } from "@/lib/api";
import { User, VLESSKey } from "@/lib/types";
import { UsersSection } from "@/components/admin/UsersSection";
import { KeysSection } from "@/components/admin/KeysSection";

export default function AdminPage() {
  const router = useRouter();
  const { authenticated, loading: authLoading, logout } = useAuth();
  const { toast } = useToast();
  const [usersList, setUsersList] = useState<User[]>([]);
  const [keysList, setKeysList] = useState<VLESSKey[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    try {
      const [usersRes, keysRes] = await Promise.all([usersApi.list(), keysApi.list()]);
      setUsersList(usersRes.users);
      setKeysList(keysRes.keys);
    } catch {
      toast("Failed to load data", "error");
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    if (authLoading) return;
    if (!authenticated) {
      router.push("/admin/login");
      return;
    }
    fetchData();
  }, [authenticated, authLoading, router, fetchData]);

  const handleLogout = async () => {
    await logout();
    router.push("/admin/login");
  };

  if (authLoading || loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-zinc-500">Loading...</p>
      </div>
    );
  }

  if (!authenticated) return null;

  const assignableKeys = keysList.filter((k) => k.status === "active");

  return (
    <div className="min-h-screen">
      {/* Top bar */}
      <header className="border-b border-border px-6 py-4 flex items-center justify-between">
        <h1 className="text-lg font-semibold">Xray Sub</h1>
        <div className="flex items-center gap-4">
          <a href="/subscription" className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors">
            Клиентская страница
          </a>
          <button onClick={handleLogout} className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors">
            Выйти
          </button>
        </div>
      </header>

      {/* Content */}
      <main className="max-w-6xl mx-auto p-6 flex flex-col gap-8">
        <UsersSection
          users={usersList}
          assignableKeys={assignableKeys}
          onRefresh={fetchData}
        />
        <KeysSection
          keys={keysList}
          onRefresh={fetchData}
        />
      </main>
    </div>
  );
}
```

**Step 2: UsersSection component**

`frontend/src/components/admin/UsersSection.tsx`:

```tsx
"use client";

import { useState } from "react";
import { User, VLESSKey } from "@/lib/types";
import { users as usersApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { AddUserModal } from "./AddUserModal";
import { EditSubscriptionModal } from "./EditSubscriptionModal";
import { KeyAssignerModal } from "./KeyAssignerModal";
import { HwidManager } from "./HwidManager";

interface Props {
  users: User[];
  assignableKeys: VLESSKey[];
  onRefresh: () => Promise<void>;
}

export function UsersSection({ users, assignableKeys, onRefresh }: Props) {
  const { toast } = useToast();
  const [collapsed, setCollapsed] = useState(false);
  const [showAddUser, setShowAddUser] = useState(false);
  const [editSubUser, setEditSubUser] = useState<User | null>(null);
  const [editKeysUser, setEditKeysUser] = useState<User | null>(null);
  const [hwidUser, setHwidUser] = useState<User | null>(null);

  const handleDelete = async (id: number, name: string) => {
    if (!confirm(`Delete user "${name}"?`)) return;
    try {
      await usersApi.delete(id);
      toast("User deleted", "success");
      await onRefresh();
    } catch {
      toast("Failed to delete user", "error");
    }
  };

  const statusBadge = (status: string) => {
    const colors: Record<string, string> = {
      active: "bg-green-500/10 text-green-400",
      paused: "bg-yellow-500/10 text-yellow-400",
      blocked: "bg-red-500/10 text-red-400",
    };
    return (
      <span className={`text-xs px-2 py-0.5 rounded-full ${colors[status] || "bg-zinc-500/10 text-zinc-400"}`}>
        {status}
      </span>
    );
  };

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <button onClick={() => setCollapsed(!collapsed)} className="flex items-center gap-2 text-lg font-semibold">
          <span className={`transition-transform ${collapsed ? "" : "rotate-90"}`}>&#9654;</span>
          Users ({users.length})
        </button>
        <Button onClick={() => setShowAddUser(true)} className="text-xs">
          + Добавить
        </Button>
      </div>

      {!collapsed && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-zinc-500 border-b border-border">
                <th className="pb-2 pr-4">Имя</th>
                <th className="pb-2 pr-4">Email</th>
                <th className="pb-2 pr-4">Код активации</th>
                <th className="pb-2 pr-4">Статус</th>
                <th className="pb-2 pr-4">Подписка</th>
                <th className="pb-2">Действия</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id} className="border-b border-border last:border-0">
                  <td className="py-3 pr-4">{user.name}</td>
                  <td className="py-3 pr-4 text-zinc-400">{user.email || "—"}</td>
                  <td className="py-3 pr-4 font-mono text-xs">{user.activation_code}</td>
                  <td className="py-3 pr-4">{statusBadge(user.status)}</td>
                  <td className="py-3 pr-4 text-xs text-zinc-400">
                    {user.starts_at && <div>с {user.starts_at}</div>}
                    {user.expires_at && <div>до {user.expires_at}</div>}
                  </td>
                  <td className="py-3">
                    <div className="flex gap-1">
                      <Button variant="ghost" className="text-xs" onClick={() => setEditKeysUser(user)}>
                        Ключи
                      </Button>
                      <Button variant="ghost" className="text-xs" onClick={() => setEditSubUser(user)}>
                        Подписка
                      </Button>
                      <Button variant="ghost" className="text-xs" onClick={() => setHwidUser(user)}>
                        HWID
                      </Button>
                      <Button variant="danger" className="text-xs" onClick={() => handleDelete(user.id, user.name)}>
                        Удалить
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {users.length === 0 && (
                <tr>
                  <td colSpan={6} className="py-8 text-center text-zinc-500">
                    Нет пользователей
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <AddUserModal open={showAddUser} onClose={() => setShowAddUser(false)} onRefresh={onRefresh} />
      {editSubUser && (
        <EditSubscriptionModal user={editSubUser} onClose={() => setEditSubUser(null)} onRefresh={onRefresh} />
      )}
      {editKeysUser && (
        <KeyAssignerModal
          user={editKeysUser}
          assignableKeys={assignableKeys}
          onClose={() => setEditKeysUser(null)}
          onRefresh={onRefresh}
        />
      )}
      {hwidUser && (
        <HwidManager user={hwidUser} onClose={() => setHwidUser(null)} onRefresh={onRefresh} />
      )}
    </Card>
  );
}
```

**Step 3: AddUserModal**

`frontend/src/components/admin/AddUserModal.tsx`:

```tsx
"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi } from "@/lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function AddUserModal({ open, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [status, setStatus] = useState("active");
  const [days, setDays] = useState("30");

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await usersApi.create({
        name,
        email,
        activation_code: code,
        status,
        issue_days: parseInt(days) || 30,
      });
      toast("User created", "success");
      setName("");
      setEmail("");
      setCode("");
      setStatus("active");
      setDays("30");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Failed to create user", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Добавить пользователя">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Input label="Имя" value={name} onChange={(e) => setName(e.target.value)} required />
        <Input label="Email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        <Input label="Код активации" value={code} onChange={(e) => setCode(e.target.value)} required />
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-zinc-400">Статус</label>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-200"
          >
            <option value="active">active</option>
            <option value="paused">paused</option>
            <option value="blocked">blocked</option>
          </select>
        </div>
        <Input label="Дней подписки" type="number" value={days} onChange={(e) => setDays(e.target.value)} min="1" max="3650" />
        <Button type="submit" loading={loading}>
          Создать
        </Button>
      </form>
    </Modal>
  );
}
```

**Step 4: EditSubscriptionModal**

`frontend/src/components/admin/EditSubscriptionModal.tsx`:

```tsx
"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi } from "@/lib/api";
import { User } from "@/lib/types";

interface Props {
  user: User;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function EditSubscriptionModal({ user, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState(user.status);
  const [startsAt, setStartsAt] = useState(user.starts_at);
  const [expiresAt, setExpiresAt] = useState(user.expires_at);
  const [blockedReason, setBlockedReason] = useState(user.blocked_reason);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await usersApi.updateSubscription(user.id, {
        status,
        starts_at: startsAt,
        expires_at: expiresAt,
        blocked_reason: blockedReason,
      });
      toast("Subscription updated", "success");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Failed to update", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Подписка — ${user.name}`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-zinc-400">Статус</label>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-200"
          >
            <option value="active">active</option>
            <option value="paused">paused</option>
            <option value="blocked">blocked</option>
          </select>
        </div>
        <Input label="Начало" type="datetime-local" value={startsAt} onChange={(e) => setStartsAt(e.target.value)} />
        <Input label="Окончание" type="datetime-local" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} />
        {status === "blocked" && (
          <Input label="Причина блокировки" value={blockedReason} onChange={(e) => setBlockedReason(e.target.value)} />
        )}
        <Button type="submit" loading={loading}>
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}
```

**Step 5: HwidManager**

`frontend/src/components/admin/HwidManager.tsx`:

```tsx
"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi } from "@/lib/api";
import { User } from "@/lib/types";

interface Props {
  user: User;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function HwidManager({ user, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [maxDevices, setMaxDevices] = useState(String(user.max_devices));

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await usersApi.updateHwid(user.id, parseInt(maxDevices) || 1);
      toast("HWID settings updated", "success");
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Failed to update", "error");
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteHwid = async (hwid: string) => {
    try {
      await usersApi.deleteHwid(user.id, hwid);
      toast("HWID removed", "success");
      await onRefresh();
    } catch {
      toast("Failed to remove HWID", "error");
    }
  };

  return (
    <Modal open onClose={onClose} title={`HWID — ${user.name}`}>
      <form onSubmit={handleSave} className="flex flex-col gap-4">
        <Input
          label="Макс. устройств"
          type="number"
          value={maxDevices}
          onChange={(e) => setMaxDevices(e.target.value)}
          min="1"
          max="32"
        />
        <Button type="submit" loading={loading}>
          Сохранить
        </Button>
      </form>

      <div className="mt-4">
        <h3 className="text-sm text-zinc-400 mb-2">
          Подключенные устройства ({user.connected_device_count}/{user.max_devices})
        </h3>
        {user.connected_hwids && user.connected_hwids.length > 0 ? (
          <div className="flex flex-col gap-1">
            {user.connected_hwids.map((hwid) => (
              <div key={hwid} className="flex items-center justify-between bg-surface-2 rounded-lg px-3 py-2">
                <span className="font-mono text-xs text-zinc-300">{hwid}</span>
                <Button variant="danger" className="text-xs" onClick={() => handleDeleteHwid(hwid)}>
                  Удалить
                </Button>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-zinc-500">Нет подключенных устройств</p>
        )}
      </div>
    </Modal>
  );
}
```

**Step 6: Commit**

```bash
git add frontend/src/app/admin/page.tsx frontend/src/components/admin/
git commit -m "feat: add admin page with users section and modals"
```

---

### Task 14: Build keys section and key assigner

**Files:**
- Create: `frontend/src/components/admin/KeysSection.tsx`
- Create: `frontend/src/components/admin/AddKeyModal.tsx`
- Create: `frontend/src/components/admin/EditKeyModal.tsx`
- Create: `frontend/src/components/admin/KeyAssignerModal.tsx`

**Step 1: KeysSection**

`frontend/src/components/admin/KeysSection.tsx`:

```tsx
"use client";

import { useState } from "react";
import { VLESSKey } from "@/lib/types";
import { keys as keysApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { AddKeyModal } from "./AddKeyModal";
import { EditKeyModal } from "./EditKeyModal";

interface Props {
  keys: VLESSKey[];
  onRefresh: () => Promise<void>;
}

export function KeysSection({ keys, onRefresh }: Props) {
  const { toast } = useToast();
  const [collapsed, setCollapsed] = useState(false);
  const [showAddKey, setShowAddKey] = useState(false);
  const [editKey, setEditKey] = useState<VLESSKey | null>(null);
  const [checkingAll, setCheckingAll] = useState(false);

  const handleDelete = async (id: number, label: string) => {
    if (!confirm(`Delete key "${label}"?`)) return;
    try {
      await keysApi.delete(id);
      toast("Key deleted", "success");
      await onRefresh();
    } catch {
      toast("Failed to delete key", "error");
    }
  };

  const handleCheck = async (id: number) => {
    try {
      await keysApi.check(id);
      toast("Key checked", "success");
      await onRefresh();
    } catch {
      toast("Failed to check key", "error");
    }
  };

  const handleCheckAll = async () => {
    setCheckingAll(true);
    try {
      const result = await keysApi.checkAll();
      toast(`Checked ${result.checked} keys`, "success");
      await onRefresh();
    } catch {
      toast("Failed to check keys", "error");
    } finally {
      setCheckingAll(false);
    }
  };

  const handleCopy = async (url: string) => {
    await navigator.clipboard.writeText(url);
    toast("URL copied", "info");
  };

  const healthDot = (status: string) => {
    const colors: Record<string, string> = {
      up: "bg-green-500",
      down: "bg-red-500",
      unknown: "bg-zinc-500",
    };
    return <span className={`inline-block w-2 h-2 rounded-full ${colors[status] || colors.unknown}`} />;
  };

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <button onClick={() => setCollapsed(!collapsed)} className="flex items-center gap-2 text-lg font-semibold">
          <span className={`transition-transform ${collapsed ? "" : "rotate-90"}`}>&#9654;</span>
          Keys ({keys.length})
        </button>
        <div className="flex gap-2">
          <Button variant="ghost" onClick={handleCheckAll} loading={checkingAll} className="text-xs">
            Проверить все
          </Button>
          <Button onClick={() => setShowAddKey(true)} className="text-xs">
            + Добавить
          </Button>
        </div>
      </div>

      {!collapsed && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-zinc-500 border-b border-border">
                <th className="pb-2 pr-4">Label</th>
                <th className="pb-2 pr-4">URL</th>
                <th className="pb-2 pr-4">Статус</th>
                <th className="pb-2 pr-4">Health</th>
                <th className="pb-2">Действия</th>
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => (
                <tr key={key.id} className="border-b border-border last:border-0">
                  <td className="py-3 pr-4">{key.label}</td>
                  <td className="py-3 pr-4">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-xs text-zinc-400 max-w-[300px] truncate">
                        {key.url_short}
                      </span>
                      <button onClick={() => handleCopy(key.url)} className="text-zinc-500 hover:text-zinc-300 text-xs">
                        copy
                      </button>
                    </div>
                  </td>
                  <td className="py-3 pr-4">
                    <span className={`text-xs px-2 py-0.5 rounded-full ${key.status === "active" ? "bg-green-500/10 text-green-400" : "bg-zinc-500/10 text-zinc-400"}`}>
                      {key.status_label}
                    </span>
                  </td>
                  <td className="py-3 pr-4">
                    <div className="flex items-center gap-2">
                      {healthDot(key.check_status)}
                      <span className="text-xs text-zinc-400">
                        {key.check_status_label}
                        {key.last_latency_ms > 0 && ` (${key.last_latency_ms}ms)`}
                      </span>
                    </div>
                  </td>
                  <td className="py-3">
                    <div className="flex gap-1">
                      <Button variant="ghost" className="text-xs" onClick={() => handleCheck(key.id)}>
                        Check
                      </Button>
                      <Button variant="ghost" className="text-xs" onClick={() => setEditKey(key)}>
                        Edit
                      </Button>
                      <Button variant="danger" className="text-xs" onClick={() => handleDelete(key.id, key.label)}>
                        Del
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {keys.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-zinc-500">
                    Нет ключей
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <AddKeyModal open={showAddKey} onClose={() => setShowAddKey(false)} onRefresh={onRefresh} />
      {editKey && <EditKeyModal keyData={editKey} onClose={() => setEditKey(null)} onRefresh={onRefresh} />}
    </Card>
  );
}
```

**Step 2: AddKeyModal**

`frontend/src/components/admin/AddKeyModal.tsx`:

```tsx
"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function AddKeyModal({ open, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [label, setLabel] = useState("");
  const [url, setUrl] = useState("");
  const [status, setStatus] = useState("active");

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await keysApi.create({ label, url, status });
      toast("Key added", "success");
      setLabel("");
      setUrl("");
      setStatus("active");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Failed to add key", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Добавить ключ">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Input label="Label" value={label} onChange={(e) => setLabel(e.target.value)} required />
        <Input label="VLESS URL" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="vless://..." required />
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-zinc-400">Статус</label>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-200"
          >
            <option value="active">active</option>
            <option value="non-active">non-active</option>
          </select>
        </div>
        <Button type="submit" loading={loading}>
          Добавить
        </Button>
      </form>
    </Modal>
  );
}
```

**Step 3: EditKeyModal**

`frontend/src/components/admin/EditKeyModal.tsx`:

```tsx
"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import { VLESSKey } from "@/lib/types";

interface Props {
  keyData: VLESSKey;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function EditKeyModal({ keyData, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [label, setLabel] = useState(keyData.label);
  const [status, setStatus] = useState(keyData.status);
  const [uuid, setUuid] = useState(keyData.edit_uuid);
  const [host, setHost] = useState(keyData.edit_host);
  const [port, setPort] = useState(keyData.edit_port);
  const [query, setQuery] = useState(keyData.edit_query);
  const [fragment, setFragment] = useState(keyData.edit_fragment);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await keysApi.update(keyData.id, { label, status, uuid, host, port, query, fragment });
      toast("Key updated", "success");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Failed to update key", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Редактировать — ${keyData.label}`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Input label="Label" value={label} onChange={(e) => setLabel(e.target.value)} required />
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-zinc-400">Статус</label>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-200"
          >
            <option value="active">active</option>
            <option value="non-active">non-active</option>
          </select>
        </div>
        <Input label="UUID" value={uuid} onChange={(e) => setUuid(e.target.value)} />
        <Input label="Host" value={host} onChange={(e) => setHost(e.target.value)} />
        <Input label="Port" value={port} onChange={(e) => setPort(e.target.value)} />
        <Input label="Query" value={query} onChange={(e) => setQuery(e.target.value)} />
        <Input label="Fragment" value={fragment} onChange={(e) => setFragment(e.target.value)} />
        <Button type="submit" loading={loading}>
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}
```

**Step 4: KeyAssignerModal**

`frontend/src/components/admin/KeyAssignerModal.tsx`:

```tsx
"use client";

import { useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi } from "@/lib/api";
import { User, VLESSKey } from "@/lib/types";

interface Props {
  user: User;
  assignableKeys: VLESSKey[];
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function KeyAssignerModal({ user, assignableKeys, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);

  // Parse assigned key IDs from comma-separated string
  const initialAssigned = user.assigned_key_ids
    ? user.assigned_key_ids.split(",").map(Number).filter(Boolean)
    : [];
  const [assignedIds, setAssignedIds] = useState<Set<number>>(new Set(initialAssigned));

  const toggle = (id: number) => {
    setAssignedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const addAll = () => {
    setAssignedIds(new Set(assignableKeys.map((k) => k.id)));
  };

  const removeAll = () => {
    setAssignedIds(new Set());
  };

  const handleSave = async () => {
    setLoading(true);
    try {
      await usersApi.updateKeys(user.id, Array.from(assignedIds));
      toast("Keys updated", "success");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Failed to update keys", "error");
    } finally {
      setLoading(false);
    }
  };

  const assigned = assignableKeys.filter((k) => assignedIds.has(k.id));
  const available = assignableKeys.filter((k) => !assignedIds.has(k.id));

  return (
    <Modal open onClose={onClose} title={`Ключи — ${user.name}`}>
      <div className="flex gap-2 mb-4">
        <Button variant="ghost" onClick={addAll} className="text-xs">
          Добавить все
        </Button>
        <Button variant="ghost" onClick={removeAll} className="text-xs">
          Убрать все
        </Button>
      </div>

      <div className="grid grid-cols-2 gap-4 mb-4">
        <div>
          <h3 className="text-sm text-zinc-400 mb-2">Доступные</h3>
          <div className="flex flex-wrap gap-1.5 min-h-[60px] bg-surface-2 rounded-lg p-3">
            {available.map((key) => (
              <button
                key={key.id}
                onClick={() => toggle(key.id)}
                className="px-2.5 py-1 rounded-md bg-surface-1 text-xs text-zinc-300 hover:bg-accent/20 transition-colors"
              >
                {key.label}
              </button>
            ))}
            {available.length === 0 && <span className="text-xs text-zinc-600">Пусто</span>}
          </div>
        </div>
        <div>
          <h3 className="text-sm text-zinc-400 mb-2">Назначенные</h3>
          <div className="flex flex-wrap gap-1.5 min-h-[60px] bg-surface-2 rounded-lg p-3">
            {assigned.map((key) => (
              <button
                key={key.id}
                onClick={() => toggle(key.id)}
                className="px-2.5 py-1 rounded-md bg-accent/20 text-xs text-indigo-300 hover:bg-accent/30 transition-colors"
              >
                {key.label}
              </button>
            ))}
            {assigned.length === 0 && <span className="text-xs text-zinc-600">Пусто</span>}
          </div>
        </div>
      </div>

      <Button onClick={handleSave} loading={loading} className="w-full">
        Сохранить
      </Button>
    </Modal>
  );
}
```

**Step 5: Commit**

```bash
git add frontend/src/components/admin/
git commit -m "feat: add keys section, key assigner, and edit modals"
```

---

## Phase 5: Cleanup & Integration

### Task 15: Delete old template and static files

**Files:**
- Delete: `web/templates/login.html`
- Delete: `web/templates/admin.html`
- Delete: `web/templates/subscription.html`
- Delete: `web/static/styles.css`

**Step 1: Remove old frontend files**

```bash
rm -rf web/
```

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: remove old Go HTML templates and static CSS"
```

---

### Task 16: End-to-end integration test

**Step 1: Build and start Go API**

```bash
cd E:/LeetCode/vless
go build -o server ./cmd/server
ADMIN_PASSWORD=admin ./server &
```

**Step 2: Start Next.js dev server**

```bash
cd E:/LeetCode/vless/frontend
npm run dev &
```

**Step 3: Manual verification checklist**

Open `http://localhost:3000` in a browser and verify:

1. [ ] Root `/` redirects to `/subscription`
2. [ ] Subscription page renders with activation form
3. [ ] Navigate to `/admin/login`
4. [ ] Login with admin/admin works, redirects to `/admin`
5. [ ] Admin panel shows Users and Keys sections
6. [ ] "Add user" modal opens and creates a user
7. [ ] "Add key" modal opens and creates a key
8. [ ] Key health check works (check button)
9. [ ] Key assignment modal opens and assigns keys
10. [ ] Subscription editing modal works
11. [ ] HWID manager modal opens
12. [ ] User/key delete works
13. [ ] Logout works and redirects to login
14. [ ] VPN subscription delivery still works: `curl http://localhost:8080/sub/{id}`

**Step 4: Fix any issues found, then final commit**

```bash
git add -A
git commit -m "chore: final integration fixes"
```

---

## Execution Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1 | 1-5 | Convert Go backend to JSON API |
| 2 | 6-8 | Initialize Next.js with Tailwind, API client, auth |
| 3 | 9-10 | Base UI components and root layout |
| 4 | 11-14 | Login, subscription, admin pages with all modals |
| 5 | 15-16 | Remove old templates, integration test |

**Total: 16 tasks, ~15 commits**
