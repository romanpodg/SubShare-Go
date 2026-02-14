package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/subscription", http.StatusFound)
}

func (a *App) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if a.isAdminAuthenticated(r) {
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}
		if err := a.templates.ExecuteTemplate(w, "login.html", LoginPageData{Error: r.URL.Query().Get("err")}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/admin/login?err=invalid+form", http.StatusFound)
			return
		}
		username := strings.TrimSpace(r.FormValue("username"))
		password := strings.TrimSpace(r.FormValue("password"))
		if !secureEqual(username, a.adminUser) || !secureEqual(password, a.adminPass) {
			http.Redirect(w, r, "/admin/login?err=invalid+credentials", http.StatusFound)
			return
		}

		sessionID, err := generateToken(32)
		if err != nil {
			http.Redirect(w, r, "/admin/login?err=failed+to+create+session", http.StatusFound)
			return
		}
		csrfToken, err := generateToken(32)
		if err != nil {
			http.Redirect(w, r, "/admin/login?err=failed+to+create+session", http.StatusFound)
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
		http.Redirect(w, r, "/admin", http.StatusFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}

func (a *App) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users, err := a.listUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	keys, err := a.listKeys()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	assignableKeys, err := a.listAssignableKeys()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := AdminPageData{
		Users:          users,
		Keys:           keys,
		AssignableKeys: assignableKeys,
		BaseURL:        resolveBaseURL(r),
		CSRFToken:      a.mustCSRFToken(r),
		Message:        r.URL.Query().Get("msg"),
		Error:          r.URL.Query().Get("err"),
	}

	if err := a.templates.ExecuteTemplate(w, "admin.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) handleSubscriptionPage(w http.ResponseWriter, r *http.Request) {
	data := SubscriptionPageData{}

	switch r.Method {
	case http.MethodGet:
		if err := a.templates.ExecuteTemplate(w, "subscription.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			data.Error = "invalid form"
			_ = a.templates.ExecuteTemplate(w, "subscription.html", data)
			return
		}

		activationCode := strings.TrimSpace(r.FormValue("token"))
		if activationCode == "" || strings.Contains(activationCode, "/") {
			data.Error = "Введите корректный ключ активации"
			_ = a.templates.ExecuteTemplate(w, "subscription.html", data)
			return
		}

		subscriptionID, code, reason, err := a.redeemActivationCode(activationCode)
		if err != nil {
			http.Error(w, "failed to activate subscription", http.StatusInternalServerError)
			return
		}
		if code != http.StatusOK {
			if strings.TrimSpace(reason) == "" {
				data.Error = "Не удалось активировать подписку"
			} else {
				data.Error = reason
			}
			data.Token = activationCode
			_ = a.templates.ExecuteTemplate(w, "subscription.html", data)
			return
		}

		data.Token = activationCode
		data.SubscriptionURL = fmt.Sprintf("%s/sub/%s", resolveBaseURL(r), subscriptionID)
		data.Message = "Ключ активирован. Ссылка готова — скопируйте и вставьте её в VPN-клиент"
		if err := a.templates.ExecuteTemplate(w, "subscription.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.redirectAdmin(w, r, "", "invalid form")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	activationCode := strings.TrimSpace(r.FormValue("activation_code"))
	status, ok := normalizeUserStatus(r.FormValue("status"))
	if !ok {
		a.redirectAdmin(w, r, "", "invalid subscription status")
		return
	}
	issueDays := 30
	if rawDays := strings.TrimSpace(r.FormValue("issue_days")); rawDays != "" {
		days, err := strconv.Atoi(rawDays)
		if err != nil || days <= 0 || days > 3650 {
			a.redirectAdmin(w, r, "", "issue days must be between 1 and 3650")
			return
		}
		issueDays = days
	}
	blockedReason := strings.TrimSpace(r.FormValue("blocked_reason"))
	if status != userStatusBlocked {
		blockedReason = ""
	}

	if name == "" {
		a.redirectAdmin(w, r, "", "name is required")
		return
	}
	if activationCode == "" || strings.Contains(activationCode, "/") {
		a.redirectAdmin(w, r, "", "activation code is required")
		return
	}

	legacyToken, err := generateToken(24)
	if err != nil {
		a.redirectAdmin(w, r, "", "failed to generate user token")
		return
	}

	now := time.Now().UTC()
	expiresAt := now.AddDate(0, 0, issueDays)

	_, err = a.db.Exec(
		`INSERT INTO users(name, email, token, activation_code, status, starts_at, expires_at, blocked_reason, max_devices) VALUES(?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		name,
		email,
		legacyToken,
		activationCode,
		status,
		now,
		expiresAt,
		blockedReason,
	)
	if err != nil {
		a.redirectAdmin(w, r, "", "failed to create user (check activation code uniqueness)")
		return
	}
	a.redirectAdmin(w, r, "user created", "")
}

func (a *App) handleUserActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/admin/users/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		a.redirectAdmin(w, r, "", "invalid user id")
		return
	}

	switch parts[1] {
	case "delete":
		if _, err := a.db.Exec(`DELETE FROM users WHERE id = ?`, id); err != nil {
			a.redirectAdmin(w, r, "", "failed to delete user")
			return
		}
		a.redirectAdmin(w, r, "user deleted", "")
	case "keys":
		if err := r.ParseForm(); err != nil {
			a.redirectAdmin(w, r, "", "invalid form")
			return
		}

		tx, err := a.db.Begin()
		if err != nil {
			a.redirectAdmin(w, r, "", "failed to update user keys")
			return
		}
		defer tx.Rollback()

		if _, err := tx.Exec(`DELETE FROM user_keys WHERE user_id = ?`, id); err != nil {
			a.redirectAdmin(w, r, "", "failed to update user keys")
			return
		}

		rawKeyIDs := r.Form["key_ids"]
		seen := make(map[int64]struct{}, len(rawKeyIDs))
		for _, raw := range rawKeyIDs {
			keyID, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if err != nil || keyID <= 0 {
				a.redirectAdmin(w, r, "", "invalid key id")
				return
			}
			if _, ok := seen[keyID]; ok {
				continue
			}
			seen[keyID] = struct{}{}

			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO user_keys(user_id, key_id)
				 SELECT ?, id
				 FROM vless_keys
				 WHERE id = ?
				   AND LOWER(COALESCE(NULLIF(TRIM(status), ''), 'active')) = 'active'`,
				id,
				keyID,
			); err != nil {
				a.redirectAdmin(w, r, "", "failed to update user keys")
				return
			}
		}

		if err := tx.Commit(); err != nil {
			a.redirectAdmin(w, r, "", "failed to update user keys")
			return
		}
		a.redirectAdmin(w, r, "subscription keys updated", "")
	case "subscription":
		if err := r.ParseForm(); err != nil {
			a.redirectAdmin(w, r, "", "invalid form")
			return
		}

		status, ok := normalizeUserStatus(r.FormValue("status"))
		if !ok {
			a.redirectAdmin(w, r, "", "invalid subscription status")
			return
		}

		startsAt, err := parseOptionalDateTimeLocal(r.FormValue("starts_at"))
		if err != nil {
			a.redirectAdmin(w, r, "", "invalid starts_at datetime")
			return
		}
		expiresAt, err := parseOptionalDateTimeLocal(r.FormValue("expires_at"))
		if err != nil {
			a.redirectAdmin(w, r, "", "invalid expires_at datetime")
			return
		}

		if startsAt.Valid && expiresAt.Valid && startsAt.Time.After(expiresAt.Time) {
			a.redirectAdmin(w, r, "", "starts_at must be before expires_at")
			return
		}

		blockedReason := strings.TrimSpace(r.FormValue("blocked_reason"))
		if status != userStatusBlocked {
			blockedReason = ""
		}

		_, err = a.db.Exec(
			`UPDATE users SET status = ?, starts_at = ?, expires_at = ?, blocked_reason = ? WHERE id = ?`,
			status,
			nullTimeValue(startsAt),
			nullTimeValue(expiresAt),
			nullStringValue(blockedReason),
			id,
		)
		if err != nil {
			a.redirectAdmin(w, r, "", "failed to update subscription")
			return
		}
		a.redirectAdmin(w, r, "subscription updated", "")
	case "hwid":
		if err := r.ParseForm(); err != nil {
			a.redirectAdmin(w, r, "", "invalid form")
			return
		}

		maxDevices, err := strconv.Atoi(strings.TrimSpace(r.FormValue("max_devices")))
		if err != nil || maxDevices <= 0 || maxDevices > 32 {
			a.redirectAdmin(w, r, "", "max_devices must be between 1 and 32")
			return
		}

		if _, err := a.db.Exec(`UPDATE users SET max_devices = ? WHERE id = ?`, maxDevices, id); err != nil {
			a.redirectAdmin(w, r, "", "failed to update hwid settings")
			return
		}
		a.redirectAdmin(w, r, "hwid settings updated", "")
	default:
		http.NotFound(w, r)
		return
	}
}

func (a *App) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.redirectAdmin(w, r, "", "invalid form")
		return
	}

	label := strings.TrimSpace(r.FormValue("label"))
	keyURL := strings.TrimSpace(r.FormValue("url"))
	status, ok := normalizeKeyStatus(r.FormValue("status"))
	if !ok {
		a.redirectAdmin(w, r, "", "invalid key status")
		return
	}
	if label == "" || keyURL == "" {
		a.redirectAdmin(w, r, "", "label and url are required")
		return
	}
	if !strings.HasPrefix(strings.ToLower(keyURL), "vless://") {
		a.redirectAdmin(w, r, "", "url must start with vless://")
		return
	}

	if _, err := a.db.Exec(`INSERT INTO vless_keys(label, url, status, check_status) VALUES(?, ?, ?, 'unknown')`,
		label,
		keyURL,
		status,
	); err != nil {
		a.redirectAdmin(w, r, "", "failed to add key (maybe duplicate)")
		return
	}
	a.redirectAdmin(w, r, "key added", "")
}

