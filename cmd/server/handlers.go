package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/storage"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/middleware"
	"github.com/romanpodg/SubShare-Go/internal/model"
	adminpassword "github.com/romanpodg/SubShare-Go/internal/security/password"
)

var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{8}$`)

func applySensitiveResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

func normalizeKeyCategory(raw string) string {
	return keymanagement.NormalizeKeyCategory(raw)
}

func (a *App) upsertKeyCategory(category string) error {
	_, err := a.keyService().EnsureCategory(context.Background(), category)
	return err
}

// --- Auth API ---

func (a *App) apiLogin(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)
	adminID, authenticated, err := a.authenticateAdministrator(r.Context(), username, req.Password)
	if err != nil {
		log.Printf("apiLogin: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}
	if !authenticated {
		httpapi.WriteError(w, r, http.StatusUnauthorized, "invalid credentials")
		return
	}

	sessionID, err := generateToken(32)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create session")
		return
	}
	csrfToken, err := generateToken(32)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create session")
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	_, err = a.db.Exec(`INSERT INTO admin_sessions(id, admin_id, csrf_token, expires_at) VALUES(?, ?, ?, ?)`, hashSessionID(sessionID), adminID, csrfToken, expiresAt)
	if err != nil {
		log.Printf("apiLogin: failed to save session to database: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create session")
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
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"csrf_token": csrfToken})
}

func (a *App) apiLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(model.AdminSessionCookieName)
	if err == nil && cookie.Value != "" {
		_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE id = ?`, hashSessionID(cookie.Value))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     model.AdminSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	httpapi.WriteMessage(w, "logged out")
}

func (a *App) apiMe(w http.ResponseWriter, r *http.Request) {
	session, _, _ := a.adminSessionFromRequest(r)
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
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
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to query admins")
		return
	}
	defer rows.Close()

	var admins []model.Admin
	for rows.Next() {
		var adm model.Admin
		if err := rows.Scan(&adm.ID, &adm.Username, &adm.Role, &adm.CreatedAt); err != nil {
			log.Printf("apiListAdmins scan: %v", err)
			httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to scan admins")
			return
		}
		admins = append(admins, adm)
	}

	if admins == nil {
		admins = []model.Admin{}
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"admins": admins})
}

