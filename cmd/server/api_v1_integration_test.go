package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/middleware"
)

func newIntegrationApp(t *testing.T) *App {
	t.Helper()
	db, err := initializeSQLite(filepath.Join(t.TempDir(), "integration.db"))
	if err != nil {
		t.Fatalf("initialize sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &App{db: db, subscriptionBodyEncoding: "base64", profileFingerprintKey: []byte("0123456789abcdef0123456789abcdef")}
}

func seedIntegrationSession(t *testing.T, app *App, role string) (string, string, int64) {
	t.Helper()
	result, err := app.db.Exec(`INSERT INTO admins(username, password_hash, role) VALUES(?, 'test', ?)`, "admin-"+role, role)
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	adminID, _ := result.LastInsertId()
	sessionID := "session-" + role
	csrf := "csrf-" + role
	if _, err := app.db.Exec(
		`INSERT INTO admin_sessions(id, admin_id, csrf_token, expires_at) VALUES(?, ?, ?, ?)`,
		sessionID,
		adminID,
		csrf,
		time.Now().Add(time.Hour),
	); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return sessionID, csrf, adminID
}

func decodeJSONMap(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return payload
}

func TestV1AuthErrorUsesStableEnvelope(t *testing.T) {
	app := newIntegrationApp(t)
	handler := app.requireAdmin(http.HandlerFunc(app.apiV1Dashboard))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	request = request.WithContext(context.WithValue(request.Context(), middleware.CtxKeyRequestID, "request-1"))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	payload := decodeJSONMap(t, recorder)
	if payload["code"] != "unauthorized" || payload["request_id"] != "request-1" {
		t.Fatalf("unexpected error envelope: %#v", payload)
	}
	if _, ok := payload["field_errors"]; !ok {
		t.Fatalf("field_errors missing from envelope: %#v", payload)
	}
}

func TestViewerCannotMutateV1API(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, csrf, _ := seedIntegrationSession(t, app, "viewer")
	handler := app.requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
	request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	request.Header.Set("X-CSRF-Token", csrf)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if payload := decodeJSONMap(t, recorder); payload["code"] != "viewer_read_only" {
		t.Fatalf("unexpected error: %#v", payload)
	}
}

func TestUnsafeSessionRequestRequiresValidCSRF(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, csrf, _ := seedIntegrationSession(t, app, "owner")
	handler := app.requireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPut, "/api/v1/users/1/settings", nil)
	request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	request.Header.Set("X-CSRF-Token", "wrong")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("invalid CSRF status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	request = httptest.NewRequest(http.MethodPut, "/api/v1/users/1/settings", nil)
	request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	request.Header.Set("X-CSRF-Token", csrf)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("valid CSRF status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestScopedBearerTokenCanReadDashboard(t *testing.T) {
	app := newIntegrationApp(t)
	_, _, adminID := seedIntegrationSession(t, app, "owner")
	rawToken := "ss_" + strings.Repeat("01", 32)
	hash := sha256.Sum256([]byte(rawToken))
	if _, err := app.db.Exec(
		`INSERT INTO api_tokens(name, token_prefix, token_hash, scopes_json, created_by_admin_id)
		 VALUES('integration', 'ss_0123456789', ?, '["read"]', ?)`,
		hex.EncodeToString(hash[:]),
		adminID,
	); err != nil {
		t.Fatalf("insert API token: %v", err)
	}
	handler := app.requireAdmin(http.HandlerFunc(app.apiV1Dashboard))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	request.Header.Set("Authorization", "Bearer "+rawToken)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeJSONMap(t, recorder)
	if _, ok := payload["users"]; !ok {
		t.Fatalf("dashboard response missing users: %#v", payload)
	}
}

func TestV1CompatibilityTransformsLegacyErrors(t *testing.T) {
	app := newIntegrationApp(t)
	handler := app.v1Compatibility(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusBadRequest, "legacy validation failed")
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	payload := decodeJSONMap(t, recorder)
	if recorder.Code != http.StatusBadRequest || payload["code"] != "compatibility_error" {
		t.Fatalf("unexpected compatibility response: status=%d payload=%#v", recorder.Code, payload)
	}
}

func TestV1GetUserReturnsEmptyDeviceArrays(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, "active")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	request.SetPathValue("id", strconv.FormatInt(userID, 10))
	recorder := httptest.NewRecorder()

	app.apiV1GetUser(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeJSONMap(t, recorder)
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("user data missing from response: %#v", payload)
	}
	if devices, ok := data["connected_devices"].([]any); !ok || len(devices) != 0 {
		t.Fatalf("connected_devices = %#v, want []", data["connected_devices"])
	}
	if hwids, ok := data["connected_hwids"].([]any); !ok || len(hwids) != 0 {
		t.Fatalf("connected_hwids = %#v, want []", data["connected_hwids"])
	}
}
