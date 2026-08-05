package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestSafeDetailEndpointNeverExposesSecrets(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")

	// Create SS key
	ssURI := "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example:443#TestSS"
	createBody, _ := json.Marshal(map[string]any{
		"label":         "SS Key",
		"creation_mode": "raw",
		"raw_uri":       ssURI,
		"status":        "active",
		"kind":          "real",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	rec := httptest.NewRecorder()
	app.apiV1CreateKeyProfile(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}

	// GET detail
	detailReq := httptest.NewRequest(http.MethodGet, "/api/v1/keys/1", nil)
	detailReq.SetPathValue("id", "1")
	detailReq.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	detailRec := httptest.NewRecorder()
	app.apiV1GetKey(detailRec, detailReq)

	if detailRec.Code != http.StatusOK {
		t.Fatalf("get detail status=%d body=%s", detailRec.Code, detailRec.Body.String())
	}

	bodyStr := detailRec.Body.String()
	for _, secret := range []string{"MTIzNDU2Nzg5MDEyMzQ1Ng==", "ss://", "raw_uri", "encrypted_url"} {
		if strings.Contains(bodyStr, secret) {
			t.Fatalf("GET /api/v1/keys/1 exposed secret %q: %s", secret, bodyStr)
		}
	}

	// Verify headers
	if cc := detailRec.Header().Get("Cache-Control"); cc != "no-store, private" {
		t.Fatalf("Cache-Control = %q, want 'no-store, private'", cc)
	}
}

func TestRevealEndpointRequiresConcurrenyRevisionAndEmitsAuditEvent(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")

	// Create HY2 key
	hy2URI := "hysteria2://my-secret-auth@hy.example:443?sni=example.com#HY2Key"
	createBody, _ := json.Marshal(map[string]any{
		"label":         "HY2 Key",
		"creation_mode": "raw",
		"raw_uri":       hy2URI,
		"status":        "active",
		"kind":          "real",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	rec := httptest.NewRecorder()
	app.apiV1CreateKeyProfile(rec, req)

	// Reveal mismatch revision -> 409
	revealBody, _ := json.Marshal(map[string]any{
		"profile_revision": 999,
		"target":           "raw",
	})
	revealReq := httptest.NewRequest(http.MethodPost, "/api/v1/keys/1/reveal", bytes.NewReader(revealBody))
	revealReq.SetPathValue("id", "1")
	revealReq.Header.Set("Content-Type", "application/json")
	revealReq.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	revealRec := httptest.NewRecorder()
	app.apiV1RevealKey(revealRec, revealReq)

	if revealRec.Code != http.StatusConflict {
		t.Fatalf("reveal status=%d, want 409", revealRec.Code)
	}

	// Reveal correct revision -> 200
	revealBody2, _ := json.Marshal(map[string]any{
		"profile_revision": 1,
		"target":           "raw",
	})
	revealReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/keys/1/reveal", bytes.NewReader(revealBody2))
	revealReq2.SetPathValue("id", "1")
	revealReq2.Header.Set("Content-Type", "application/json")
	revealReq2.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	revealRec2 := httptest.NewRecorder()
	app.apiV1RevealKey(revealRec2, revealReq2)

	if revealRec2.Code != http.StatusOK {
		t.Fatalf("reveal status=%d, want 200", revealRec2.Code)
	}

	// Check audit event
	var auditAction, auditMeta string
	if err := app.db.QueryRow(`SELECT action, metadata_json FROM audit_events WHERE action = 'key.reveal'`).Scan(&auditAction, &auditMeta); err != nil {
		t.Fatalf("read audit event: %v", err)
	}
	if strings.Contains(auditMeta, "my-secret-auth") || strings.Contains(auditMeta, "hysteria2://") {
		t.Fatalf("audit event exposed revealed secret: %s", auditMeta)
	}
}

func TestUpdateIncrementsProfileRevisionAndRejectsStale(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")

	ssURI := "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example:443#Key1"
	createBody, _ := json.Marshal(map[string]any{
		"label":         "Key 1",
		"creation_mode": "raw",
		"raw_uri":       ssURI,
		"status":        "active",
		"kind":          "real",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	rec := httptest.NewRecorder()
	app.apiV1CreateKeyProfile(rec, req)

	// Update with rev 1 -> 200
	updateBody, _ := json.Marshal(map[string]any{
		"label":            "Updated Key 1",
		"status":           "active",
		"kind":             "real",
		"profile_revision": 1,
		"patch_mode":       "raw",
		"raw_uri":          ssURI,
	})
	upReq := httptest.NewRequest(http.MethodPut, "/api/v1/keys/1", bytes.NewReader(updateBody))
	upReq.SetPathValue("id", "1")
	upReq.Header.Set("Content-Type", "application/json")
	upReq.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	upRec := httptest.NewRecorder()
	app.apiV1UpdateKeyProfile(upRec, upReq)

	if upRec.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", upRec.Code, upRec.Body.String())
	}

	var newRev int64
	var proto string
	if err := app.db.QueryRow(`SELECT profile_revision, protocol FROM vless_keys WHERE id = 1`).Scan(&newRev, &proto); err != nil {
		t.Fatalf("read updated rev: %v", err)
	}
	if newRev != 2 || proto != "shadowsocks" {
		t.Fatalf("revision=%d, proto=%q, want rev=2, proto=shadowsocks", newRev, proto)
	}

	// Update with stale rev 1 -> 409
	upReq2 := httptest.NewRequest(http.MethodPut, "/api/v1/keys/1", bytes.NewReader(updateBody))
	upReq2.SetPathValue("id", "1")
	upReq2.Header.Set("Content-Type", "application/json")
	upReq2.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	upRec2 := httptest.NewRecorder()
	app.apiV1UpdateKeyProfile(upRec2, upReq2)

	if upRec2.Code != http.StatusConflict {
		t.Fatalf("stale update status=%d, want 409", upRec2.Code)
	}
}

func TestSourceOwnedProfileMaterialIsImmutable(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")

	sourceID := seedExternalProfileSource(t, app, "https://provider.example/immutable-test")
	ssURI := "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example:443#SourceOwned"
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, ssURI)); err != nil {
		t.Fatalf("sync source: %v", err)
	}

	// Attempt to update source-owned profile -> 403 Forbidden
	updateBody, _ := json.Marshal(map[string]any{
		"label":            "Attempted Hack",
		"status":           "active",
		"kind":             "real",
		"profile_revision": 1,
		"patch_mode":       "raw",
		"raw_uri":          ssURI,
	})
	upReq := httptest.NewRequest(http.MethodPut, "/api/v1/keys/1", bytes.NewReader(updateBody))
	upReq.SetPathValue("id", "1")
	upReq.Header.Set("Content-Type", "application/json")
	upReq.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	upRec := httptest.NewRecorder()
	app.apiV1UpdateKeyProfile(upRec, upReq)

	if upRec.Code != http.StatusForbidden {
		t.Fatalf("source-owned update status=%d, want 403", upRec.Code)
	}
}

func TestCloneKeyCreatesLocalUnlinkedRecordAtRevisionOne(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")

	sourceID := seedExternalProfileSource(t, app, "https://provider.example/clone-test")
	ssURI := "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example:443#SourceOwned"
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, ssURI)); err != nil {
		t.Fatalf("sync source: %v", err)
	}

	var syncedKeyID int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&syncedKeyID); err != nil {
		t.Fatalf("read synced key ID: %v", err)
	}

	cloneBody, _ := json.Marshal(map[string]any{
		"expected_profile_revision": 1,
		"new_label":                 "My Cloned SS",
	})
	cloneReq := httptest.NewRequest(http.MethodPost, "/api/v1/keys/"+strconv.FormatInt(syncedKeyID, 10)+"/clone", bytes.NewReader(cloneBody))
	cloneReq.SetPathValue("id", strconv.FormatInt(syncedKeyID, 10))
	cloneReq.Header.Set("Content-Type", "application/json")
	cloneReq.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	cloneRec := httptest.NewRecorder()
	app.apiV1CloneKey(cloneRec, cloneReq)

	if cloneRec.Code != http.StatusCreated {
		t.Fatalf("clone status=%d body=%s", cloneRec.Code, cloneRec.Body.String())
	}

	var clonedSourceID *int64
	var clonedRev int64
	var clonedLabel string
	if err := app.db.QueryRow(`SELECT external_source_id, profile_revision, label FROM vless_keys WHERE id != ?`, syncedKeyID).Scan(&clonedSourceID, &clonedRev, &clonedLabel); err != nil {
		t.Fatalf("read cloned key: %v", err)
	}

	if clonedSourceID != nil || clonedRev != 1 || clonedLabel != "My Cloned SS" {
		t.Fatalf("cloned key invalid: sourceID=%v rev=%d label=%q", clonedSourceID, clonedRev, clonedLabel)
	}
}

