package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _, ok := a.adminSessionFromRequest(r)
		if !ok {
			writeAdminAuthError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if isUnsafeHTTPMethod(r.Method) && session.Role == "viewer" {
			writeAdminAuthError(w, r, http.StatusForbidden, "viewer_read_only", "viewer role is read-only")
			return
		}
		if session.AuthKind == "api_token" && !apiTokenAllows(session.Scopes, r.Method, r.URL.Path) {
			writeAdminAuthError(w, r, http.StatusForbidden, "token_scope_forbidden", "API token scope does not allow this action")
			return
		}
		if isUnsafeHTTPMethod(r.Method) && session.AuthKind != "api_token" {
			csrfToken := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
			if !secureEqual(csrfToken, session.CSRFToken) {
				writeAdminAuthError(w, r, http.StatusForbidden, "csrf_invalid", "invalid CSRF token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) requireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _, ok := a.adminSessionFromRequest(r)
		if !ok {
			writeAdminAuthError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if !isOwnerRole(session.Role) {
			writeAdminAuthError(w, r, http.StatusForbidden, "owner_required", "owner role is required")
			return
		}
		if session.AuthKind == "api_token" && !apiTokenAllows(session.Scopes, r.Method, r.URL.Path) {
			writeAdminAuthError(w, r, http.StatusForbidden, "token_scope_forbidden", "API token scope does not allow this action")
			return
		}
		if isUnsafeHTTPMethod(r.Method) && session.AuthKind != "api_token" {
			csrfToken := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
			if !secureEqual(csrfToken, session.CSRFToken) {
				writeAdminAuthError(w, r, http.StatusForbidden, "csrf_invalid", "invalid CSRF token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeAdminAuthError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		writeV1Error(w, r, status, code, message)
		return
	}
	writeJSON(w, status, map[string]any{"error": message})
}

func isOwnerRole(role string) bool {
	return role == "owner" || role == "super_admin"
}

func normalizeAdminRole(role string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "super_admin":
		return "owner", true
	case "operator", "support_admin":
		return "operator", true
	case "viewer":
		return "viewer", true
	default:
		return "", false
	}
}

func (a *App) adminSessionFromRequest(r *http.Request) (model.AdminSession, string, bool) {
	cookie, err := r.Cookie(model.AdminSessionCookieName)
	if err != nil || cookie.Value == "" {
		return a.apiTokenSessionFromRequest(r)
	}

	storedID := hashSessionID(cookie.Value)
	var adminID int64
	var csrfToken string
	var expiresAt time.Time
	err = a.db.QueryRow(`SELECT admin_id, csrf_token, expires_at FROM admin_sessions WHERE id = ?`, storedID).Scan(&adminID, &csrfToken, &expiresAt)
	if err != nil {
		return model.AdminSession{}, "", false
	}

	if time.Now().After(expiresAt) {
		_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE id = ?`, storedID)
		return model.AdminSession{}, "", false
	}

	var role string
	err = a.db.QueryRow(`SELECT role FROM admins WHERE id = ?`, adminID).Scan(&role)
	if err != nil {
		return model.AdminSession{}, "", false
	}

	session := model.AdminSession{
		AdminID:   adminID,
		Role:      role,
		CSRFToken: csrfToken,
		ExpiresAt: expiresAt,
		AuthKind:  "session",
	}

	return session, cookie.Value, true
}

func (a *App) apiTokenSessionFromRequest(r *http.Request) (model.AdminSession, string, bool) {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return model.AdminSession{}, "", false
	}
	raw := strings.TrimSpace(authorization[len("Bearer "):])
	if !strings.HasPrefix(raw, "ss_") || len(raw) < 32 {
		return model.AdminSession{}, "", false
	}
	hash := sha256.Sum256([]byte(raw))
	hashText := hex.EncodeToString(hash[:])

	var tokenID int64
	var adminID int64
	var role string
	var scopesJSON string
	var expiresAt sql.NullTime
	err := a.db.QueryRow(`
		SELECT t.id, t.created_by_admin_id, a.role, t.scopes_json, t.expires_at
		FROM api_tokens t
		JOIN admins a ON a.id = t.created_by_admin_id
		WHERE t.token_hash = ? AND t.revoked_at IS NULL
	`, hashText).Scan(&tokenID, &adminID, &role, &scopesJSON, &expiresAt)
	if err != nil || (expiresAt.Valid && time.Now().After(expiresAt.Time)) {
		return model.AdminSession{}, "", false
	}
	var scopes []string
	if json.Unmarshal([]byte(scopesJSON), &scopes) != nil {
		return model.AdminSession{}, "", false
	}
	_, _ = a.db.Exec(`UPDATE api_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, tokenID)
	return model.AdminSession{
		AdminID:  adminID,
		Role:     role,
		AuthKind: "api_token",
		Scopes:   scopes,
	}, strconv.FormatInt(tokenID, 10), true
}

func apiTokenAllows(scopes []string, method, path string) bool {
	has := func(wanted string) bool {
		for _, scope := range scopes {
			if scope == "*" || scope == wanted {
				return true
			}
		}
		return false
	}
	if method == http.MethodGet || method == http.MethodHead {
		return has("read")
	}
	switch {
	case strings.Contains(path, "/admins"), strings.Contains(path, "/api-tokens"):
		// Creating administrators and minting tokens is ownership itself, so it
		// stays reachable only from an explicitly unrestricted token.
		return has("*")
	case strings.Contains(path, "/users"):
		return has("users:write")
	case strings.Contains(path, "/keys"),
		strings.Contains(path, "/key-categories"),
		strings.Contains(path, "/sources"),
		strings.Contains(path, "/external-sources"),
		strings.Contains(path, "/source-categories"):
		return has("keys:write")
	case strings.Contains(path, "/templates"),
		strings.Contains(path, "/response-rules"),
		strings.Contains(path, "/routing-settings"),
		strings.Contains(path, "/panel-settings"),
		strings.Contains(path, "/subscription-settings"),
		strings.Contains(path, "/subscription-delivery-settings"),
		strings.Contains(path, "/subscription-page-config"),
		strings.Contains(path, "/jobs"):
		return has("settings:write")
	default:
		// Deny by default. An unrecognized write endpoint is far more likely to
		// be newly added and privileged than to be safe, and the permissive
		// default this replaces handed /admins and /api-tokens — account
		// creation and token minting — to any token holding settings:write.
		return false
	}
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

// hashSessionID converts a session cookie into the value stored in
// admin_sessions.id. Storing the raw cookie made every backup — runBackup
// copies the whole database with VACUUM INTO — a file full of directly usable
// admin credentials. API tokens were already hashed this way; sessions now
// match. Changing this invalidates sessions issued before the upgrade, so
// administrators sign in again once after deployment.
func hashSessionID(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
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

func (a *App) cleanupExpiredSessions(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		_, err := a.db.Exec(`DELETE FROM admin_sessions WHERE expires_at < ?`, time.Now())
		if err != nil {
			log.Printf("cleanupExpiredSessions: %v", err)
		}
	}
}
