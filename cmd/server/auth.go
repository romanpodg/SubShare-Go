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
			if prefersJSONResponse(r) {
				a.writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		if isUnsafeHTTPMethod(r.Method) {
			if err := r.ParseForm(); err != nil {
				if prefersJSONResponse(r) {
					a.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid form"})
					return
				}
				a.redirectAdmin(w, r, "", "invalid form")
				return
			}

			csrfToken := strings.TrimSpace(r.FormValue("csrf_token"))
			if csrfToken == "" {
				_ = r.ParseMultipartForm(8 << 20)
				csrfToken = strings.TrimSpace(r.FormValue("csrf_token"))
			}

			if !secureEqual(csrfToken, session.CSRFToken) {
				if prefersJSONResponse(r) {
					a.writeJSON(w, http.StatusForbidden, map[string]any{"error": "invalid csrf token"})
					return
				}
				a.redirectAdmin(w, r, "", "invalid csrf token")
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
