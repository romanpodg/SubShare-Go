package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func TestKeyAdministrationHandler_ContractsAndSecretSafety(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForHTTPAPI(t)
	service := keymanagement.NewService(storage.NewProfileRepository(db, kr), storage.NewKeyRepository(db, kr), nil)

	createReal := func(label, rawURL string) int64 {
		t.Helper()
		id, _, err := service.CreateLegacy(t.Context(), keymanagement.CreateLegacyParams{
			Label: label, URL: rawURL, Kind: "real", Status: "active",
		})
		if err != nil {
			t.Fatalf("create %s: %v", label, err)
		}
		return id
	}
	firstURL := "vless://11111111-1111-1111-1111-111111111111@example.com:443#first"
	secondURL := "vless://22222222-2222-2222-2222-222222222222@example.com:443#second"
	firstID := createReal("First", firstURL)
	secondID := createReal("Second", secondURL)
	_, _, err := service.CreateLegacy(t.Context(), keymanagement.CreateLegacyParams{
		Label: "Information", Kind: "informational", Status: "active", TemplateText: "notice",
	})
	if err != nil {
		t.Fatalf("create informational key: %v", err)
	}
	missingID := createReal("Missing", "vless://33333333-3333-3333-3333-333333333333@example.com:443#missing")
	corruptID := createReal("Corrupt", "vless://44444444-4444-4444-4444-444444444444@example.com:443#corrupt")
	if _, err := db.Exec(`DELETE FROM vless_key_secrets WHERE vless_key_id = ?`, missingID); err != nil {
		t.Fatalf("remove secret: %v", err)
	}
	if _, err := db.Exec(`UPDATE vless_key_secrets SET encrypted_url = 'corrupt' WHERE vless_key_id = ?`, corruptID); err != nil {
		t.Fatalf("corrupt secret: %v", err)
	}

	var auditLog []auditRecord
	audit := func(r *http.Request, eventName, entityType, entityID string, metadata map[string]any) {
		auditLog = append(auditLog, auditRecord{eventName: eventName, entityType: entityType, entityID: entityID, metadata: metadata})
	}
	var checkedMu sync.Mutex
	checked := make([]int64, 0)
	checkKey := func(ctx context.Context, id int64, rawURL string) error {
		checkedMu.Lock()
		checked = append(checked, id)
		checkedMu.Unlock()
		if rawURL == "" {
			t.Errorf("checker received an empty credential for key %d", id)
		}
		return service.SaveHealthCheckResult(ctx, id, "up", "", 12)
	}
	startedJobs := 0
	finishedJobs := 0
	handler := NewKeyAdministrationHandler(service, audit, KeyAdministrationRuntime{
		CheckKey: checkKey,
		StartJob: func(kind, targetType, targetID string) int64 {
			startedJobs++
			if kind != "keys_health_check" || targetType != "key" || targetID != "all" {
				t.Errorf("unexpected job identity: %s/%s/%s", kind, targetType, targetID)
			}
			return 91
		},
		FinishJob: func(jobID int64, err error) {
			finishedJobs++
			if jobID != 91 || err != nil {
				t.Errorf("unexpected job finish: id=%d err=%v", jobID, err)
			}
		},
		QueueHealthJob: func(r *http.Request) int64 { return 123 },
	})

	recorder := httptest.NewRecorder()
	handler.BulkUpdateKeys(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/keys/bulk/status", bytes.NewBufferString(`{"ids":`)))
	if recorder.Code != http.StatusBadRequest || len(auditLog) != 0 {
		t.Fatalf("malformed bulk update: status=%d audit=%#v body=%s", recorder.Code, auditLog, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.BulkUpdateKeys(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/keys/bulk/status", bytes.NewBufferString(`{"ids":[],"status":"active"}`)))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "ids list is empty") {
		t.Fatalf("empty bulk update: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	bulkBody := bytes.NewBufferString(`{"ids":[` + jsonInt(firstID) + `,` + jsonInt(secondID) + `],"status":"non-active","category":"Bulk"}`)
	handler.BulkUpdateKeys(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/keys/bulk/status", bulkBody))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"updated":2`) {
		t.Fatalf("bulk update: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(auditLog) != 1 || auditLog[0].eventName != "keys.bulk_status" || auditLog[0].metadata["status"] != "non-active" {
		t.Fatalf("bulk update audit = %#v", auditLog)
	}

	recorder = httptest.NewRecorder()
	missingBulkBody := bytes.NewBufferString(`{"ids":[` + jsonInt(firstID) + `,99999],"status":"active"}`)
	handler.BulkUpdateKeys(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/keys/bulk/status", missingBulkBody))
	if recorder.Code != http.StatusNotFound || len(auditLog) != 1 {
		t.Fatalf("missing bulk update: status=%d audit=%#v body=%s", recorder.Code, auditLog, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/admin/keys/1/check", nil)
	request.SetPathValue("id", jsonInt(firstID))
	handler.CheckKey(recorder, request)
	if recorder.Code != http.StatusOK ||
		!strings.Contains(recorder.Body.String(), `"check_status":"up"`) ||
		!strings.Contains(recorder.Body.String(), `"last_latency_ms":12`) ||
		!strings.Contains(recorder.Body.String(), `"last_checked_at":"`) {
		t.Fatalf("single check: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), firstURL) {
		t.Fatal("single-check response exposed the decrypted credential URL")
	}
	var firstStatus string
	if err := db.QueryRow(`SELECT status FROM vless_keys WHERE id = ?`, firstID).Scan(&firstStatus); err != nil {
		t.Fatalf("read manually checked key status: %v", err)
	}
	if firstStatus != "non-active" {
		t.Fatalf("manual health check changed inactive key status to %q", firstStatus)
	}

	assertHealthFailure := func(id int64, wantMessage string) {
		t.Helper()
		beforeChecks := len(checked)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/admin/keys/check", nil)
		req.SetPathValue("id", jsonInt(id))
		handler.CheckKey(rec, req)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), wantMessage) {
			t.Fatalf("health failure for %d: status=%d body=%s", id, rec.Code, rec.Body.String())
		}
		if len(checked) != beforeChecks || strings.Contains(rec.Body.String(), "vless://") {
			t.Fatalf("failed health request reached checker or leaked a secret: checks=%v body=%s", checked, rec.Body.String())
		}
	}
	assertHealthFailure(missingID, "missing profile key secret")
	assertHealthFailure(corruptID, "failed to decrypt key")

	checkedMu.Lock()
	checked = checked[:0]
	checkedMu.Unlock()
	recorder = httptest.NewRecorder()
	handler.CheckAllKeys(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/keys/check-all", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"checked":0`) {
		t.Fatalf("check all: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if startedJobs != 1 || finishedJobs != 1 || len(auditLog) != 2 || auditLog[1].eventName != "keys.health_check" {
		t.Fatalf("check-all callbacks/audit: started=%d finished=%d audit=%#v", startedJobs, finishedJobs, auditLog)
	}
	if strings.Contains(recorder.Body.String(), firstURL) || strings.Contains(recorder.Body.String(), secondURL) {
		t.Fatal("check-all response exposed decrypted credential URLs")
	}

	recorder = httptest.NewRecorder()
	deleteBody := bytes.NewBufferString(`{"ids":[` + jsonInt(secondID) + `]}`)
	handler.BulkDeleteKeys(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/keys/bulk/delete", deleteBody))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"deleted":1`) {
		t.Fatalf("bulk delete: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(auditLog) != 3 || auditLog[2].eventName != "keys.bulk_delete" {
		t.Fatalf("bulk delete audit = %#v", auditLog)
	}

	recorder = httptest.NewRecorder()
	handler.QueueHealthCheck(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/keys/check-all", nil))
	if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), `"job_id":123`) {
		t.Fatalf("queue health check: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestKeyAdministrationHandler_QueueFailureUsesV1ErrorContract(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForHTTPAPI(t)
	service := keymanagement.NewService(storage.NewProfileRepository(db, kr), storage.NewKeyRepository(db, kr), nil)
	handler := NewKeyAdministrationHandler(service, nil, KeyAdministrationRuntime{})
	recorder := httptest.NewRecorder()
	handler.QueueHealthCheck(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/keys/check-all", nil))
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"code":"job_queue_failed"`) {
		t.Fatalf("queue failure: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func jsonInt(value int64) string {
	data, _ := json.Marshal(value)
	return string(data)
}
