package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func TestLegacyKeyHandler_FullSuite(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForHTTPAPI(t)
	repo := storage.NewProfileRepository(db, kr)
	svc := keymanagement.NewService(repo, nil)

	var auditLog []auditRecord
	recordAudit := func(r *http.Request, eventName, entityType, entityID string, metadata map[string]any) {
		auditLog = append(auditLog, auditRecord{
			eventName:  eventName,
			entityType: entityType,
			entityID:   entityID,
			metadata:   metadata,
		})
	}

	handler := NewLegacyKeyHandler(svc, recordAudit)

	// 1. List Keys (Empty) -> 200
	reqListEmpty := httptest.NewRequest("GET", "/api/admin/keys", nil)
	recListEmpty := httptest.NewRecorder()
	handler.ListKeys(recListEmpty, reqListEmpty)
	if recListEmpty.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recListEmpty.Code)
	}
	if cache := recListEmpty.Header().Get("Cache-Control"); cache != "no-store, private" {
		t.Fatalf("unexpected Cache-Control: %q", cache)
	}

	// 2. Create Key -> 200 {"message": "key added"}
	vlessURI := "vless://00000000-0000-0000-0000-000000000000@example.com:443?type=tcp#TestKey"
	bodyCreate := bytes.NewBufferString(`{"label":"My Key","url":"` + vlessURI + `","kind":"real","status":"active"}`)
	reqCreate := httptest.NewRequest("POST", "/api/admin/keys", bodyCreate)
	recCreate := httptest.NewRecorder()
	handler.CreateKey(recCreate, reqCreate)
	if recCreate.Code != http.StatusOK {
		t.Fatalf("CreateKey expected 200, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	if len(auditLog) != 1 || auditLog[0].eventName != "key.create" || auditLog[0].entityID != "1" {
		t.Fatalf("unexpected audit log: %#v", auditLog)
	}

	// 3. Create Key Invalid Kind -> 400
	bodyBadKind := bytes.NewBufferString(`{"label":"Bad Kind","url":"` + vlessURI + `","kind":"invalid_kind","status":"active"}`)
	reqBadKind := httptest.NewRequest("POST", "/api/admin/keys", bodyBadKind)
	recBadKind := httptest.NewRecorder()
	handler.CreateKey(recBadKind, reqBadKind)
	if recBadKind.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad kind, got %d", recBadKind.Code)
	}

	// 4. Update Key -> 200 {"message": "key updated"}
	bodyUpdate := bytes.NewBufferString(`{"label":"Updated Key","raw_url":"` + vlessURI + `","kind":"real","status":"active"}`)
	reqUpdate := httptest.NewRequest("PUT", "/api/admin/keys/1", bodyUpdate)
	reqUpdate.SetPathValue("id", "1")
	recUpdate := httptest.NewRecorder()
	handler.UpdateKey(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("UpdateKey expected 200, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}

	// 5. Update Key Not Found -> 404
	bodyUpdate404 := bytes.NewBufferString(`{"label":"Updated Key","raw_url":"` + vlessURI + `","kind":"real","status":"active"}`)
	reqUpdate404 := httptest.NewRequest("PUT", "/api/admin/keys/999", bodyUpdate404)
	reqUpdate404.SetPathValue("id", "999")
	recUpdate404 := httptest.NewRecorder()
	handler.UpdateKey(recUpdate404, reqUpdate404)
	if recUpdate404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for update non-existent, got %d: %s", recUpdate404.Code, recUpdate404.Body.String())
	}

	// 6. Delete Key -> 200 {"message": "key deleted"}
	reqDelete := httptest.NewRequest("DELETE", "/api/admin/keys/1", nil)
	reqDelete.SetPathValue("id", "1")
	recDelete := httptest.NewRecorder()
	handler.DeleteKey(recDelete, reqDelete)
	if recDelete.Code != http.StatusOK {
		t.Fatalf("DeleteKey expected 200, got %d: %s", recDelete.Code, recDelete.Body.String())
	}

	// 7. Delete Key Not Found -> 404
	reqDelete404 := httptest.NewRequest("DELETE", "/api/admin/keys/1", nil)
	reqDelete404.SetPathValue("id", "1")
	recDelete404 := httptest.NewRecorder()
	handler.DeleteKey(recDelete404, reqDelete404)
	if recDelete404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for delete non-existent, got %d", recDelete404.Code)
	}
}

func TestLegacyKeyHandler_SecurityAndListProjection(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForHTTPAPI(t)
	repo := storage.NewProfileRepository(db, kr)
	svc := keymanagement.NewService(repo, nil)

	handler := NewLegacyKeyHandler(svc, nil)
	vlessURI := "vless://11111111-1111-1111-1111-111111111111@example.com:443?type=tcp#SecKey"

	// Create a key
	bodyCreate := bytes.NewBufferString(`{"label":"SecKey","url":"` + vlessURI + `","kind":"real","status":"active"}`)
	reqCreate := httptest.NewRequest("POST", "/api/admin/keys", bodyCreate)
	recCreate := httptest.NewRecorder()
	handler.CreateKey(recCreate, reqCreate)
	if recCreate.Code != http.StatusOK {
		t.Fatalf("CreateKey failed: %d", recCreate.Code)
	}

	// Fetch GET /api/admin/keys
	reqList := httptest.NewRequest("GET", "/api/admin/keys", nil)
	recList := httptest.NewRecorder()
	handler.ListKeys(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("ListKeys failed: %d", recList.Code)
	}

	rawJSON := recList.Body.String()
	// Security assertion: Internal values MUST NOT appear anywhere in response JSON
	prohibitedSubstrings := []string{
		"encrypted_url",
		"url_blind_index",
		"key-1",
		"bik-1",
	}
	for _, forbidden := range prohibitedSubstrings {
		if strings.Contains(rawJSON, forbidden) {
			t.Fatalf("security violation: internal value %q found in ListKeys response: %s", forbidden, rawJSON)
		}
	}
}
