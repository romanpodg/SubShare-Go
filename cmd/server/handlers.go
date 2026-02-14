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
