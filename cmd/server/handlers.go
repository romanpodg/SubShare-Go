package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/middleware"
	"github.com/romanpodg/SubShare-Go/internal/model"
	adminpassword "github.com/romanpodg/SubShare-Go/internal/security/password"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{8}$`)

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func readJSON(r *http.Request, dst any) error {
	return readJSONWithLimit(nil, r, dst, 1<<20)
}

func readJSONWithLimit(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON value")
		}
		return err
	}
	return nil
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

func normalizeBulkKeyIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids list is empty")
	}

	normalized := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))

	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("ids list contains invalid key id")
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("ids list contains duplicates")
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized, nil
}

func normalizeKeyCategory(raw string) string {
	return keymanagement.NormalizeKeyCategory(raw)
}

func normalizeKeyCategoryColor(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "#d8b33d"
	}
	if matched, _ := regexp.MatchString(`^#[0-9A-Fa-f]{6}$`, value); matched {
		return strings.ToUpper(value)
	}
	return "#d8b33d"
}

func (a *App) upsertKeyCategory(category string) error {
	category = normalizeKeyCategory(category)
	if category == "" {
		return nil
	}
	var nextSortOrder int64
	if err := a.db.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextSortOrder); err != nil {
		return err
	}
	_, err := a.db.Exec(
		`INSERT INTO key_categories(name, color, sort_order, updated_at)
		 VALUES(?, '#d8b33d', ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		category,
		nextSortOrder,
	)
	return err
}

func (a *App) keyCategoryID(category string) (any, error) {
	category = normalizeKeyCategory(category)
	if category == "" {
		return nil, nil
	}
	var id int64
	if err := a.db.QueryRow(`SELECT id FROM key_categories WHERE name = ?`, category).Scan(&id); err != nil {
		return nil, err
	}
	return id, nil
}

func (a *App) listKeyCategories() ([]model.KeyCategory, error) {
	rows, err := a.db.Query(`SELECT id, name, color FROM key_categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	countByName := make(map[string]int)
	colorByName := make(map[string]string)
	categories := make([]model.KeyCategory, 0, 16)
	for rows.Next() {
		var id int64
		var name sql.NullString
		var color sql.NullString
		if err := rows.Scan(&id, &name, &color); err != nil {
			return nil, err
		}
		normalized := normalizeKeyCategory(name.String)
		if normalized == "" {
			continue
		}
		if _, exists := countByName[normalized]; exists {
			continue
		}
		countByName[normalized] = 0
		colorByName[normalized] = normalizeKeyCategoryColor(color.String)
		categories = append(categories, model.KeyCategory{ID: id, Name: normalized, Color: colorByName[normalized], KeysCount: 0})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	countRows, err := a.db.Query(`
		SELECT kc.name, COUNT(*)
		FROM vless_keys k
		JOIN key_categories kc ON kc.id = k.category_id
		GROUP BY k.category_id, kc.name
	`)
	if err != nil {
		return nil, err
	}
	defer countRows.Close()

	for countRows.Next() {
		var category sql.NullString
		var count int64
		if err := countRows.Scan(&category, &count); err != nil {
			return nil, err
		}
		normalized := normalizeKeyCategory(category.String)
		if normalized == "" {
			continue
		}
		if _, exists := countByName[normalized]; !exists {
			colorByName[normalized] = "#d8b33d"
			var id int64
			_ = a.db.QueryRow(`SELECT id FROM key_categories WHERE name = ?`, normalized).Scan(&id)
			categories = append(categories, model.KeyCategory{ID: id, Name: normalized, Color: colorByName[normalized], KeysCount: 0})
		}
		countByName[normalized] += int(count)
	}
	if err := countRows.Err(); err != nil {
		return nil, err
	}

	for index := range categories {
		categories[index].KeysCount = countByName[categories[index].Name]
		categories[index].Color = normalizeKeyCategoryColor(colorByName[categories[index].Name])
	}
	return categories, nil
}

// --- Auth API ---

func (a *App) apiLogin(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)
	adminID, authenticated, err := a.authenticateAdministrator(r.Context(), username, req.Password)
	if err != nil {
		log.Printf("apiLogin: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if !authenticated {
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

	expiresAt := time.Now().Add(24 * time.Hour)
	_, err = a.db.Exec(`INSERT INTO admin_sessions(id, admin_id, csrf_token, expires_at) VALUES(?, ?, ?, ?)`, sessionID, adminID, csrfToken, expiresAt)
	if err != nil {
		log.Printf("apiLogin: failed to save session to database: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     model.AdminSessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || (middleware.TrustedProxy(r) && firstForwardedValue(r.Header.Get("X-Forwarded-Proto")) == "https"),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int((24 * time.Hour).Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]any{"csrf_token": csrfToken})
}

func (a *App) apiLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(model.AdminSessionCookieName)
	if err == nil && cookie.Value != "" {
		_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE id = ?`, cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     model.AdminSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	writeMessage(w, "logged out")
}

func (a *App) apiMe(w http.ResponseWriter, r *http.Request) {
	session, _, _ := a.adminSessionFromRequest(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"role":          session.Role,
		"csrf_token":    session.CSRFToken,
	})
}

// --- Admins Management API (Super Admin only) ---

func (a *App) apiListAdmins(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT id, username, role, created_at FROM admins ORDER BY id ASC`)
	if err != nil {
		log.Printf("apiListAdmins: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to query admins")
		return
	}
	defer rows.Close()

	var admins []model.Admin
	for rows.Next() {
		var adm model.Admin
		if err := rows.Scan(&adm.ID, &adm.Username, &adm.Role, &adm.CreatedAt); err != nil {
			log.Printf("apiListAdmins scan: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to scan admins")
			return
		}
		admins = append(admins, adm)
	}

	if admins == nil {
		admins = []model.Admin{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"admins": admins})
}

func (a *App) apiCreateAdmin(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAdminRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)
	role, roleOK := normalizeAdminRole(req.Role)

	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if !roleOK {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}

	// Check if username already exists
	var exists bool
	err := a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM admins WHERE username = ?)`, username).Scan(&exists)
	if err != nil {
		log.Printf("apiCreateAdmin check exists: %v", err)
		writeError(w, http.StatusInternalServerError, "internal database error")
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "username is already taken")
		return
	}

	passwordHash, err := a.passwordHasher().Hash(req.Password)
	if err != nil {
		if adminpassword.IsPolicyError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("apiCreateAdmin: password hashing failed")
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	result, err := a.db.Exec(`INSERT INTO admins (username, password_hash, role) VALUES (?, ?, ?)`, username, passwordHash, role)
	if err != nil {
		log.Printf("apiCreateAdmin insert: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create admin")
		return
	}
	adminID, _ := result.LastInsertId()
	a.recordAuditEvent(r, "admin.create", "admin", strconv.FormatInt(adminID, 10), map[string]any{"username": username, "role": role})

	writeMessage(w, "administrator created successfully")
}

func (a *App) apiUpdateAdmin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid administrator id")
		return
	}

	var req model.UpdateAdminRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	role := strings.TrimSpace(req.Role)
	passwordValue := req.Password

	session, _, _ := a.adminSessionFromRequest(r)

	// Fetch current admin info
	var currentUsername string
	var currentRole string
	err = a.db.QueryRow(`SELECT username, role FROM admins WHERE id = ?`, id).Scan(&currentUsername, &currentRole)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "administrator not found")
			return
		}
		log.Printf("apiUpdateAdmin query: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Update password if provided
	if passwordValue != "" {
		passwordHash, err := a.passwordHasher().Hash(passwordValue)
		if err != nil {
			if adminpassword.IsPolicyError(err) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			log.Printf("apiUpdateAdmin: password hashing failed")
			writeError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}
		_, err = a.db.Exec(`UPDATE admins SET password_hash = ? WHERE id = ?`, passwordHash, id)
		if err != nil {
			log.Printf("apiUpdateAdmin password update: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to update password")
			return
		}
		// Invalidate all active sessions for this admin since password changed, except the current one
		cookie, err := r.Cookie(model.AdminSessionCookieName)
		if err == nil && cookie.Value != "" {
			_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ? AND id != ?`, id, cookie.Value)
		} else {
			_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, id)
		}
	}

	// Update role if provided
	if role != "" {
		normalizedRole, roleOK := normalizeAdminRole(role)
		if !roleOK {
			writeError(w, http.StatusBadRequest, "invalid role")
			return
		}
		role = normalizedRole

		// Prevent changing own role
		if id == session.AdminID {
			writeError(w, http.StatusBadRequest, "you cannot change your own role")
			return
		}

		// Prevent changing role of the last owner
		if isOwnerRole(currentRole) && !isOwnerRole(role) {
			var superAdminCount int
			err = a.db.QueryRow(`SELECT COUNT(*) FROM admins WHERE role IN ('owner', 'super_admin')`).Scan(&superAdminCount)
			if err != nil {
				log.Printf("apiUpdateAdmin count super admins: %v", err)
				writeError(w, http.StatusInternalServerError, "database error")
				return
			}
			if superAdminCount <= 1 {
				writeError(w, http.StatusBadRequest, "cannot demote the only remaining super admin")
				return
			}
		}

		_, err = a.db.Exec(`UPDATE admins SET role = ? WHERE id = ?`, role, id)
		if err != nil {
			log.Printf("apiUpdateAdmin role update: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to update role")
			return
		}

		// Invalidate all sessions for the updated admin since role changed
		_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, id)
	}

	a.recordAuditEvent(r, "admin.update", "admin", strconv.FormatInt(id, 10), map[string]any{
		"role":             role,
		"password_changed": passwordValue != "",
	})
	writeMessage(w, "administrator updated successfully")
}