func (a *App) apiCreateAdmin(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAdminRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)
	role, roleOK := normalizeAdminRole(req.Role)

	if username == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "username is required")
		return
	}
	if !roleOK {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid role")
		return
	}

	// Check if username already exists
	var exists bool
	err := a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM admins WHERE username = ?)`, username).Scan(&exists)
	if err != nil {
		log.Printf("apiCreateAdmin check exists: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "internal database error")
		return
	}
	if exists {
		httpapi.WriteError(w, r, http.StatusConflict, "username is already taken")
		return
	}

	passwordHash, err := a.passwordHasher().Hash(req.Password)
	if err != nil {
		if adminpassword.IsPolicyError(err) {
			httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("apiCreateAdmin: password hashing failed")
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to hash password")
		return
	}

	result, err := a.db.Exec(`INSERT INTO admins (username, password_hash, role) VALUES (?, ?, ?)`, username, passwordHash, role)
	if err != nil {
		log.Printf("apiCreateAdmin insert: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create admin")
		return
	}
	adminID, _ := result.LastInsertId()
	a.recordAuditEvent(r, "admin.create", "admin", strconv.FormatInt(adminID, 10), map[string]any{"username": username, "role": role})

	httpapi.WriteMessage(w, "administrator created successfully")
}

func (a *App) apiUpdateAdmin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid administrator id")
		return
	}

	var req model.UpdateAdminRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	role := strings.TrimSpace(req.Role)
	passwordValue := req.Password

	session, _, _ := a.adminSessionFromRequest(r)

	// sources.Fetch current admin info
	var currentUsername string
	var currentRole string
	err = a.db.QueryRow(`SELECT username, role FROM admins WHERE id = ?`, id).Scan(&currentUsername, &currentRole)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpapi.WriteError(w, r, http.StatusNotFound, "administrator not found")
			return
		}
		log.Printf("apiUpdateAdmin query: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "database error")
		return
	}

	// Update password if provided
	if passwordValue != "" {
		passwordHash, err := a.passwordHasher().Hash(passwordValue)
		if err != nil {
			if adminpassword.IsPolicyError(err) {
				httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
				return
			}
			log.Printf("apiUpdateAdmin: password hashing failed")
			httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to hash password")
			return
		}
		_, err = a.db.Exec(`UPDATE admins SET password_hash = ? WHERE id = ?`, passwordHash, id)
		if err != nil {
			log.Printf("apiUpdateAdmin password update: %v", err)
			httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update password")
			return
		}
		// Invalidate all active sessions for this admin since password changed, except the current one
		cookie, err := r.Cookie(model.AdminSessionCookieName)
		if err == nil && cookie.Value != "" {
			_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ? AND id != ?`, id, hashSessionID(cookie.Value))
		} else {
			_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, id)
		}
	}

	// Update role if provided
	if role != "" {
		normalizedRole, roleOK := normalizeAdminRole(role)
		if !roleOK {
			httpapi.WriteError(w, r, http.StatusBadRequest, "invalid role")
			return
		}
		role = normalizedRole

		// Prevent changing own role
		if id == session.AdminID {
			httpapi.WriteError(w, r, http.StatusBadRequest, "you cannot change your own role")
			return
		}

		// Prevent changing role of the last owner
		if isOwnerRole(currentRole) && !isOwnerRole(role) {
			var superAdminCount int
			err = a.db.QueryRow(`SELECT COUNT(*) FROM admins WHERE role IN ('owner', 'super_admin')`).Scan(&superAdminCount)
			if err != nil {
				log.Printf("apiUpdateAdmin count super admins: %v", err)
				httpapi.WriteError(w, r, http.StatusInternalServerError, "database error")
				return
			}
			if superAdminCount <= 1 {
				httpapi.WriteError(w, r, http.StatusBadRequest, "cannot demote the only remaining super admin")
				return
			}
		}

		_, err = a.db.Exec(`UPDATE admins SET role = ? WHERE id = ?`, role, id)
		if err != nil {
			log.Printf("apiUpdateAdmin role update: %v", err)
			httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update role")
			return
		}

		// Invalidate all sessions for the updated admin since role changed
		_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, id)
	}

	a.recordAuditEvent(r, "admin.update", "admin", strconv.FormatInt(id, 10), map[string]any{
		"role":             role,
		"password_changed": passwordValue != "",
	})
	httpapi.WriteMessage(w, "administrator updated successfully")
}

func (a *App) apiDeleteAdmin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid administrator id")
		return
	}

	session, _, _ := a.adminSessionFromRequest(r)

	// Prevent deleting oneself
	if id == session.AdminID {
		httpapi.WriteError(w, r, http.StatusBadRequest, "you cannot delete your own account")
		return
	}

	// sources.Fetch admin to delete
	var role string
	err = a.db.QueryRow(`SELECT role FROM admins WHERE id = ?`, id).Scan(&role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpapi.WriteError(w, r, http.StatusNotFound, "administrator not found")
			return
		}
		log.Printf("apiDeleteAdmin query: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "database error")
		return
	}

	// Prevent deleting the last owner
	if isOwnerRole(role) {
		var superAdminCount int
		err = a.db.QueryRow(`SELECT COUNT(*) FROM admins WHERE role IN ('owner', 'super_admin')`).Scan(&superAdminCount)
		if err != nil {
			log.Printf("apiDeleteAdmin count super admins: %v", err)
			httpapi.WriteError(w, r, http.StatusInternalServerError, "database error")
			return
		}
		if superAdminCount <= 1 {
			httpapi.WriteError(w, r, http.StatusBadRequest, "cannot delete the only remaining super admin")
			return
		}
	}

	// Delete sessions first
	_, err = a.db.Exec(`DELETE FROM admin_sessions WHERE admin_id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteAdmin delete sessions: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to delete sessions")
		return
	}

	// Delete admin
	res, err := a.db.Exec(`DELETE FROM admins WHERE id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteAdmin delete admin: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to delete admin")
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		httpapi.WriteError(w, r, http.StatusNotFound, "administrator not found")
		return
	}

	a.recordAuditEvent(r, "admin.delete", "admin", strconv.FormatInt(id, 10), nil)
	httpapi.WriteMessage(w, "administrator deleted successfully")
}

// --- Users API ---

