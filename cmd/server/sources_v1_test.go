package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sourceTestLink = "vless://11111111-1111-4111-8111-111111111111@example.com:443?security=tls&type=tcp#Test"

func sourceRequestBody(t *testing.T, sourceURL, rawBody, hwid string) []byte {
	t.Helper()
	payload := map[string]any{
		"name":                  "Provider",
		"category":              "External",
		"key_category":          "Imported",
		"key_insert_mode":       "bottom",
		"source_url":            sourceURL,
		"enabled":               true,
		"apply_remote_metadata": false,
		"pass_hwid":             hwid != "",
		"hwid_version":          "1.0",
		"hwid_model_name":       "SubShare",
		"hwid_value":            hwid,
		"raw_body":              rawBody,
		"raw_content_type":      "text/plain; charset=utf-8",
		"raw_final_url":         sourceURL,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal source request: %v", err)
	}
	return body
}

func createSourceForTest(t *testing.T, app *App, sourceURL, hwid string) int64 {
	t.Helper()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources",
		bytes.NewReader(sourceRequestBody(t, sourceURL, sourceTestLink, hwid)),
	)
	recorder := httptest.NewRecorder()
	app.apiV1CreateSource(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var sourceID int64
	if err := app.db.QueryRow(
		`SELECT id FROM external_subscription_sources WHERE source_url = ?`,
		sourceURL,
	).Scan(&sourceID); err != nil {
		t.Fatalf("load created source: %v", err)
	}
	return sourceID
}