func (a *App) apiDeleteAdmin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid administrator id")
		return
	}

	session, _, _ := a.adminSessionFromRequest(r)

	// Prevent deleting oneself
	if id == session.AdminID {
		writeError(w, http.StatusBadRequest, "you cannot delete your own account")
		return
	}

	// Fetch admin to delete
	var role string
	err = a.db.QueryRow(`SELECT role FROM admins WHERE id = ?`, id).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "administrator not found")
			return
		}
		log.Printf("apiDeleteAdmin query: %v", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Prevent deleting the last owner
	if isOwnerRole(role) {
		var superAdminCount int
		err = a.db.QueryRow(`SELECT COUNT(*) FROM admins WHERE role IN ('owner', 'super_admin')`).Scan(&superAdminCount)
		if err != nil {
			log.Printf("apiDeleteAdmin count super admins: %v", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		if superAdminCount <= 1 {
			writeError(w, http.StatusBadRequest, "cannot delete the only remaining super admin")
			return
		}
	}

	// Delete sessions first
	_, err = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteAdmin delete sessions: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete sessions")
		return
	}

	// Delete admin
	res, err := a.db.Exec(`DELETE FROM admins WHERE id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteAdmin delete admin: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete admin")
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		writeError(w, http.StatusNotFound, "administrator not found")
		return
	}

	a.recordAuditEvent(r, "admin.delete", "admin", strconv.FormatInt(id, 10), nil)
	writeMessage(w, "administrator deleted successfully")
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
		users = []model.User{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (a *App) apiCreateUser(w http.ResponseWriter, r *http.Request) {
	var req model.CreateUserRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	activationCode := strings.TrimSpace(req.ActivationCode)
	status, ok := model.NormalizeUserStatus(req.Status)
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
	if status != model.UserStatusBlocked {
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
	if len(name) > 255 {
		writeError(w, http.StatusBadRequest, "name is too long (max 255 characters)")
		return
	}
	if len(email) > 255 {
		writeError(w, http.StatusBadRequest, "email is too long (max 255 characters)")
		return
	}
	if len(activationCode) > 128 {
		writeError(w, http.StatusBadRequest, "activation code is too long (max 128 characters)")
		return
	}

	legacyToken, err := generateToken(24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate user token")
		return
	}

	subscriptionID, err := generateToken(24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate subscription token")
		return
	}

	now := time.Now().UTC()
	expiresAt := now.AddDate(0, 0, issueDays)

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	defer tx.Rollback()

	var userID int64
	for attempt := 0; attempt < 5; attempt++ {
		var res sql.Result
		res, err = tx.Exec(
			`INSERT INTO users(name, email, token, activation_code, subscription_id, status, starts_at, expires_at, blocked_reason, max_devices) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			name, email, legacyToken, activationCode, subscriptionID, status, now, expiresAt, blockedReason,
		)
		if err == nil {
			userID, _ = res.LastInsertId()
			break
		}

		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "users.subscription_id") || strings.Contains(errText, "idx_users_subscription_id") {
			subscriptionID, err = generateToken(24)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to generate subscription token")
				return
			}
			continue
		}
		break
	}
	if err != nil {
		log.Printf("apiCreateUser: %v", err)
		writeError(w, http.StatusConflict, "failed to create user (check activation code uniqueness)")
		return
	}

	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO user_keys(user_id, key_id) SELECT ?, id FROM vless_keys`,
		userID,
	); err != nil {
		log.Printf("apiCreateUser: failed to assign keys to user %d: %v", userID, err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	a.recordAuditEvent(r, "user.create", "user", strconv.FormatInt(userID, 10), map[string]any{"name": name})
	writeMessage(w, "user created")
}

func (a *App) apiDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	res, err := a.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteUser: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	a.recordAuditEvent(r, "user.delete", "user", strconv.FormatInt(id, 10), nil)
	writeMessage(w, "user deleted")
}

func (a *App) apiUpdateUserKeys(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateUserKeysRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := a.updateUserKeyAssignment(id, model.KeyAssignmentModeSelected, req.KeyIDs); err != nil {
		if errors.Is(err, errAssignmentUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, errAssignmentKeyNotFound) {
			writeError(w, http.StatusBadRequest, "one or more keys do not exist")
			return
		}
		log.Printf("apiUpdateUserKeys: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update user keys")
		return
	}
	a.recordAuditEvent(r, "user.keys.update", "user", strconv.FormatInt(id, 10), map[string]any{
		"mode": model.KeyAssignmentModeSelected, "keys_count": len(req.KeyIDs),
	})
	writeMessage(w, "subscription keys updated")
}

func (a *App) apiUpdateUserSubscription(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateSubscriptionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	status, ok := model.NormalizeUserStatus(req.Status)
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
	if status != model.UserStatusBlocked {
		blockedReason = ""
	}

	subscriptionName := strings.TrimSpace(req.SubscriptionName)
	if len(subscriptionName) > 120 {
		writeError(w, http.StatusBadRequest, "subscription_name is too long (max 120 characters)")
		return
	}

	subscriptionRefreshHours := req.SubscriptionRefreshHours
	if subscriptionRefreshHours <= 0 {
		subscriptionRefreshHours = 12
	}
	if subscriptionRefreshHours > 720 {
		writeError(w, http.StatusBadRequest, "subscription_refresh_hours must be between 1 and 720")
		return
	}

	normalizeURL := func(raw string, field string) (string, bool) {
		normalized, err := normalizeAbsoluteHTTPURL(raw, field)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return "", false
		}
		return normalized, true
	}

	subscriptionInfoURL, ok := normalizeURL(req.SubscriptionInfoURL, "subscription_info_url")
	if !ok {
		return
	}
	subscriptionExtraURL, ok := normalizeURL(req.SubscriptionExtraURL, "subscription_extra_url")
	if !ok {
		return
	}

	subscriptionExtraStatus := strings.TrimSpace(req.SubscriptionExtraStatus)
	if len(subscriptionExtraStatus) > 255 {
		writeError(w, http.StatusBadRequest, "subscription_extra_status is too long (max 255 characters)")
		return
	}

	_, err = a.db.Exec(
		`UPDATE users
		 SET status = ?,
		     starts_at = ?,
		     expires_at = ?,
		     blocked_reason = ?,
		     subscription_name = ?,
		     subscription_refresh_hours = ?,
		     subscription_info_url = ?,
		     subscription_extra_url = ?,
		     subscription_extra_status = ?
		 WHERE id = ?`,
		status,
		nullTimeValue(startsAt),
		nullTimeValue(expiresAt),
		nullStringValue(blockedReason),
		nullStringValue(subscriptionName),
		subscriptionRefreshHours,
		nullStringValue(subscriptionInfoURL),
		nullStringValue(subscriptionExtraURL),
		nullStringValue(subscriptionExtraStatus),
		id,
	)
	if err != nil {
		log.Printf("apiUpdateUserSubscription: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}
	a.recordAuditEvent(r, "user.subscription.update", "user", strconv.FormatInt(id, 10), map[string]any{"status": status})
	writeMessage(w, "subscription updated")
}

func (a *App) apiGetUserSubscriptionURLs(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var subscriptionID sql.NullString
	if err := a.db.QueryRow(`SELECT subscription_id FROM users WHERE id = ?`, id).Scan(&subscriptionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("apiGetUserSubscriptionURLs: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load user subscription")
		return
	}

	subID := strings.TrimSpace(subscriptionID.String)
	if subID == "" {
		writeError(w, http.StatusBadRequest, "subscription id is empty")
		return
	}

	plainURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subID)
	encryptedURL := ""
	if encrypted, err := a.encryptSubscriptionURL(plainURL); err == nil {
		encryptedURL = strings.TrimSpace(encrypted)
	} else {
		log.Printf("apiGetUserSubscriptionURLs: encrypt failed for user_id=%d: %v", id, err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"plain_url":     plainURL,
		"encrypted_url": encryptedURL,
	})
}

func (a *App) apiUpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateUserSettingsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	timeZone := strings.TrimSpace(req.TimeZone)
	if timeZone == "" {
		timeZone = "Europe/Moscow"
	}
	if len(timeZone) > 64 {
		writeError(w, http.StatusBadRequest, "time_zone is too long (max 64 characters)")
		return
	}
	if _, err := time.LoadLocation(timeZone); err != nil {
		writeError(w, http.StatusBadRequest, "time_zone must be a valid IANA timezone")
		return
	}

	language := strings.ToLower(strings.TrimSpace(req.Language))
	if language == "" {
		language = "ru"
	}
	switch language {
	case "ru", "en":
	default:
		writeError(w, http.StatusBadRequest, "language must be one of: ru, en")
		return
	}

	res, err := a.db.Exec(`UPDATE users SET time_zone = ?, language = ? WHERE id = ?`, timeZone, language, id)
	if err != nil {
		log.Printf("apiUpdateUserSettings: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update user settings")
		return
	}
	updated, _ := res.RowsAffected()
	if updated == 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	a.recordAuditEvent(r, "user.settings.update", "user", strconv.FormatInt(id, 10), map[string]any{
		"time_zone": timeZone,
		"language":  language,
	})
	writeMessage(w, "user settings updated")
}

func (a *App) apiGetPanelSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getPanelSettings()
	if err != nil {
		log.Printf("apiGetPanelSettings: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load panel settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) apiUpdatePanelSettings(w http.ResponseWriter, r *http.Request) {
	// Panel settings may contain base64-encoded images — allow up to 16 MB.
	var req model.PanelSettings
	if err := readJSONWithLimit(w, r, &req, 16<<20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.PanelTitle = strings.TrimSpace(req.PanelTitle)
	if req.PanelTitle == "" {
		req.PanelTitle = "SubShare"
	}
	if req.PageTitleAdmin == "" {
		req.PageTitleAdmin = "Панель управления — SubShare"
	}
	if req.PageTitleAdminLogin == "" {
		req.PageTitleAdminLogin = "Вход — SubShare"
	}
	if req.PageTitleSubscription == "" {
		req.PageTitleSubscription = "VPN-подписка — SubShare"
	}
	if err := a.updatePanelSettings(req); err != nil {
		log.Printf("apiUpdatePanelSettings: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save panel settings")
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (a *App) apiGetSubscriptionSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getSubscriptionSettings()
	if err != nil {
		log.Printf("apiGetSubscriptionSettings: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load subscription settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) apiUpdateSubscriptionSettings(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSubscriptionSettingsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "AllKeys"
	}
	if len(title) > 120 {
		writeError(w, http.StatusBadRequest, "title is too long (max 120 characters)")
		return
	}

	refreshHours := req.RefreshHours
	if refreshHours <= 0 {
		refreshHours = 12
	}
	if refreshHours > 720 {
		writeError(w, http.StatusBadRequest, "refresh_hours must be between 1 and 720")
		return
	}

	normalizeURL := func(raw string, field string) (string, bool) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return "", true
		}
		parsed, err := url.ParseRequestURI(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			writeError(w, http.StatusBadRequest, field+" must be a valid absolute URL")
			return "", false
		}
		return raw, true
	}

	infoURL, ok := normalizeURL(req.InfoURL, "info_url")
	if !ok {
		return
	}
	extraURL, ok := normalizeURL(req.ExtraURL, "extra_url")
	if !ok {
		return
	}

	extraStatus := strings.TrimSpace(req.ExtraStatus)
	if len(extraStatus) > 255 {
		writeError(w, http.StatusBadRequest, "extra_status is too long (max 255 characters)")
		return
	}
	subscriptionFormat, ok := model.NormalizeSubscriptionFormat(req.SubscriptionFormat)
	if !ok {
		writeError(w, http.StatusBadRequest, "subscription_format must be one of: links, xray-json")
		return
	}

	timeZone := strings.TrimSpace(req.TimeZone)
	if timeZone == "" {
		timeZone = "Europe/Moscow"
	}
	if len(timeZone) > 64 {
		writeError(w, http.StatusBadRequest, "time_zone is too long (max 64 characters)")
		return
	}
	if _, err := time.LoadLocation(timeZone); err != nil {
		writeError(w, http.StatusBadRequest, "time_zone must be a valid IANA timezone")
		return
	}

	language := strings.ToLower(strings.TrimSpace(req.Language))
	if language == "" {
		language = "ru"
	}
	switch language {
	case "ru", "en":
	default:
		writeError(w, http.StatusBadRequest, "language must be one of: ru, en")
		return
	}

	providerID := strings.TrimSpace(req.ProviderID)
	if providerID != "" && !providerIDPattern.MatchString(providerID) {
		writeError(w, http.StatusBadRequest, "provider_id must match ^[A-Za-z0-9]{8}$")
		return
	}

	happNoLimitMode := req.HappNoLimitMode
	happNoLimitModeXHTTPOnly := req.HappNoLimitModeXHTTPOnly
	happMandatoryHWID := req.HappMandatoryHWID
	happNotifyExpiration := req.HappNotifyExpiration
	happHideServerSettings := req.HappHideServerSettings
	happSubscriptionBody := req.HappSubscriptionBody
	if len(happSubscriptionBody) > 10000 {
		writeError(w, http.StatusBadRequest, "happ_subscription_body is too long (max 10000 characters)")
		return
	}
	if _, err := a.db.Exec(
		`UPDATE subscription_settings
		 SET title = ?, refresh_hours = ?, info_url = ?, extra_url = ?, extra_status = ?, subscription_format = ?, time_zone = ?, language = ?,
		     provider_id = ?, happ_no_limit_mode = ?, happ_no_limit_mode_xhttp_only = ?, happ_mandatory_hwid = ?,
		     happ_notify_expiration = ?, happ_hide_server_settings = ?, happ_subscription_body = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = 1`,
		title,
		refreshHours,
		nullStringValue(infoURL),
		nullStringValue(extraURL),
		nullStringValue(extraStatus),
		subscriptionFormat,
		timeZone,
		language,
		nullStringValue(providerID),
		boolToInt(happNoLimitMode),
		boolToInt(happNoLimitModeXHTTPOnly),
		boolToInt(happMandatoryHWID),
		boolToInt(happNotifyExpiration),
		boolToInt(happHideServerSettings),
		nullStringValue(happSubscriptionBody),
	); err != nil {
		log.Printf("apiUpdateSubscriptionSettings: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update subscription settings")
		return
	}

	writeMessage(w, "subscription settings updated")
}

func (a *App) apiGetRoutingSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getRoutingSettings()
	if err != nil {
		log.Printf("apiGetRoutingSettings: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load routing settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) apiUpdateRoutingSettings(w http.ResponseWriter, r *http.Request) {
	var req model.RoutingSettings
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	configJSON := strings.TrimSpace(req.ConfigJSON)
	if configJSON != "" {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(configJSON), &parsed); err != nil {
			writeError(w, http.StatusBadRequest, "config_json must be a valid JSON object")
			return
		}
	}

	if err := a.updateRoutingSettings(model.RoutingSettings{ConfigJSON: configJSON}); err != nil {
		log.Printf("apiUpdateRoutingSettings: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update routing settings")
		return
	}

	writeMessage(w, "routing settings updated")
}

func (a *App) apiUpdateUserHWID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateHWIDRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MaxDevices < 0 || req.MaxDevices > 32 {
		writeError(w, http.StatusBadRequest, "max_devices must be between 0 and 32 (0 means unlimited)")
		return
	}

	if _, err := a.db.Exec(`UPDATE users SET max_devices = ? WHERE id = ?`, req.MaxDevices, id); err != nil {
		log.Printf("apiUpdateUserHWID: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update hwid settings")
		return
	}
	a.recordAuditEvent(r, "user.hwid.update", "user", strconv.FormatInt(id, 10), map[string]any{"max_devices": req.MaxDevices})
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

	res, err := a.db.Exec(`DELETE FROM user_devices WHERE user_id = ? AND hwid = ?`, id, hwid)
	if err != nil {
		log.Printf("apiDeleteUserHWID: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete hwid")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeError(w, http.StatusNotFound, "hwid not found")
		return
	}
	a.recordAuditEvent(r, "user.hwid.delete", "user", strconv.FormatInt(id, 10), nil)
	writeMessage(w, "hwid removed")
}

// --- Keys API ---

func (a *App) legacyKeyHandler() *httpapi.LegacyKeyHandler {
	return httpapi.NewLegacyKeyHandler(a.keyService(), a.recordAuditEvent)
}

func (a *App) apiListKeys(w http.ResponseWriter, r *http.Request) {
	a.legacyKeyHandler().ListKeys(w, r)
}

func (a *App) apiListKeyCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.listKeyCategories()
	if err != nil {
		log.Printf("apiListKeyCategories: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list key categories")
		return
	}
	if categories == nil {
		categories = []model.KeyCategory{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
	})
}

func (a *App) apiCreateKeyCategory(w http.ResponseWriter, r *http.Request) {
	var req model.CreateKeyCategoryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rawName := strings.TrimSpace(req.Name)
	color := normalizeKeyCategoryColor(req.Color)
	if rawName == "" {
		writeError(w, http.StatusBadRequest, "category name is required")
		return
	}

	name := normalizeKeyCategory(rawName)
	var nextSortOrder int64
	if err := a.db.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextSortOrder); err != nil {
		log.Printf("apiCreateKeyCategory load nextSortOrder: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create key category")
		return
	}
	if _, err := a.db.Exec(
		`INSERT INTO key_categories(name, color, sort_order, updated_at)
		 VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET color = excluded.color, updated_at = CURRENT_TIMESTAMP`,
		name,
		color,
		nextSortOrder,
	); err != nil {
		log.Printf("apiCreateKeyCategory: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create key category")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"category": model.KeyCategory{Name: name, Color: color},
		"message":  "key category saved",
	})
}

func (a *App) apiUpdateKeyCategory(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateKeyCategoryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	oldRaw := strings.TrimSpace(req.OldName)
	newRaw := strings.TrimSpace(req.NewName)
	color := normalizeKeyCategoryColor(req.Color)
	if oldRaw == "" || newRaw == "" {
		writeError(w, http.StatusBadRequest, "both old_name and new_name are required")
		return
	}

	oldName := normalizeKeyCategory(oldRaw)
	newName := normalizeKeyCategory(newRaw)
	if oldName == "" || newName == "" {
		writeError(w, http.StatusBadRequest, "category name cannot be empty")
		return
	}

	var keyCount int64
	if err := a.db.QueryRow(`
		SELECT COUNT(*) FROM vless_keys
		 WHERE category_id = (SELECT id FROM key_categories WHERE name = ?)
		    OR (category_id IS NULL AND category = ?)
	`, oldName, oldName).Scan(&keyCount); err != nil {
		log.Printf("apiUpdateKeyCategory count keys: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update key category")
		return
	}
	var categoryCount int64
	var existingSortOrder int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM key_categories WHERE name = ?`, oldName).Scan(&categoryCount); err != nil {
		log.Printf("apiUpdateKeyCategory count categories: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update key category")
		return
	}
	_ = a.db.QueryRow(`SELECT COALESCE(sort_order, 0) FROM key_categories WHERE name = ?`, oldName).Scan(&existingSortOrder)
	if keyCount == 0 && categoryCount == 0 {
		writeError(w, http.StatusNotFound, "key category not found")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update key category")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO key_categories(name, color, sort_order, updated_at)
		 VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET color = excluded.color, sort_order = COALESCE(NULLIF(key_categories.sort_order, 0), excluded.sort_order), updated_at = CURRENT_TIMESTAMP`,
		newName,
		color,
		existingSortOrder,
	); err != nil {
		log.Printf("apiUpdateKeyCategory upsert new category: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update key category")
		return
	}

	if oldName != newName {
		var newCategoryID int64
		if err := tx.QueryRow(`SELECT id FROM key_categories WHERE name = ?`, newName).Scan(&newCategoryID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve key category")
			return
		}
		if _, err := tx.Exec(
			`UPDATE vless_keys
			 SET category_id = ?, category = ?
			 WHERE category_id = (SELECT id FROM key_categories WHERE name = ?)
			    OR (category_id IS NULL AND category = ?)`,
			newCategoryID,
			newName,
			oldName,
			oldName,
		); err != nil {
			log.Printf("apiUpdateKeyCategory update keys: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to update key category")
			return
		}
		if _, err := tx.Exec(`
			UPDATE external_subscription_sources
			   SET key_category_id = ?, key_category = ?
			 WHERE key_category_id = (SELECT id FROM key_categories WHERE name = ?)
			    OR (key_category_id IS NULL AND key_category = ?)
		`, newCategoryID, newName, oldName, oldName); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update source key category")
			return
		}

		if _, err := tx.Exec(`DELETE FROM key_categories WHERE name = ?`, oldName); err != nil {
			log.Printf("apiUpdateKeyCategory delete old category: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to update key category")
			return
		}
	}

	if _, err := tx.Exec(`UPDATE key_categories SET color = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`, color, newName); err != nil {
		log.Printf("apiUpdateKeyCategory update color: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update key category")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update key category")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"category": model.KeyCategory{Name: newName, Color: color},
		"message":  "key category updated",
	})
}

func (a *App) apiRenameKeyCategory(w http.ResponseWriter, r *http.Request) {
	var req model.RenameKeyCategoryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var currentColor sql.NullString
	_ = a.db.QueryRow(`SELECT color FROM key_categories WHERE name = ?`, normalizeKeyCategory(req.OldName)).Scan(&currentColor)

	a.apiUpdateKeyCategory(w, withJSONBody(r, model.UpdateKeyCategoryRequest{
		OldName: req.OldName,
		NewName: req.NewName,
		Color:   currentColor.String,
	}))
}

func withJSONBody[T any](r *http.Request, payload T) *http.Request {
	body, _ := json.Marshal(payload)
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(strings.NewReader(string(body)))
	clone.ContentLength = int64(len(body))
	return clone
}

func (a *App) apiDeleteKeyCategory(w http.ResponseWriter, r *http.Request) {
	var req model.DeleteKeyCategoryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := normalizeKeyCategory(req.Name)
	mode := strings.TrimSpace(req.Mode)
	if mode != "delete_with_keys" && mode != "keep_keys" {
		writeError(w, http.StatusBadRequest, "invalid delete mode")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete key category")
		return
	}
	defer tx.Rollback()
	var categoryID sql.NullInt64
	if err := tx.QueryRow(`SELECT id FROM key_categories WHERE name = ?`, name).Scan(&categoryID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to resolve key category")
		return
	}

	if mode == "delete_with_keys" {
		if _, err := tx.Exec(`
			DELETE FROM vless_keys
			 WHERE category_id = ? OR (category_id IS NULL AND category = ?)
		`, categoryID, name); err != nil {
			log.Printf("apiDeleteKeyCategory delete keys: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to delete key category")
			return
		}
	} else {
		if _, err := tx.Exec(`
			UPDATE vless_keys SET category_id = NULL, category = ''
			 WHERE category_id = ? OR (category_id IS NULL AND category = ?)
		`, categoryID, name); err != nil {
			log.Printf("apiDeleteKeyCategory move keys: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to delete key category")
			return
		}
	}

	if _, err := tx.Exec(`DELETE FROM key_categories WHERE name = ?`, name); err != nil {
		log.Printf("apiDeleteKeyCategory delete category: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete key category")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete key category")
		return
	}

	writeMessage(w, "key category deleted")
}

func (a *App) apiReorderKeyCategories(w http.ResponseWriter, r *http.Request) {
	var req model.ReorderKeyCategoriesRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Names) == 0 {
		writeError(w, http.StatusBadRequest, "category names are required")
		return
	}

	normalized := make([]string, 0, len(req.Names))
	seen := make(map[string]struct{}, len(req.Names))
	for _, name := range req.Names {
		value := normalizeKeyCategory(name)
		if value == "" {
			writeError(w, http.StatusBadRequest, "category name cannot be empty")
			return
		}
		if _, exists := seen[value]; exists {
			writeError(w, http.StatusBadRequest, "duplicate category names")
			return
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reorder key categories")
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE key_categories SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reorder key categories")
		return
	}
	defer stmt.Close()

	for index, name := range normalized {
		if _, err := stmt.Exec(index+1, name); err != nil {
			log.Printf("apiReorderKeyCategories update %s: %v", name, err)
			writeError(w, http.StatusInternalServerError, "failed to reorder key categories")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reorder key categories")
		return
	}

	writeMessage(w, "key categories reordered")
}

func (a *App) apiCreateKey(w http.ResponseWriter, r *http.Request) {
	a.legacyKeyHandler().CreateKey(w, r)
}

func (a *App) apiUpdateKey(w http.ResponseWriter, r *http.Request) {
	a.legacyKeyHandler().UpdateKey(w, r)
}

func (a *App) apiBulkUpdateKeyStatus(w http.ResponseWriter, r *http.Request) {
	var req model.BulkUpdateKeyStatusRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ids, err := normalizeBulkKeyIDs(req.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	status, ok := model.NormalizeKeyStatus(req.Status)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid key status")
		return
	}
	applyCategory := strings.TrimSpace(req.Category) != ""
	category := normalizeKeyCategory(req.Category)
	var categoryID any
	if applyCategory {
		if err := a.upsertKeyCategory(category); err != nil {
			log.Printf("apiBulkUpdateKeyStatus upsert category: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to save key category")
			return
		}
		categoryID, err = a.keyCategoryID(category)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to resolve key category")
			return
		}
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update keys")
		return
	}
	defer tx.Rollback()

	query := `UPDATE vless_keys SET status = ? WHERE id = ?`
	if applyCategory {
		query = `UPDATE vless_keys SET status = ?, category_id = ?, category = ? WHERE id = ?`
	}
	stmt, err := tx.Prepare(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update keys")
		return
	}
	defer stmt.Close()

	for _, id := range ids {
		var (
			res sql.Result
			err error
		)
		if applyCategory {
			res, err = stmt.Exec(status, categoryID, category, id)
		} else {
			res, err = stmt.Exec(status, id)
		}
		if err != nil {
			log.Printf("apiBulkUpdateKeyStatus: update key id=%d: %v", id, err)
			writeError(w, http.StatusInternalServerError, "failed to update keys")
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			writeError(w, http.StatusNotFound, fmt.Sprintf("key not found: %d", id))
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("apiBulkUpdateKeyStatus: commit: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update keys")
		return
	}

	a.recordAuditEvent(r, "keys.bulk_status", "key", "multiple", map[string]any{
		"count":  len(ids),
		"status": status,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "keys updated",
		"updated": len(ids),
	})
}

func (a *App) apiBulkDeleteKeys(w http.ResponseWriter, r *http.Request) {
	var req model.BulkDeleteKeysRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ids, err := normalizeBulkKeyIDs(req.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete keys")
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`DELETE FROM vless_keys WHERE id = ?`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete keys")
		return
	}
	defer stmt.Close()

	for _, id := range ids {
		res, err := stmt.Exec(id)
		if err != nil {
			log.Printf("apiBulkDeleteKeys: delete key id=%d: %v", id, err)
			writeError(w, http.StatusInternalServerError, "failed to delete keys")
			return
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			writeError(w, http.StatusNotFound, fmt.Sprintf("key not found: %d", id))
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("apiBulkDeleteKeys: commit: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete keys")
		return
	}

	a.recordAuditEvent(r, "keys.bulk_delete", "key", "multiple", map[string]any{"count": len(ids)})
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "keys deleted",
		"deleted": len(ids),
	})
}

func (a *App) apiReorderKeys(w http.ResponseWriter, r *http.Request) {
	var req model.ReorderKeysRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rows, err := a.db.Query(`SELECT id FROM vless_keys ORDER BY sort_order, id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load keys")
		return
	}
	defer rows.Close()

	existingIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read keys")
			return
		}
		existingIDs = append(existingIDs, id)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read keys")
		return
	}

	if len(existingIDs) != len(req.IDs) {
		writeError(w, http.StatusBadRequest, "ids list must include all keys")
		return
	}

	allowed := make(map[int64]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		allowed[id] = struct{}{}
	}
	seen := make(map[int64]struct{}, len(req.IDs))
	for _, id := range req.IDs {
		if _, ok := allowed[id]; !ok {
			writeError(w, http.StatusBadRequest, "ids list contains unknown key")
			return
		}
		if _, ok := seen[id]; ok {
			writeError(w, http.StatusBadRequest, "ids list contains duplicates")
			return
		}
		seen[id] = struct{}{}
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reorder keys")
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE vless_keys SET sort_order = ? WHERE id = ?`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reorder keys")
		return
	}
	defer stmt.Close()

	for index, id := range req.IDs {
		if _, err := stmt.Exec(index+1, id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to reorder keys")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reorder keys")
		return
	}

	a.recordAuditEvent(r, "keys.reorder", "key", "multiple", map[string]any{"count": len(req.IDs)})
	writeMessage(w, "keys reordered")
}

func (a *App) apiDeleteKey(w http.ResponseWriter, r *http.Request) {
	a.legacyKeyHandler().DeleteKey(w, r)
}

func (a *App) apiCheckKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var encURL sql.NullString
	var kind string
	if err := a.db.QueryRow(`SELECT s.encrypted_url, k.key_kind FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = ?`, id).Scan(&encURL, &kind); err != nil {
		writeError(w, http.StatusNotFound, "key not found")
		return
	}
	if normalizedKind, _ := model.NormalizeKeyKind(kind); normalizedKind == model.KeyKindInformational {
		writeError(w, http.StatusBadRequest, "informational keys do not require checks")
		return
	}
	if !encURL.Valid || encURL.String == "" {
		writeError(w, http.StatusInternalServerError, "missing profile key secret")
		return
	}
	sec, err := profilestorage.Decrypt(encURL.String, a.profileKeyring, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decrypt key")
		return
	}

	if err := a.checkAndPersistKey(id, sec.Reveal()); err != nil {
		log.Printf("apiCheckKey: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check key")
		return
	}
	a.respondJSONKeyCheck(w, id)
}

func (a *App) apiCheckAllKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`SELECT k.id, s.encrypted_url, k.key_kind FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id ORDER BY CASE WHEN k.key_kind = 'real' THEN 0 ELSE 1 END, k.sort_order, k.id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load keys")
		return
	}
	defer rows.Close()

	type keyRow struct {
		id   int64
		url  string
		kind string
	}
	keys := make([]keyRow, 0)
	for rows.Next() {
		var id int64
		var encURL sql.NullString
		var kind string
		if err := rows.Scan(&id, &encURL, &kind); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read keys")
			return
		}
		normalizedKind, _ := model.NormalizeKeyKind(kind)
		if normalizedKind == model.KeyKindInformational {
			continue
		}
		if !encURL.Valid || encURL.String == "" {
			continue
		}
		sec, err := profilestorage.Decrypt(encURL.String, a.profileKeyring, id)
		if err != nil {
			continue
		}
		keys = append(keys, keyRow{id: id, url: sec.Reveal(), kind: kind})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read keys")
		return
	}

	jobID := a.startTrackedJob("keys_health_check", "key", "all")
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	checked := 0
	for _, key := range keys {
		checked++
		wg.Add(1)
		sem <- struct{}{}
		go func(id int64, url string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := a.checkAndPersistKey(id, url); err != nil {
				log.Printf("checkAndPersistKey(%d): %v", id, err)
			}
		}(key.id, key.url)
	}
	wg.Wait()

	updatedRows, err := a.db.Query(`
		SELECT id, check_status, check_error, last_checked_at, last_latency_ms
		FROM vless_keys ORDER BY CASE WHEN key_kind = 'real' THEN 0 ELSE 1 END, sort_order, id
	`)
	if err != nil {
		a.finishTrackedJob(jobID, err)
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
			a.finishTrackedJob(jobID, err)
			writeError(w, http.StatusInternalServerError, "failed to load key checks")
			return
		}
		payload := map[string]any{
			"id":                 id,
			"check_status":       model.NormalizeCheckStatus(checkStatus.String),
			"check_status_label": model.CheckStatusLabel(model.NormalizeCheckStatus(checkStatus.String)),
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
		a.finishTrackedJob(jobID, err)
		writeError(w, http.StatusInternalServerError, "failed to load key checks")
		return
	}
	a.finishTrackedJob(jobID, nil)
	a.recordAuditEvent(r, "keys.health_check", "key", "all", map[string]any{"checked": checked})
	writeJSON(w, http.StatusOK, map[string]any{"checked": checked, "keys": results})
}