func (a *App) apiListUsers(w http.ResponseWriter, r *http.Request) {
	applySensitiveResponseHeaders(w)
	users, err := a.listUsers()
	if err != nil {
		log.Printf("apiListUsers: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	// The legacy token column is retained for database compatibility, but it is
	// not used by current subscription delivery or the administration frontend.
	// Do not return that access material from bulk user projections.
	for i := range users {
		users[i].Token = ""
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (a *App) apiCreateUser(w http.ResponseWriter, r *http.Request) {
	var req model.CreateUserRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	activationCode := strings.TrimSpace(req.ActivationCode)
	status, ok := model.NormalizeUserStatus(req.Status)
	if !ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid subscription status")
		return
	}

	issueDays := req.IssueDays
	if issueDays <= 0 {
		issueDays = 30
	}
	if issueDays > 3650 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "issue days must be between 1 and 3650")
		return
	}

	blockedReason := strings.TrimSpace(req.BlockedReason)
	if status != model.UserStatusBlocked {
		blockedReason = ""
	}

	if name == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "name is required")
		return
	}
	if activationCode == "" || strings.Contains(activationCode, "/") {
		httpapi.WriteError(w, r, http.StatusBadRequest, "activation code is required")
		return
	}
	if len(name) > 255 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "name is too long (max 255 characters)")
		return
	}
	if len(email) > 255 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "email is too long (max 255 characters)")
		return
	}
	if len(activationCode) > 128 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "activation code is too long (max 128 characters)")
		return
	}

	legacyToken, err := generateToken(24)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to generate user token")
		return
	}

	subscriptionID, err := generateToken(24)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to generate subscription token")
		return
	}

	now := time.Now().UTC()
	expiresAt := now.AddDate(0, 0, issueDays)

	tx, err := a.db.Begin()
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create user")
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
				httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to generate subscription token")
				return
			}
			continue
		}
		break
	}
	if err != nil {
		log.Printf("apiCreateUser: %v", err)
		httpapi.WriteError(w, r, http.StatusConflict, "failed to create user (check activation code uniqueness)")
		return
	}

	if err := storage.AssignAllKeysToUser(context.Background(), tx, userID); err != nil {
		log.Printf("apiCreateUser: failed to assign keys to user %d: %v", userID, err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create user")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create user")
		return
	}

	a.recordAuditEvent(r, "user.create", "user", strconv.FormatInt(userID, 10), map[string]any{"name": name})
	httpapi.WriteMessage(w, "user created")
}

func (a *App) apiDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	res, err := a.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteUser: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to delete user")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
		return
	}
	a.recordAuditEvent(r, "user.delete", "user", strconv.FormatInt(id, 10), nil)
	httpapi.WriteMessage(w, "user deleted")
}

func (a *App) apiUpdateUserKeys(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateUserKeysRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := a.updateUserKeyAssignment(id, model.KeyAssignmentModeSelected, req.KeyIDs); err != nil {
		if errors.Is(err, errAssignmentUserNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, errAssignmentKeyNotFound) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "one or more keys do not exist")
			return
		}
		log.Printf("apiUpdateUserKeys: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update user keys")
		return
	}
	a.recordAuditEvent(r, "user.keys.update", "user", strconv.FormatInt(id, 10), map[string]any{
		"mode": model.KeyAssignmentModeSelected, "keys_count": len(req.KeyIDs),
	})
	httpapi.WriteMessage(w, "subscription keys updated")
}

func (a *App) apiUpdateUserSubscription(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateSubscriptionRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	status, ok := model.NormalizeUserStatus(req.Status)
	if !ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid subscription status")
		return
	}

	startsAt, err := parseOptionalDateTimeLocal(req.StartsAt)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid starts_at datetime")
		return
	}
	expiresAt, err := parseOptionalDateTimeLocal(req.ExpiresAt)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid expires_at datetime")
		return
	}

	if startsAt.Valid && expiresAt.Valid && startsAt.Time.After(expiresAt.Time) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "starts_at must be before expires_at")
		return
	}

	blockedReason := strings.TrimSpace(req.BlockedReason)
	if status != model.UserStatusBlocked {
		blockedReason = ""
	}

	subscriptionName := strings.TrimSpace(req.SubscriptionName)
	if len(subscriptionName) > 120 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription_name is too long (max 120 characters)")
		return
	}

	subscriptionRefreshHours := req.SubscriptionRefreshHours
	if subscriptionRefreshHours <= 0 {
		subscriptionRefreshHours = 12
	}
	if subscriptionRefreshHours > 720 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription_refresh_hours must be between 1 and 720")
		return
	}

	normalizeURL := func(raw string, field string) (string, bool) {
		normalized, err := normalizeAbsoluteHTTPURL(raw, field)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
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
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription_extra_status is too long (max 255 characters)")
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
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update subscription")
		return
	}
	a.recordAuditEvent(r, "user.subscription.update", "user", strconv.FormatInt(id, 10), map[string]any{"status": status})
	httpapi.WriteMessage(w, "subscription updated")
}