func TestCreateAndUpdateRejectMutuallyExclusiveAndInvalidModes(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")

	// Raw mode with structured payload -> 400
	badBody, _ := json.Marshal(map[string]any{
		"label":         "Bad Key",
		"creation_mode": "raw",
		"raw_uri":       "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example:443",
		"structured":    map[string]any{"server": "foo"},
		"status":        "active",
		"kind":          "real",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader(badBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	rec := httptest.NewRecorder()
	app.apiV1CreateKeyProfile(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create status=%d, want 400", rec.Code)
	}

	// Structured creation with TUIC v4 -> 400
	badTuicBody, _ := json.Marshal(map[string]any{
		"label":         "TUIC v4",
		"creation_mode": "structured",
		"protocol":      "tuic_v4",
		"structured":    map[string]any{"server": "foo"},
		"status":        "active",
		"kind":          "real",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader(badTuicBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	rec2 := httptest.NewRecorder()
	app.apiV1CreateKeyProfile(rec2, req2)

	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("tuic v4 structured create status=%d, want 400", rec2.Code)
	}
}

func TestTriStateSemanticsInStructuredUpdate(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")

	ssURI := "ss://2022-blake3-aes-128-gcm:MTIzNDU2Nzg5MDEyMzQ1Ng==@ss.example:443?plugin=v2ray-plugin%3Bopt1%3Dval1#TestSS"
	createBody, _ := json.Marshal(map[string]any{
		"label":         "SS Key",
		"creation_mode": "raw",
		"raw_uri":       ssURI,
		"status":        "active",
		"kind":          "real",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	rec := httptest.NewRecorder()
	app.apiV1CreateKeyProfile(rec, req)

	// Clear plugin name -> plugin becomes nil
	updateBody, _ := json.Marshal(map[string]any{
		"label":            "SS Key",
		"status":           "active",
		"kind":             "real",
		"profile_revision": 1,
		"patch_mode":       "structured",
		"structured_patch": map[string]any{
			"shadowsocks": map[string]any{
				"plugin_name": map[string]any{
					"operation": "clear",
				},
			},
		},
	})
	upReq := httptest.NewRequest(http.MethodPut, "/api/v1/keys/1", bytes.NewReader(updateBody))
	upReq.SetPathValue("id", "1")
	upReq.Header.Set("Content-Type", "application/json")
	upReq.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	upRec := httptest.NewRecorder()
	app.apiV1UpdateKeyProfile(upRec, upReq)

	if upRec.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", upRec.Code, upRec.Body.String())
	}

	_, decURI, err := app.fetchKeyByID(1)
	if err != nil {
		t.Fatalf("fetch key: %v", err)
	}

	if strings.Contains(decURI, "plugin=") {
		t.Fatalf("plugin was not cleared: %s", decURI)
	}
}

func TestKeyEditorSchemaEndpointReturnsValidProtocolsAndExclusionReasons(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/key-editor-schema", nil)
	req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	rec := httptest.NewRecorder()
	app.apiV1GetKeyEditorSchema(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("schema status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data model.KeyEditorSchemaResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode schema: %v", err)
	}

	if len(resp.Data.Protocols) != 3 {
		t.Fatalf("protocols count=%d, want 3", len(resp.Data.Protocols))
	}
	if len(resp.Data.ExclusionReasonCodes) == 0 {
		t.Fatalf("exclusion reason codes empty")
	}
}

func TestLegacyFullKeysEndpointCacheControl(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/keys/full", nil)
	req.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	rec := httptest.NewRecorder()
	app.apiListKeys(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/keys/full status=%d body=%s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store, private" {
		t.Fatalf("Cache-Control = %q, want 'no-store, private'", cc)
	}
}