// --- Subscription API ---

func (a *App) apiActivateSubscription(w http.ResponseWriter, r *http.Request) {
	var req model.ActivateRequest
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

	subscriptionURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subscriptionID)
	if strings.TrimSpace(a.happCryptoAPIURL) != "" {
		if encryptedURL, err := a.encryptSubscriptionURL(subscriptionURL); err == nil && strings.TrimSpace(encryptedURL) != "" {
			subscriptionURL = encryptedURL
		} else if err != nil {
			log.Printf("apiActivateSubscription: failed to encrypt url via configured Happ API: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"subscription_url": subscriptionURL,
		"message":          "Ключ активирован. Ссылка готова — скопируйте и вставьте её в VPN-клиент",
	})
}

// --- Subscription delivery (unchanged --- serves plaintext for VPN clients) ---

func (a *App) handleSubscription(w http.ResponseWriter, r *http.Request) {
	subscriptionID := r.PathValue("subscription_id")
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}

	delivery, denyCode, denyStatus, denyReason, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		log.Printf("prepare subscription delivery: %v", err)
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if denyCode != 0 {
		writeSubscriptionDenial(w, denyCode, denyStatus, denyReason)
		a.incrementSubscriptionMetric(denyStatus, "denied")
		return
	}
	rule := delivery.Rule
	if (rule != nil && rule.ResponseType == "browser") || (rule == nil && isBrowserSubscriptionRequest(r)) {
		a.renderSubscriptionBrowserPage(w, r, subscriptionID)
		a.incrementSubscriptionMetric("browser", "success")
		return
	}

	responseType := ""
	if rule != nil {
		responseType = rule.ResponseType
	}
	generated, settings, denyCode, denyReason, err := a.generateSelectedSubscription(subscriptionID, responseType)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	applyGenerationExclusionHeaders(w.Header(), generated.Exclusions)
	if denyCode != 0 {
		if denyCode == http.StatusUnprocessableEntity && denyReason == generationReasonAllExcluded {
			writeJSON(w, denyCode, generationFailurePayload(generated))
			a.incrementSubscriptionMetric(generated.OutputFormat, "all_excluded")
			return
		}
		status := remarkStatusFromReason(denyReason)
		writeSubscriptionDenial(w, denyCode, status, a.subscriptionRemark(denyReason))
		a.incrementSubscriptionMetric(status, "denied")
		return
	}

	a.applySubscriptionResponseHeaders(w, r, settings, subscriptionID)
	a.applyGlobalDeliveryHeaders(w)
	body := generated.Body
	if rule != nil {
		if template, loadErr := a.loadTemplate(rule.TemplateID); loadErr == nil && template != nil && template.Enabled {
			body = applyTemplateContent(template.Content, body, settings.Title)
		}
		applyRuleHeaders(w, rule.Headers)
	}
	if err := validateGeneratedStructuredBody(responseType, body); err != nil {
		http.Error(w, "failed to render subscription format", http.StatusUnprocessableEntity)
		a.incrementSubscriptionMetric(responseType, "render_failed")
		return
	}
	switch responseType {
	case "base64":
		body = encodeBase64Subscription(body)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	case "plain":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	case "mihomo":
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.yaml"`, sanitizeSubscriptionFilenamePart(settings.Title)))
	case "sing-box", "xray-json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, sanitizeSubscriptionFilenamePart(settings.Title)))
	default:
		if a.subscriptionBodyEncoding == "base64" && settings.SubscriptionFormat != model.SubscriptionFormatXrayJSON {
			body = base64.StdEncoding.EncodeToString([]byte(body))
			responseType = "base64"
		} else if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
			responseType = "xray-json"
		} else {
			responseType = "plain"
		}
	}
	_, _ = w.Write([]byte(body))
	a.incrementSubscriptionMetric(responseType, "success")
}