func (a *App) handleCheckAllKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := a.db.Query(`SELECT id, url FROM vless_keys ORDER BY id`)
	if err != nil {
		if prefersJSONResponse(r) {
			a.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to load keys"})
			return
		}
		a.redirectAdmin(w, r, "", "failed to load keys")
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
			if prefersJSONResponse(r) {
				a.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to read keys"})
				return
			}
			a.redirectAdmin(w, r, "", "failed to read keys")
			return
		}
		keys = append(keys, row)
	}
	if err := rows.Err(); err != nil {
		if prefersJSONResponse(r) {
			a.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to read keys"})
			return
		}
		a.redirectAdmin(w, r, "", "failed to read keys")
		return
	}

	checked := 0
	for _, key := range keys {
		if err := a.checkAndPersistKey(key.id, key.url); err != nil {
			if prefersJSONResponse(r) {
				a.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to check keys"})
				return
			}
			a.redirectAdmin(w, r, "", "failed to check keys")
			return
		}
		checked++
	}

	if prefersJSONResponse(r) {
		updatedRows, err := a.db.Query(`
			SELECT id, check_status, check_error, last_checked_at, last_latency_ms
			FROM vless_keys
			ORDER BY id
		`)
		if err != nil {
			a.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to load key checks"})
			return
		}
		defer updatedRows.Close()

		results := make([]map[string]any, 0, checked)
		for updatedRows.Next() {
			var id int64
			var checkStatus sql.NullString
			var checkError sql.NullString
			var lastCheckedAt sql.NullTime
			var latency sql.NullInt64
			if err := updatedRows.Scan(&id, &checkStatus, &checkError, &lastCheckedAt, &latency); err != nil {
				a.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to load key checks"})
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
			a.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to load key checks"})
			return
		}
		a.writeJSON(w, http.StatusOK, map[string]any{"checked": checked, "keys": results})
		return
	}

	a.redirectAdmin(w, r, fmt.Sprintf("checked %d keys", checked), "")
}