func (a *App) apiGetUserSubscriptionURLs(w http.ResponseWriter, r *http.Request) {
	applySensitiveResponseHeaders(w)
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var subscriptionID sql.NullString
	if err := a.db.QueryRow(`SELECT subscription_id FROM users WHERE id = ?`, id).Scan(&subscriptionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("apiGetUserSubscriptionURLs: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to load user subscription")
		return
	}

	subID := strings.TrimSpace(subscriptionID.String)
	if subID == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription id is empty")
		return
	}

	plainURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subID)
	encryptedURL := ""
	if encrypted, err := a.encryptSubscriptionURL(plainURL); err == nil {
		encryptedURL = strings.TrimSpace(encrypted)
	} else {
		log.Printf("apiGetUserSubscriptionURLs: encrypt failed for user_id=%d: %v", id, err)
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"plain_url":     plainURL,
		"encrypted_url": encryptedURL,
	})
}

func (a *App) apiUpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateUserSettingsRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	timeZone := strings.TrimSpace(req.TimeZone)
	if timeZone == "" {
		timeZone = "Europe/Moscow"
	}
	if len(timeZone) > 64 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "time_zone is too long (max 64 characters)")
		return
	}
	if _, err := time.LoadLocation(timeZone); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "time_zone must be a valid IANA timezone")
		return
	}

	language := strings.ToLower(strings.TrimSpace(req.Language))
	if language == "" {
		language = "ru"
	}
	switch language {
	case "ru", "en":
	default:
		httpapi.WriteError(w, r, http.StatusBadRequest, "language must be one of: ru, en")
		return
	}

	res, err := a.db.Exec(`UPDATE users SET time_zone = ?, language = ? WHERE id = ?`, timeZone, language, id)
	if err != nil {
		log.Printf("apiUpdateUserSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update user settings")
		return
	}
	updated, _ := res.RowsAffected()
	if updated == 0 {
		httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
		return
	}

	a.recordAuditEvent(r, "user.settings.update", "user", strconv.FormatInt(id, 10), map[string]any{
		"time_zone": timeZone,
		"language":  language,
	})
	httpapi.WriteMessage(w, "user settings updated")
}

func (a *App) apiGetPanelSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getPanelSettings()
	if err != nil {
		log.Printf("apiGetPanelSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to load panel settings")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, settings)
}

func (a *App) apiUpdatePanelSettings(w http.ResponseWriter, r *http.Request) {
	// Panel settings may contain base64-encoded images — allow up to 16 MB.
	var req model.PanelSettings
	if err := httpapi.ReadJSONWithLimit(w, r, &req, 16<<20); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
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
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to save panel settings")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, req)
}

func (a *App) apiGetSubscriptionSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getSubscriptionSettings()
	if err != nil {
		log.Printf("apiGetSubscriptionSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to load subscription settings")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, settings)
}