func sanitizeSubscriptionFilenamePart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "subscription"
	}

	var builder strings.Builder
	for _, ch := range raw {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
		case ch >= 'A' && ch <= 'Z':
			builder.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
		case ch == '-', ch == '_', ch == '.':
			builder.WriteRune(ch)
		default:
			builder.WriteRune('_')
		}
	}

	name := strings.Trim(strings.TrimSpace(builder.String()), "._")
	if name == "" {
		return "subscription"
	}
	return name
}

func buildSubscriptionAttachmentFilename(settings model.SubscriptionSettings) string {
	base := sanitizeSubscriptionFilenamePart(settings.Title)
	if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		return base + ".json"
	}
	return base + ".txt"
}

func (a *App) applySubscriptionResponseHeaders(
	w http.ResponseWriter,
	r *http.Request,
	settings model.SubscriptionSettings,
	subscriptionID string,
) {
	if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}

	filename := buildSubscriptionAttachmentFilename(settings)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if title := strings.TrimSpace(settings.Title); title != "" {
		w.Header().Set("profile-title", "base64:"+base64.StdEncoding.EncodeToString([]byte(title)))
	}
	if settings.RefreshHours > 0 {
		w.Header().Set("profile-update-interval", strconv.Itoa(settings.RefreshHours))
	}
	subscriptionURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subscriptionID)
	profileURL := strings.TrimSpace(settings.InfoURL)
	if profileURL == "" {
		profileURL = subscriptionURL
	}
	w.Header().Set("profile-web-page-url", profileURL)
	w.Header().Set("Subscription-Status", "active")
	if providerID := strings.TrimSpace(settings.ProviderID); providerID != "" {
		w.Header().Set("providerid", providerID)
	}
	if extraURL := strings.TrimSpace(settings.ExtraURL); extraURL != "" {
		w.Header().Set("support-url", extraURL)
	}
	if extraStatus := strings.TrimSpace(settings.ExtraStatus); extraStatus != "" {
		w.Header().Set("announce", "base64:"+base64.StdEncoding.EncodeToString([]byte(extraStatus)))
	}
}

