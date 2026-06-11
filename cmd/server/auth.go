package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"subshare/internal/model"
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

func (a *App) requireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _, ok := a.adminSessionFromRequest(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		if session.Role != "super_admin" {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
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

func (a *App) adminSessionFromRequest(r *http.Request) (model.AdminSession, string, bool) {
	cookie, err := r.Cookie("subshare_admin_session")
	if err != nil || cookie.Value == "" {
		return model.AdminSession{}, "", false
	}

	var adminID int64
	var csrfToken string
	var expiresAt time.Time
	err = a.db.QueryRow(`SELECT admin_id, csrf_token, expires_at FROM admin_sessions WHERE id = ?`, cookie.Value).Scan(&adminID, &csrfToken, &expiresAt)
	if err != nil {
		return model.AdminSession{}, "", false
	}

	if time.Now().After(expiresAt) {
		_, _ = a.db.Exec(`DELETE FROM admin_sessions WHERE id = ?`, cookie.Value)
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
