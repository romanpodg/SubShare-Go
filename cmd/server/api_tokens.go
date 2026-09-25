package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type apiTokenRecord struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

type createAPITokenRequest struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt string   `json:"expires_at"`
}

var allowedAPITokenScopes = map[string]struct{}{
	"read":           {},
	"users:write":    {},
	"keys:write":     {},
	"settings:write": {},
}

func normalizeAPITokenScopes(scopes []string) ([]string, bool) {
	if len(scopes) == 0 {
		return nil, false
	}
	out := make([]string, 0, len(scopes))
	seen := map[string]struct{}{}
	for _, scope := range scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		if _, ok := allowedAPITokenScopes[scope]; !ok {
			return nil, false
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	return out, true
}

func (a *App) apiV1ListAPITokens(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.Query(`
		SELECT id, name, token_prefix, scopes_json, expires_at, last_used_at, created_at, revoked_at
		FROM api_tokens ORDER BY id DESC
	`)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "tokens_list_failed", "failed to load API tokens")
		return
	}
	defer rows.Close()
	items := []apiTokenRecord{}
	for rows.Next() {
		var item apiTokenRecord
		var scopesJSON string
		var expiresAt, lastUsedAt, revokedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.Prefix, &scopesJSON, &expiresAt, &lastUsedAt, &item.CreatedAt, &revokedAt); err != nil {
			httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "tokens_list_failed", "failed to load API tokens")
			return
		}
		_ = json.Unmarshal([]byte(scopesJSON), &item.Scopes)
		if expiresAt.Valid {
			value := expiresAt.Time
			item.ExpiresAt = &value
		}
		if lastUsedAt.Valid {
			value := lastUsedAt.Time
			item.LastUsedAt = &value
		}
		if revokedAt.Valid {
			value := revokedAt.Time
			item.RevokedAt = &value
		}
		items = append(items, item)
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (a *App) apiV1CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	session, _, _ := a.adminSessionFromRequest(r)
	if session.AuthKind != "session" {
		httpapi.WriteV1Error(w, r, http.StatusForbidden, "interactive_session_required", "API tokens can only be created from an interactive session")
		return
	}
	var input createAPITokenRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "token_name_invalid", "name must contain 1..80 characters")
		return
	}
	scopes, ok := normalizeAPITokenScopes(input.Scopes)
	if !ok {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "token_scopes_invalid", "one or more token scopes are invalid")
		return
	}
	var expiresAt sql.NullTime
	if strings.TrimSpace(input.ExpiresAt) != "" {
		value, err := time.Parse(time.RFC3339, input.ExpiresAt)
		if err != nil || !value.After(time.Now()) {
			httpapi.WriteV1Error(w, r, http.StatusBadRequest, "token_expiry_invalid", "expires_at must be a future RFC3339 timestamp")
			return
		}
		expiresAt = sql.NullTime{Time: value, Valid: true}
	}
	random, err := generateToken(32)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "token_generate_failed", "failed to generate API token")
		return
	}
	raw := "ss_" + random
	hash := sha256.Sum256([]byte(raw))
	hashText := hex.EncodeToString(hash[:])
	prefix := raw
	if len(prefix) > 14 {
		prefix = prefix[:14]
	}
	scopesJSON, _ := json.Marshal(scopes)
	result, err := a.db.Exec(`
		INSERT INTO api_tokens(name, token_prefix, token_hash, scopes_json, expires_at, created_by_admin_id)
		VALUES(?, ?, ?, ?, ?, ?)
	`, input.Name, prefix, hashText, string(scopesJSON), nullTimeValue(expiresAt), session.AdminID)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "token_create_failed", "failed to create API token")
		return
	}
	id, _ := result.LastInsertId()
	a.recordAuditEvent(r, "api_token.create", "api_token", strconv.FormatInt(id, 10), map[string]any{"name": input.Name, "scopes": scopes})
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": id, "name": input.Name, "prefix": prefix, "scopes": scopes,
		"expires_at": input.ExpiresAt, "token": raw,
	})
}

func (a *App) apiV1RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	session, _, _ := a.adminSessionFromRequest(r)
	if session.AuthKind != "session" {
		httpapi.WriteV1Error(w, r, http.StatusForbidden, "interactive_session_required", "API tokens can only be revoked from an interactive session")
		return
	}
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	result, err := a.db.Exec(`UPDATE api_tokens SET revoked_at = CURRENT_TIMESTAMP WHERE id = ? AND revoked_at IS NULL`, id)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "token_revoke_failed", "failed to revoke API token")
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		httpapi.WriteV1Error(w, r, http.StatusNotFound, "token_not_found", "active API token not found")
		return
	}
	a.recordAuditEvent(r, "api_token.revoke", "api_token", strconv.FormatInt(id, 10), nil)
	httpapi.WriteMessage(w, "API token revoked")
}