func (a *App) handleSubscriptionSubBody(w http.ResponseWriter, r *http.Request) {
	subscriptionID := strings.TrimSpace(r.PathValue("subscription_id"))
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}

	_, code, status, reason, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if code != 0 {
		writeSubscriptionDenial(w, code, status, reason)
		a.incrementSubscriptionMetric(status, "denied")
		return
	}

	generated, settings, denyCode, denyReason, err := a.generateSelectedSubscription(subscriptionID, "plain")
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	applyGenerationExclusionHeaders(w.Header(), generated.Exclusions)
	if denyCode != 0 {
		status = remarkStatusFromReason(denyReason)
		writeSubscriptionDenial(w, denyCode, status, a.subscriptionRemark(denyReason))
		a.incrementSubscriptionMetric(status, "denied")
		return
	}

	a.applySubscriptionResponseHeaders(w, r, settings, subscriptionID)
	a.applyGlobalDeliveryHeaders(w)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(encodeBase64Subscription(generated.Body)))
	a.incrementSubscriptionMetric("subbody-base64", "success")
}

func (a *App) handleSubscriptionSubBodyPlain(w http.ResponseWriter, r *http.Request) {
	subscriptionID := strings.TrimSpace(r.PathValue("subscription_id"))
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}

	_, code, status, reason, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if code != 0 {
		writeSubscriptionDenial(w, code, status, reason)
		a.incrementSubscriptionMetric(status, "denied")
		return
	}

	generated, settings, denyCode, denyReason, err := a.generateSelectedSubscription(subscriptionID, "plain")
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	applyGenerationExclusionHeaders(w.Header(), generated.Exclusions)
	if denyCode != 0 {
		status = remarkStatusFromReason(denyReason)
		writeSubscriptionDenial(w, denyCode, status, a.subscriptionRemark(denyReason))
		a.incrementSubscriptionMetric(status, "denied")
		return
	}

	a.applySubscriptionResponseHeaders(w, r, settings, subscriptionID)
	a.applyGlobalDeliveryHeaders(w)
	if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	_, _ = w.Write([]byte(generated.Body))
	a.incrementSubscriptionMetric("subbody-plain", "success")
}