func (a *App) handleKeyActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.redirectAdmin(w, r, "", "invalid form")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/admin/keys/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		a.redirectAdmin(w, r, "", "invalid key id")
		return
	}

	switch parts[1] {
	case "delete":
		if _, err := a.db.Exec(`DELETE FROM vless_keys WHERE id = ?`, id); err != nil {
			a.redirectAdmin(w, r, "", "failed to delete key")
			return
		}
		a.redirectAdmin(w, r, "key deleted", "")
	case "check":
		var rawURL string
		if err := a.db.QueryRow(`SELECT url FROM vless_keys WHERE id = ?`, id).Scan(&rawURL); err != nil {
			a.redirectAdmin(w, r, "", "key not found")
			return
		}

		if err := a.checkAndPersistKey(id, rawURL); err != nil {
			if prefersJSONResponse(r) {
				a.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to check key"})
				return
			}
			a.redirectAdmin(w, r, "", "failed to check key")
			return
		}
		if prefersJSONResponse(r) {
			a.respondJSONKeyCheck(w, id)
			return
		}
		a.redirectAdmin(w, r, "key checked", "")
	case "edit":
		label := strings.TrimSpace(r.FormValue("label"))
		status, ok := normalizeKeyStatus(r.FormValue("status"))
		if !ok {
			a.redirectAdmin(w, r, "", "invalid key status")
			return
		}

		uuid := strings.TrimSpace(r.FormValue("uuid"))
		host := strings.TrimSpace(r.FormValue("host"))
		port := strings.TrimSpace(r.FormValue("port"))
		query := strings.TrimSpace(r.FormValue("query"))
		fragment := strings.TrimSpace(r.FormValue("fragment"))

		if label == "" {
			a.redirectAdmin(w, r, "", "label is required")
			return
		}

		builtURL, err := buildVLESSURL(uuid, host, port, query, fragment)
		if err != nil {
			a.redirectAdmin(w, r, "", err.Error())
			return
		}

		if _, err := a.db.Exec(
			`UPDATE vless_keys SET label = ?, url = ?, status = ? WHERE id = ?`,
			label,
			builtURL,
			status,
			id,
		); err != nil {
			a.redirectAdmin(w, r, "", "failed to update key")
			return
		}
		a.redirectAdmin(w, r, "key updated", "")
	case "update":
		status, ok := normalizeKeyStatus(r.FormValue("status"))
		if !ok {
			a.redirectAdmin(w, r, "", "invalid key status")
			return
		}

		if _, err = a.db.Exec(
			`UPDATE vless_keys SET status = ? WHERE id = ?`,
			status,
			id,
		); err != nil {
			a.redirectAdmin(w, r, "", "failed to update key")
			return
		}
		a.redirectAdmin(w, r, "key updated", "")
	default:
		http.NotFound(w, r)
		return
	}
}