func (a *App) apiUpdateSubscriptionSettings(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateSubscriptionSettingsRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "AllKeys"
	}
	if len(title) > 120 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "title is too long (max 120 characters)")
		return
	}

	refreshHours := req.RefreshHours
	if refreshHours <= 0 {
		refreshHours = 12
	}
	if refreshHours > 720 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "refresh_hours must be between 1 and 720")
		return
	}

	normalizeURL := func(raw string, field string) (string, bool) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return "", true
		}
		parsed, err := url.ParseRequestURI(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, field+" must be a valid absolute URL")
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
		httpapi.WriteError(w, r, http.StatusBadRequest, "extra_status is too long (max 255 characters)")
		return
	}
	subscriptionFormat, ok := model.NormalizeSubscriptionFormat(req.SubscriptionFormat)
	if !ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription_format must be one of: links, xray-json")
		return
	}

	timeZone := strings.TrimSpace(req.TimeZone)
	if timeZone == "" {
		timeZone = "Europe/Moscow"
	}
	if len(timeZone) > 64 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "time_zone is too long (max 64 characters)")
		return
	}
	if _, err := time.LoadLocation(timeZone); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "time_zone must be a valid IANA timezone")
		return
	}

	language := strings.ToLower(strings.TrimSpace(req.Language))
	if language == "" {
		language = "ru"
	}
	switch language {
	case "ru", "en":
	default:
		httpapi.WriteError(w, r, http.StatusBadRequest, "language must be one of: ru, en")
		return
	}

	providerID := strings.TrimSpace(req.ProviderID)
	if providerID != "" && !providerIDPattern.MatchString(providerID) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "provider_id must match ^[A-Za-z0-9]{8}$")
		return
	}

	happNoLimitMode := req.HappNoLimitMode
	happNoLimitModeXHTTPOnly := req.HappNoLimitModeXHTTPOnly
	happMandatoryHWID := req.HappMandatoryHWID
	happNotifyExpiration := req.HappNotifyExpiration
	happHideServerSettings := req.HappHideServerSettings
	happSubscriptionBody := req.HappSubscriptionBody
	var showSubscriptionExpiration any
	if req.ShowSubscriptionExpiration != nil {
		showSubscriptionExpiration = boolToInt(*req.ShowSubscriptionExpiration)
	}
	if len(happSubscriptionBody) > 10000 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "happ_subscription_body is too long (max 10000 characters)")
		return
	}
	if _, err := a.db.Exec(
		`UPDATE subscription_settings
		 SET title = ?, refresh_hours = ?, info_url = ?, extra_url = ?, extra_status = ?, subscription_format = ?,
		     show_subscription_expiration = COALESCE(?, show_subscription_expiration), time_zone = ?, language = ?,
		     provider_id = ?, happ_no_limit_mode = ?, happ_no_limit_mode_xhttp_only = ?, happ_mandatory_hwid = ?,
		     happ_notify_expiration = ?, happ_hide_server_settings = ?, happ_subscription_body = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = 1`,
		title,
		refreshHours,
		nullStringValue(infoURL),
		nullStringValue(extraURL),
		nullStringValue(extraStatus),
		subscriptionFormat,
		showSubscriptionExpiration,
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
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update subscription settings")
		return
	}

	httpapi.WriteMessage(w, "subscription settings updated")
}

func (a *App) apiGetRoutingSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.getRoutingSettings()
	if err != nil {
		log.Printf("apiGetRoutingSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to load routing settings")
		return
	}
	if !validRoutingDeliveryMode(settings.DeliveryMode) {
		settings.DeliveryMode = routingDeliveryModeDisabled
	}
	httpapi.WriteJSON(w, http.StatusOK, routingSettingsWithLinks(settings))
}

func (a *App) apiUpdateRoutingSettings(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateRoutingSettingsRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	configJSON := strings.TrimSpace(req.ConfigJSON)
	current, err := a.getRoutingSettings()
	if err != nil {
		log.Printf("apiUpdateRoutingSettings load current: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update routing settings")
		return
	}
	deliveryMode := current.DeliveryMode
	if !validRoutingDeliveryMode(deliveryMode) {
		deliveryMode = routingDeliveryModeDisabled
	}
	if req.DeliveryMode != nil {
		deliveryMode = strings.TrimSpace(*req.DeliveryMode)
	}
	if !validRoutingDeliveryMode(deliveryMode) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "delivery_mode must be disabled, add, or onadd")
		return
	}
	if err := validateHappRoutingConfig(configJSON, deliveryMode != routingDeliveryModeDisabled); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	settings := model.RoutingSettings{ConfigJSON: configJSON, DeliveryMode: deliveryMode}
	if err := a.updateRoutingSettings(settings); err != nil {
		log.Printf("apiUpdateRoutingSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update routing settings")
		return
	}

	response := routingSettingsWithLinks(settings)
	response.Message = "routing settings updated"
	httpapi.WriteJSON(w, http.StatusOK, response)
}