func (a *App) buildSubscriptionBodyPlain(
	r *http.Request,
	subscriptionID string,
) (string, model.SubscriptionSettings, int, string, error) {
	return a.buildSubscriptionBodyPlainForFormat(r, subscriptionID, "")
}

func (a *App) buildSubscriptionBodyPlainForFormat(
	r *http.Request,
	subscriptionID string,
	responseType string,
) (string, model.SubscriptionSettings, int, string, error) {
	generated, settings, denyCode, denyReason, err := a.generateSelectedSubscription(subscriptionID, responseType)
	return generated.Body, settings, denyCode, denyReason, err
}

func isBrowserSubscriptionRequest(r *http.Request) bool {
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	if !strings.Contains(accept, "text/html") {
		return false
	}

	secFetchDest := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Dest")))
	if secFetchDest != "" && secFetchDest != "document" {
		return false
	}

	return true
}

func (a *App) renderSubscriptionBrowserPage(w http.ResponseWriter, r *http.Request, subscriptionID string) {
	allowed, _, code, reason, err := a.subscriptionAccessAllowed(subscriptionID)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, reason, code)
		return
	}

	panelSettings, err := a.getPanelSettings()
	if err != nil {
		panelSettings = model.PanelSettings{
			PanelTitle:            "SubShare",
			PageTitleSubscription: "VPN-подписка — SubShare",
		}
	}

	pageTitle := strings.TrimSpace(panelSettings.PageTitleSubscription)
	if pageTitle == "" {
		pageTitle = "VPN-подписка — SubShare"
	}

	subscriptionURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subscriptionID)
	importSubscriptionURL := subscriptionURL
	if encryptedURL, err := a.encryptSubscriptionURL(subscriptionURL); err == nil && strings.TrimSpace(encryptedURL) != "" {
		importSubscriptionURL = encryptedURL
	} else if err != nil {
		log.Printf("renderSubscriptionBrowserPage: failed to encrypt url via happ api: %v", err)
	}

	cfg, _, err := normalizeSubscriptionPageConfig(panelSettings.SubscriptionPageConfig)
	if err != nil {
		log.Printf("renderSubscriptionBrowserPage: invalid subscription page config, using default: %v", err)
		cfg = defaultSubscriptionPageConfig()
	}

	htmlDoc := renderSubscriptionPageHTML(
		cfg,
		panelSettings,
		pageTitle,
		strings.TrimSpace(panelSettings.FaviconDataURL),
		strings.TrimSpace(panelSettings.LogoDataURL),
		subscriptionURL,
		importSubscriptionURL,
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Subscription-Status", "active")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' data: https: http:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	_, _ = w.Write([]byte(htmlDoc))
}

