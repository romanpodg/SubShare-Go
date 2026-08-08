package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/middleware"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func TestCharacterization_APIV1ErrorEnvelopeShape(t *testing.T) {
	app := newIntegrationApp(t)
	handler := app.requireAdmin(http.HandlerFunc(app.apiV1Dashboard))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.CtxKeyRequestID, "test-req-id-123"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	payload := decodeJSONMap(t, rec)
	expectedKeys := []string{"error", "code", "message", "field_errors", "request_id"}
	for _, key := range expectedKeys {
		if _, ok := payload[key]; !ok {
			t.Fatalf("API v1 error envelope missing key %q in %#v", key, payload)
		}
	}
	if payload["code"] != "unauthorized" {
		t.Fatalf("code = %v, want 'unauthorized'", payload["code"])
	}
	if payload["request_id"] != "test-req-id-123" {
		t.Fatalf("request_id = %v, want 'test-req-id-123'", payload["request_id"])
	}
}

func TestCharacterization_LegacyAPIErrorResponseShape(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "invalid parameter")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	payload := decodeJSONMap(t, rec)
	if len(payload) != 1 || payload["error"] != "invalid parameter" {
		t.Fatalf("legacy error payload = %#v, want map[error:invalid parameter]", payload)
	}
}

func TestCharacterization_RepresentativeStatusCodes(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, csrf, _ := seedIntegrationSession(t, app, "owner")

	t.Run("401 Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
		rec := httptest.NewRecorder()
		app.requireAdmin(http.HandlerFunc(app.apiV1ListKeys)).ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("401 Unauthorized reveal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/1/reveal", bytes.NewReader([]byte(`{"target":"raw","profile_revision":1}`)))
		req.SetPathValue("id", "1")
		rec := httptest.NewRecorder()
		app.requireAdmin(http.HandlerFunc(app.apiV1RevealKey)).ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader([]byte("{invalid-json")))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
		req.Header.Set("X-CSRF-Token", csrf)
		rec := httptest.NewRecorder()
		app.requireAdmin(http.HandlerFunc(app.apiV1CreateKeyProfile)).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("404 Not Found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/keys/999999", nil)
		req.SetPathValue("id", "999999")
		req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
		rec := httptest.NewRecorder()
		app.requireAdmin(http.HandlerFunc(app.apiV1GetKey)).ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("409 Conflict", func(t *testing.T) {
		if _, err := app.db.Exec(`INSERT INTO admins(username, password_hash, role) VALUES('dupadmin', 'hash', 'operator')`); err != nil {
			t.Fatalf("insert admin: %v", err)
		}
		body, _ := json.Marshal(map[string]string{"username": "dupadmin", "password": "password123!", "role": "operator"})
		req := httptest.NewRequest(http.MethodPost, "/api/admin/admins", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
		req.Header.Set("X-CSRF-Token", csrf)
		rec := httptest.NewRecorder()
		app.requireSuperAdmin(http.HandlerFunc(app.apiCreateAdmin)).ServeHTTP(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestCharacterization_CacheControlAndPragmaHeaders(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, csrf, _ := seedIntegrationSession(t, app, "owner")

	// Seed a key profile
	createBody, _ := json.Marshal(map[string]any{
		"label":         "SS Key",
		"creation_mode": "raw",
		"raw_uri":       "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example:443#TestSS",
		"status":        "active",
		"kind":          "real",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	createReq.Header.Set("X-CSRF-Token", csrf)
	createRec := httptest.NewRecorder()
	app.requireAdmin(http.HandlerFunc(app.apiV1CreateKeyProfile)).ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create key failed: status=%d body=%s", createRec.Code, createRec.Body.String())
	}
	keyID := int64(decodeJSONMap(t, createRec)["data"].(map[string]any)["id"].(float64))

	var revision int64
	t.Run("safe key detail GET /api/v1/keys/{id}", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/keys/"+strconv.FormatInt(keyID, 10), nil)
		req.SetPathValue("id", strconv.FormatInt(keyID, 10))
		req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
		rec := httptest.NewRecorder()
		app.requireAdmin(http.HandlerFunc(app.apiV1GetKey)).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store, private" {
			t.Fatalf("Cache-Control = %q, want 'no-store, private'", cc)
		}
		if pragma := rec.Header().Get("Pragma"); pragma != "no-cache" {
			t.Fatalf("Pragma = %q, want 'no-cache'", pragma)
		}
		payload := decodeJSONMap(t, rec)
		data := payload["data"].(map[string]any)
		revision = int64(data["profile_revision"].(float64))
	})

	t.Run("explicit secret reveal POST /api/v1/keys/{id}/reveal", func(t *testing.T) {
		revealBody, _ := json.Marshal(map[string]any{
			"target":           "raw",
			"profile_revision": revision,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/keys/"+strconv.FormatInt(keyID, 10)+"/reveal", bytes.NewReader(revealBody))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", strconv.FormatInt(keyID, 10))
		req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
		req.Header.Set("X-CSRF-Token", csrf)
		rec := httptest.NewRecorder()
		app.requireAdmin(http.HandlerFunc(app.apiV1RevealKey)).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store, no-cache, must-revalidate, private" {
			t.Fatalf("Cache-Control = %q, want 'no-store, no-cache, must-revalidate, private'", cc)
		}
		if pragma := rec.Header().Get("Pragma"); pragma != "no-cache" {
			t.Fatalf("Pragma = %q, want 'no-cache'", pragma)
		}
		if expires := rec.Header().Get("Expires"); expires != "0" {
			t.Fatalf("Expires = %q, want '0'", expires)
		}
		if cto := rec.Header().Get("X-Content-Type-Options"); cto != "nosniff" {
			t.Fatalf("X-Content-Type-Options = %q, want 'nosniff'", cto)
		}
	})

	t.Run("deprecated full key GET /api/v1/keys/full", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/keys/full", nil)
		req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
		rec := httptest.NewRecorder()
		app.requireAdmin(http.HandlerFunc(app.apiListKeys)).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store, private" {
			t.Fatalf("Cache-Control = %q, want 'no-store, private'", cc)
		}
		if pragma := rec.Header().Get("Pragma"); pragma != "no-cache" {
			t.Fatalf("Pragma = %q, want 'no-cache'", pragma)
		}
	})
}

func TestCharacterization_StartupInvariantFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("missing keyring on migrated database returns ErrMissingKeyring", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "v11_nokeyring.db")
		db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer db.Close()

		if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER); INSERT INTO schema_migrations VALUES (11);`); err != nil {
			t.Fatalf("seed schema: %v", err)
		}

		err = verifyStartupEnvelopesAndInvariants(ctx, db, nil)
		if err != profilestorage.ErrMissingKeyring {
			t.Fatalf("verifyStartupEnvelopesAndInvariants = %v, want ErrMissingKeyring", err)
		}
	})

	t.Run("parent without secret returns ErrMigrationVerificationFailed", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "v11_missing_secret.db")
		data, _ := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
		kr, _ := profilestorage.LoadKeyringJSON(data)
		db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer db.Close()

		schema := `
			CREATE TABLE schema_migrations (version INTEGER);
			INSERT INTO schema_migrations VALUES (11);
			CREATE TABLE vless_keys (id INTEGER PRIMARY KEY, label TEXT);
			CREATE TABLE vless_key_secrets (vless_key_id INTEGER PRIMARY KEY, encrypted_url TEXT);
			INSERT INTO vless_keys (id, label) VALUES (1, 'orphaned parent');
		`
		if _, err := db.Exec(schema); err != nil {
			t.Fatalf("seed schema: %v", err)
		}

		err = verifyStartupEnvelopesAndInvariants(ctx, db, kr)
		if err == nil || !strings.Contains(err.Error(), "parents without secrets") {
			t.Fatalf("expected parents without secrets error, got %v", err)
		}
	})
}