func (a *App) apiUpdateUserHWID(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateHWIDRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MaxDevices < 0 || req.MaxDevices > 32 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "max_devices must be between 0 and 32 (0 means unlimited)")
		return
	}

	if _, err := a.db.Exec(`UPDATE users SET max_devices = ? WHERE id = ?`, req.MaxDevices, id); err != nil {
		log.Printf("apiUpdateUserHWID: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update hwid settings")
		return
	}
	a.recordAuditEvent(r, "user.hwid.update", "user", strconv.FormatInt(id, 10), map[string]any{"max_devices": req.MaxDevices})
	httpapi.WriteMessage(w, "hwid settings updated")
}

func (a *App) apiDeleteUserHWID(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	hwid := r.PathValue("hwid")
	if strings.TrimSpace(hwid) == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "hwid is required")
		return
	}

	res, err := a.db.Exec(`DELETE FROM user_devices WHERE user_id = ? AND hwid = ?`, id, hwid)
	if err != nil {
		log.Printf("apiDeleteUserHWID: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to delete hwid")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		httpapi.WriteError(w, r, http.StatusNotFound, "hwid not found")
		return
	}
	a.recordAuditEvent(r, "user.hwid.delete", "user", strconv.FormatInt(id, 10), nil)
	httpapi.WriteMessage(w, "hwid removed")
}

// --- Subscription API ---

func (a *App) apiActivateSubscription(w http.ResponseWriter, r *http.Request) {
	var req model.ActivateRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	activationCode := strings.TrimSpace(req.ActivationCode)
	if activationCode == "" || strings.Contains(activationCode, "/") {
		httpapi.WriteError(w, r, http.StatusBadRequest, "Введите корректный ключ активации")
		return
	}

	subscriptionID, code, reason, err := a.redeemActivationCode(activationCode)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to activate subscription")
		return
	}
	if code != http.StatusOK {
		if strings.TrimSpace(reason) == "" {
			reason = "Не удалось активировать подписку"
		}
		httpapi.WriteError(w, r, code, reason)
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
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"subscription_url": subscriptionURL,
		"message":          "Ключ активирован. Ссылка готова — скопируйте и вставьте её в VPN-клиент",
	})
}

// --- Subscription delivery (unchanged --- serves plaintext for VPN clients) ---

func (a *App) handleSubscription(w http.ResponseWriter, r *http.Request) {
	subscriptionID := strings.TrimSpace(r.PathValue("subscription_id"))
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}

	prepared, denial, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		log.Printf("prepare subscription delivery: %v", err)
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if denial.denied() {
		writeSubscriptionDenial(w, denial)
		a.incrementSubscriptionMetric(denial.Status, "denied")
		return
	}
	rule := prepared.Rule
	if (rule != nil && rule.ResponseType == "browser") || (rule == nil && isBrowserSubscriptionRequest(r)) {
		a.renderSubscriptionBrowserPage(w, r, subscriptionID)
		a.incrementSubscriptionMetric("browser", "success")
		return
	}

	responseType := ""
	if rule != nil {
		responseType = rule.ResponseType
	}
	generated, ok := a.writeGeneratedOrDeny(w, r, prepared, responseType)
	if !ok {
		return
	}
	settings := prepared.Settings
	body := generated.Body
	if rule != nil {
		if template, loadErr := a.loadTemplate(rule.TemplateID); loadErr == nil && template != nil && template.Enabled {
			body = applyTemplateContent(template.Content, body, settings.Title)
		}
		applyRuleHeaders(w, rule.Headers)
	}
	if err := delivery.ValidateStructuredBody(responseType, body); err != nil {
		http.Error(w, "failed to render subscription format", http.StatusUnprocessableEntity)
		a.incrementSubscriptionMetric(responseType, "render_failed")
		return
	}
	switch responseType {
	case "base64":
		body = delivery.EncodeBase64(body)
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

// writeGeneratedOrDeny renders the subscription for a prepared request and
// writes the common headers. It returns false after writing a denial or
// error response.
func (a *App) writeGeneratedOrDeny(w http.ResponseWriter, r *http.Request, prepared subscriptionDeliveryContext, responseType string) (delivery.Generated, bool) {
	generated, denial, err := a.generateSubscription(prepared, responseType)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return generated, false
	}
	delivery.ApplyExclusionHeaders(w.Header(), generated.Exclusions)
	if denial.denied() {
		if denial.Code == http.StatusUnprocessableEntity && denial.Reason == delivery.ReasonAllExcluded {
			httpapi.WriteJSON(w, denial.Code, delivery.FailurePayload(generated))
			a.incrementSubscriptionMetric(generated.OutputFormat, "all_excluded")
			return generated, false
		}
		status := remarkStatusFromReason(denial.Reason)
		writeSubscriptionDenial(w, deny(denial.Code, status, a.subscriptionRemark(denial.Reason)))
		a.incrementSubscriptionMetric(status, "denied")
		return generated, false
	}
	a.applySubscriptionResponseHeaders(w, r, prepared.Settings, prepared.SubscriptionID)
	a.applyGlobalDeliveryHeaders(w)
	applySubscriptionExpirationMetadata(w.Header(), prepared)
	return generated, true
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

func subscriptionUserinfoWithExpire(current string, expire *int64) string {
	parts := strings.Split(current, ";")
	result := make([]string, 0, len(parts)+1)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, _, _ := strings.Cut(part, "=")
		if strings.EqualFold(strings.TrimSpace(key), "expire") {
			continue
		}
		result = append(result, part)
	}
	if expire != nil {
		result = append(result, "expire="+strconv.FormatInt(*expire, 10))
	}
	return strings.Join(result, "; ")
}