func TestV1SourceCreateIsAtomicAndDuplicateIsStructured(t *testing.T) {
	app := newIntegrationApp(t)
	sourceURL := "https://provider.example/subscription/secret-token"
	sourceID := createSourceForTest(t, app, sourceURL, "secret-hwid")

	var sourceCount, keyCount int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources`).Scan(&sourceCount); err != nil {
		t.Fatalf("count sources: %v", err)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&keyCount); err != nil {
		t.Fatalf("count imported keys: %v", err)
	}
	if sourceCount != 1 || keyCount != 1 {
		t.Fatalf("sourceCount=%d keyCount=%d, want 1/1", sourceCount, keyCount)
	}

	duplicate := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources",
		bytes.NewReader(sourceRequestBody(t, sourceURL, sourceTestLink, "")),
	)
	duplicateRecorder := httptest.NewRecorder()
	app.apiV1CreateSource(duplicateRecorder, duplicate)
	if duplicateRecorder.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", duplicateRecorder.Code, duplicateRecorder.Body.String())
	}
	duplicatePayload := decodeJSONMap(t, duplicateRecorder)
	if duplicatePayload["code"] != "source_url_conflict" {
		t.Fatalf("unexpected duplicate envelope: %#v", duplicatePayload)
	}
	fieldErrors, ok := duplicatePayload["field_errors"].(map[string]any)
	if !ok || fieldErrors["source_url"] == nil {
		t.Fatalf("source_url field error missing: %#v", duplicatePayload)
	}

	invalidURL := "https://provider.example/subscription/invalid"
	invalid := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/sources",
		bytes.NewReader(sourceRequestBody(t, invalidURL, "not a subscription", "")),
	)
	invalidRecorder := httptest.NewRecorder()
	app.apiV1CreateSource(invalidRecorder, invalid)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}
	if err := app.db.QueryRow(
		`SELECT COUNT(*) FROM external_subscription_sources WHERE source_url = ?`,
		invalidURL,
	).Scan(&sourceCount); err != nil {
		t.Fatalf("count invalid source: %v", err)
	}
	if sourceCount != 0 {
		t.Fatalf("failed import left %d orphan sources", sourceCount)
	}
}

func TestV1SourceSummaryMasksSecretsAndHWIDUpdateIsExplicit(t *testing.T) {
	app := newIntegrationApp(t)
	sourceURL := "https://provider.example/private/secret-token?access=another-secret"
	sourceID := createSourceForTest(t, app, sourceURL, "secret-hwid")

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sources?page=1&page_size=20", nil)
	listRecorder := httptest.NewRecorder()
	app.apiV1ListSources(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := listRecorder.Body.String()
	for _, secret := range []string{"secret-token", "another-secret", "secret-hwid"} {
		if strings.Contains(listBody, secret) {
			t.Fatalf("summary leaked %q: %s", secret, listBody)
		}
	}
	if !strings.Contains(listBody, "provider.example") {
		t.Fatalf("summary does not include safe host: %s", listBody)
	}

	updatePayload := map[string]any{
		"name":                  "Provider updated",
		"category":              "External",
		"key_category":          "Imported",
		"key_insert_mode":       "bottom",
		"source_url":            sourceURL,
		"enabled":               true,
		"apply_remote_metadata": false,
		"pass_hwid":             true,
		"hwid_version":          "2.0",
		"hwid_model_name":       "Updated",
	}
	updateBody, _ := json.Marshal(updatePayload)
	updateRequest := httptest.NewRequest(http.MethodPut, "/api/v1/sources/1", bytes.NewReader(updateBody))
	updateRequest.SetPathValue("id", "1")
	updateRecorder := httptest.NewRecorder()
	app.apiV1UpdateSource(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var storedHWID string
	if err := app.db.QueryRow(
		`SELECT COALESCE(hwid_value, '') FROM external_subscription_sources WHERE id = ?`,
		sourceID,
	).Scan(&storedHWID); err != nil {
		t.Fatalf("load preserved HWID: %v", err)
	}
	if storedHWID != "secret-hwid" {
		t.Fatalf("omitted hwid_value changed secret to %q", storedHWID)
	}

	updatePayload["clear_hwid_value"] = true
	updateBody, _ = json.Marshal(updatePayload)
	clearRequest := httptest.NewRequest(http.MethodPut, "/api/v1/sources/1", bytes.NewReader(updateBody))
	clearRequest.SetPathValue("id", "1")
	clearRecorder := httptest.NewRecorder()
	app.apiV1UpdateSource(clearRecorder, clearRequest)
	if clearRecorder.Code != http.StatusOK {
		t.Fatalf("clear status = %d, body = %s", clearRecorder.Code, clearRecorder.Body.String())
	}
	if err := app.db.QueryRow(
		`SELECT COALESCE(hwid_value, '') FROM external_subscription_sources WHERE id = ?`,
		sourceID,
	).Scan(&storedHWID); err != nil {
		t.Fatalf("load cleared HWID: %v", err)
	}
	if storedHWID != "" {
		t.Fatalf("clear_hwid_value left %q", storedHWID)
	}
}

func TestV1DashboardAggregatesEffectiveStatuses(t *testing.T) {
	app := newIntegrationApp(t)
	users := []struct {
		name, token, status, expires string
		maxDevices                   int
	}{
		{"Active", "token-active", "active", "", 2},
		{"Paused", "token-paused", "paused", "", 1},
		{"Blocked", "token-blocked", "blocked", "", 1},
		{"Expired", "token-expired", "active", "2000-01-01 00:00:00", 1},
		{"Limited", "token-limited", "active", "", 1},
	}
	var limitedID int64
	for _, user := range users {
		result, err := app.db.Exec(
			`INSERT INTO users(name, token, status, expires_at, max_devices) VALUES(?, ?, ?, NULLIF(?, ''), ?)`,
			user.name,
			user.token,
			user.status,
			user.expires,
			user.maxDevices,
		)
		if err != nil {
			t.Fatalf("insert user %s: %v", user.name, err)
		}
		if user.name == "Limited" {
			limitedID, _ = result.LastInsertId()
		}
	}
	if _, err := app.db.Exec(
		`INSERT INTO user_devices(user_id, hwid) VALUES(?, 'device-hash')`,
		limitedID,
	); err != nil {
		t.Fatalf("insert device: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	recorder := httptest.NewRecorder()
	app.apiV1Dashboard(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeJSONMap(t, recorder)
	counts := payload["users"].(map[string]any)
	for status, want := range map[string]float64{
		"total": 5, "active": 1, "paused": 1, "blocked": 1, "expired": 1, "limited": 1,
	} {
		if counts[status] != want {
			t.Fatalf("%s = %#v, want %.0f; payload=%#v", status, counts[status], want, counts)
		}
	}
}

func TestV1DashboardReportsDegradedSections(t *testing.T) {
	app := newIntegrationApp(t)
	if _, err := app.db.Exec(`DROP TABLE audit_events`); err != nil {
		t.Fatalf("drop audit table: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	recorder := httptest.NewRecorder()
	app.apiV1Dashboard(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeJSONMap(t, recorder)
	sections, ok := payload["degraded_sections"].([]any)
	if !ok || len(sections) != 1 || sections[0] != "audit" {
		t.Fatalf("unexpected degraded sections: %#v", payload["degraded_sections"])
	}
}

func TestV1SourceRBACAllowsSafeListButProtectsDetail(t *testing.T) {
	app := newIntegrationApp(t)
	createSourceForTest(t, app, "https://provider.example/rbac", "hidden-hwid")
	sessionID, _, _ := seedIntegrationSession(t, app, "viewer")

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	listRequest.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	listRecorder := httptest.NewRecorder()
	app.requireAdmin(http.HandlerFunc(app.apiV1ListSources)).ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("viewer list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sources/1", nil)
	detailRequest.SetPathValue("id", "1")
	detailRequest.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	detailRecorder := httptest.NewRecorder()
	app.requireSuperAdmin(http.HandlerFunc(app.apiV1GetSource)).ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusForbidden {
		t.Fatalf("viewer detail status = %d, body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	if payload := decodeJSONMap(t, detailRecorder); payload["code"] != "owner_required" {
		t.Fatalf("unexpected detail denial: %#v", payload)
	}
}

func TestV1SourceDeleteReportsLinkedKeys(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := createSourceForTest(t, app, "https://provider.example/delete", "")
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/sources/1", nil)
	request.SetPathValue("id", "1")
	recorder := httptest.NewRecorder()
	app.apiV1DeleteSource(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeJSONMap(t, recorder)
	if payload["deleted_keys"] != float64(1) {
		t.Fatalf("deleted_keys = %#v, want 1", payload["deleted_keys"])
	}
	var sources, keys int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources WHERE id = ?`, sourceID).Scan(&sources); err != nil {
		t.Fatalf("count deleted source: %v", err)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&keys); err != nil {
		t.Fatalf("count deleted keys: %v", err)
	}
	if sources != 0 || keys != 0 {
		t.Fatalf("delete left sources=%d keys=%d", sources, keys)
	}
}

func TestV1JobPollingReturnsTerminalState(t *testing.T) {
	app := newIntegrationApp(t)
	result, err := app.db.Exec(`
		INSERT INTO background_jobs(kind, status, target_type, target_id, finished_at)
		VALUES('source_sync', 'succeeded', 'external_source', '1', CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	jobID, _ := result.LastInsertId()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/1", nil)
	request.SetPathValue("id", "1")
	recorder := httptest.NewRecorder()
	app.apiV1GetJob(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("job status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	payload := decodeJSONMap(t, recorder)
	data, ok := payload["data"].(map[string]any)
	if !ok || data["id"] != float64(jobID) || data["status"] != "succeeded" {
		t.Fatalf("unexpected job payload: %#v", payload)
	}
}
