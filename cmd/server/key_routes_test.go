package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKeyRouteSurfaceExcludesBulkSecretEndpoints(t *testing.T) {
	mux := http.NewServeMux()
	(&App{}).registerKeyRoutes(mux)

	for _, test := range []struct {
		method      string
		path        string
		wantPattern string
	}{
		{http.MethodGet, "/api/v1/keys", "GET /api/v1/keys"},
		{http.MethodPost, "/api/v1/keys/7/reveal", "POST /api/v1/keys/{id}/reveal"},
		{http.MethodPost, "/api/admin/keys", "POST /api/admin/keys"},
		{http.MethodPut, "/api/admin/keys/7", "PUT /api/admin/keys/{id}"},
		{http.MethodGet, "/api/v1/keys/full", "GET /api/v1/keys/{id}"},
		{http.MethodGet, "/api/admin/keys", ""},
		{http.MethodGet, "/api/admin/export/keys", ""},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			_, pattern := mux.Handler(request)
			if pattern != test.wantPattern {
				t.Fatalf("matched pattern = %q, want %q", pattern, test.wantPattern)
			}
		})
	}
}

func TestOpenAPIKeySurfaceMatchesSupportedRoutes(t *testing.T) {
	var document struct {
		Paths map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(openAPISpec, &document); err != nil {
		t.Fatalf("parse embedded OpenAPI document: %v", err)
	}
	if _, exists := document.Paths["/keys/full"]; exists {
		t.Fatal("OpenAPI still advertises removed /keys/full route")
	}
	for _, path := range []string{
		"/keys",
		"/keys/{id}",
		"/keys/{id}/reveal",
		"/keys/{id}/clone",
		"/keys/{id}/check",
		"/keys/check-all",
		"/keys/bulk/status",
		"/keys/bulk/delete",
		"/keys/order",
		"/key-categories",
		"/key-categories/order",
		"/key-categories/rename",
		"/key-categories/delete",
	} {
		if _, exists := document.Paths[path]; !exists {
			t.Errorf("OpenAPI is missing supported key route %s", path)
		}
	}
}

func TestRevealAuthorizationBoundary(t *testing.T) {
	t.Run("viewer session is forbidden", func(t *testing.T) {
		app := newIntegrationApp(t)
		sessionID, csrf, _ := seedIntegrationSession(t, app, "viewer")
		request := httptest.NewRequest(http.MethodPost, "/api/v1/keys/1/reveal", strings.NewReader(`{"target":"raw","profile_revision":1}`))
		request.SetPathValue("id", "1")
		request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
		request.Header.Set("X-CSRF-Token", csrf)
		recorder := httptest.NewRecorder()

		app.requireAdmin(http.HandlerFunc(app.apiV1RevealKey)).ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden || decodeJSONMap(t, recorder)["code"] != "viewer_read_only" {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("read token is forbidden", func(t *testing.T) {
		app := newIntegrationApp(t)
		rawToken := seedIntegrationAPIToken(t, app, []string{"read"})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/keys/1/reveal", strings.NewReader(`{"target":"raw","profile_revision":1}`))
		request.SetPathValue("id", "1")
		request.Header.Set("Authorization", "Bearer "+rawToken)
		recorder := httptest.NewRecorder()

		app.requireAdmin(http.HandlerFunc(app.apiV1RevealKey)).ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden || decodeJSONMap(t, recorder)["code"] != "token_scope_forbidden" {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("keys write token reaches typed handler", func(t *testing.T) {
		app := newIntegrationApp(t)
		rawToken := seedIntegrationAPIToken(t, app, []string{"keys:write"})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/keys/999/reveal", strings.NewReader(`{"target":"raw","profile_revision":1}`))
		request.SetPathValue("id", "999")
		request.Header.Set("Authorization", "Bearer "+rawToken)
		recorder := httptest.NewRecorder()

		app.requireAdmin(http.HandlerFunc(app.apiV1RevealKey)).ServeHTTP(recorder, request)

		if recorder.Code != http.StatusNotFound || decodeJSONMap(t, recorder)["code"] != "key_not_found" {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	})
}

func seedIntegrationAPIToken(t *testing.T, app *App, scopes []string) string {
	t.Helper()
	_, _, adminID := seedIntegrationSession(t, app, "owner")
	rawToken := "ss_" + strings.Repeat("02", 32)
	hash := sha256.Sum256([]byte(rawToken))
	scopesJSON := `[` + `"` + strings.Join(scopes, `","`) + `"` + `]`
	if _, err := app.db.Exec(
		`INSERT INTO api_tokens(name, token_prefix, token_hash, scopes_json, created_by_admin_id)
		 VALUES('route-test', 'ss_0202020202', ?, ?, ?)`,
		hex.EncodeToString(hash[:]),
		scopesJSON,
		adminID,
	); err != nil {
		t.Fatalf("insert API token: %v", err)
	}
	return rawToken
}