func applySubscriptionExpirationMetadata(header http.Header, ctx subscriptionDeliveryContext) {
	var expire *int64
	if ctx.Settings.ShowSubscriptionExpiration && ctx.ExpiresAt.Valid {
		value := ctx.ExpiresAt.Time.UTC().Unix()
		expire = &value
	}
	userinfo := subscriptionUserinfoWithExpire(header.Get("Subscription-Userinfo"), expire)
	if userinfo == "" {
		header.Del("Subscription-Userinfo")
	} else {
		header.Set("Subscription-Userinfo", userinfo)
	}
}

func (a *App) handleSubscriptionSubBody(w http.ResponseWriter, r *http.Request) {
	a.serveSubscriptionBody(w, r, true)
}

func (a *App) handleSubscriptionSubBodyPlain(w http.ResponseWriter, r *http.Request) {
	a.serveSubscriptionBody(w, r, false)
}

// serveSubscriptionBody is the /subbody endpoint: the plain rendering, either
// base64-wrapped or raw.
func (a *App) serveSubscriptionBody(w http.ResponseWriter, r *http.Request, wrapBase64 bool) {
	subscriptionID := strings.TrimSpace(r.PathValue("subscription_id"))
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}
	prepared, denial, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		http.Error(w, "failed to load subscription", http.StatusInternalServerError)
		return
	}
	if denial.denied() {
		writeSubscriptionDenial(w, denial)
		a.incrementSubscriptionMetric(denial.Status, "denied")
		return
	}
	generated, ok := a.writeGeneratedOrDeny(w, r, prepared, "plain")
	if !ok {
		return
	}
	if wrapBase64 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(delivery.EncodeBase64(generated.Body)))
		a.incrementSubscriptionMetric("subbody-base64", "success")
		return
	}
	if prepared.Settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	_, _ = w.Write([]byte(generated.Body))
	a.incrementSubscriptionMetric("subbody-plain", "success")
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
	applySensitiveResponseHeaders(w)
	users, err := a.listUsers()
	if err != nil {
		log.Printf("apiExportUsers: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to export users")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	w.Header().Set("Content-Disposition", `attachment; filename="users.json"`)
	httpapi.WriteJSON(w, http.StatusOK, users)
}

func (a *App) apiGetSubscriptionInfo(w http.ResponseWriter, r *http.Request) {
	subscriptionID := r.PathValue("subscription_id")
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" || strings.Contains(subscriptionID, "/") {
		http.NotFound(w, r)
		return
	}
	prepared, denial, err := a.prepareSubscriptionDelivery(r, subscriptionID, true)
	if err != nil {
		http.Error(w, "failed to load subscription info", http.StatusInternalServerError)
		return
	}
	if denial.denied() {
		writeSubscriptionDenial(w, denial)
		a.incrementSubscriptionMetric(denial.Status, "denied")
		return
	}
	w.Header().Set("Subscription-Status", "active")
	if providerID := strings.TrimSpace(prepared.Settings.ProviderID); providerID != "" {
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

	httpapi.WriteJSON(w, http.StatusOK, user)
	a.incrementSubscriptionMetric("info", "success")
}
