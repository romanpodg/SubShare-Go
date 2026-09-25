package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/middleware"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

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