func (a *App) handleSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	subscriptionID := strings.TrimPrefix(r.URL.Path, "/sub/")
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

func (a *App) redirectAdmin(w http.ResponseWriter, r *http.Request, msg, errText string) {
	q := ""
	switch {
	case msg != "":
		q = "?msg=" + url.QueryEscape(msg)
	case errText != "":
		q = "?err=" + url.QueryEscape(errText)
	}
	http.Redirect(w, r, "/admin"+q, http.StatusFound)
}

func prefersJSONResponse(r *http.Request) bool {
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	xrw := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Requested-With")))
	return strings.Contains(accept, "application/json") || xrw == "xmlhttprequest"
}

func (a *App) writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *App) respondJSONKeyCheck(w http.ResponseWriter, keyID int64) {
	var checkStatus sql.NullString
	var checkError sql.NullString
	var lastCheckedAt sql.NullTime
	var latency sql.NullInt64
	if err := a.db.QueryRow(
		`SELECT check_status, check_error, last_checked_at, last_latency_ms FROM vless_keys WHERE id = ?`,
		keyID,
	).Scan(&checkStatus, &checkError, &lastCheckedAt, &latency); err != nil {
		a.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed to load key check"})
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
	a.writeJSON(w, http.StatusOK, payload)
}