// --- Data export API ---

func (a *App) apiExportUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.listUsers()
	if err != nil {
		log.Printf("apiExportUsers: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to export users")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="users.json"`)
	writeJSON(w, http.StatusOK, users)
}

func (a *App) apiExportKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := a.listKeys()
	if err != nil {
		log.Printf("apiExportKeys: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to export keys")
		return
	}
	if keys == nil {
		keys = []model.VLESSKey{}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="keys.json"`)
	writeJSON(w, http.StatusOK, keys)
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
	status := model.NormalizeCheckStatus(checkStatus.String)
	payload := map[string]any{
		"id":                 keyID,
		"check_status":       status,
		"check_status_label": model.CheckStatusLabel(status),
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
func (a *App) apiGetSubscriptionInfo(w http.ResponseWriter, r *http.Request) {
	subscriptionID := r.PathValue("subscription_id")
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}
	delivery, denyCode, denyStatus, denyReason, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		http.Error(w, "failed to load subscription info", http.StatusInternalServerError)
		return
	}
	if denyCode != 0 {
		writeSubscriptionDenial(w, denyCode, denyStatus, denyReason)
		a.incrementSubscriptionMetric(denyStatus, "denied")
		return
	}
	w.Header().Set("Subscription-Status", "active")
	if providerID := strings.TrimSpace(delivery.Settings.ProviderID); providerID != "" {
		w.Header().Set("providerid", providerID)
	}

	var user struct {
		Name      string `json:"name"`
		Status    string `json:"status"`
		ExpiresAt string `json:"expires_at,omitempty"`
	}

	var expiresAt sql.NullTime
	err = a.db.QueryRow(`
SELECT name, status, expires_at
FROM users
WHERE subscription_id = ?
`, subscriptionID).Scan(&user.Name, &user.Status, &expiresAt)

	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		log.Printf("apiGetSubscriptionInfo: %v", err)
		http.Error(w, "failed to load subscription info", http.StatusInternalServerError)
		return
	}

	if expiresAt.Valid {
		user.ExpiresAt = expiresAt.Time.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, user)
	a.incrementSubscriptionMetric("info", "success")
}
