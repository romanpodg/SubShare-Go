package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"xary-sub/internal/model"
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

func (a *App) adminSessionFromRequest(r *http.Request) (model.AdminSession, string, bool) {
	cookie, err := r.Cookie("xray_admin_session")
	if err != nil || cookie.Value == "" {
		return model.AdminSession{}, "", false
	}

	a.mu.RLock()
	session, ok := a.sessions[cookie.Value]
	a.mu.RUnlock()
	if !ok {
		return model.AdminSession{}, "", false
	}
	if time.Now().After(session.ExpiresAt) {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
		return model.AdminSession{}, "", false
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
		now := time.Now()
		a.mu.Lock()
		for id, session := range a.sessions {
			if now.After(session.ExpiresAt) {
				delete(a.sessions, id)
			}
		}
		a.mu.Unlock()
	}
}
