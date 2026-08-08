package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func TestLegacyKeyHandler_FullSuite(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForHTTPAPI(t)
	profileRepo := storage.NewProfileRepository(db, kr)
	keyRepo := storage.NewKeyRepository(db, kr)
	svc := keymanagement.NewService(profileRepo, keyRepo, nil)

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

	// Create Key -> 200 {"message": "key added"}
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

	// Create Key Invalid Kind -> 400
	bodyBadKind := bytes.NewBufferString(`{"label":"Bad Kind","url":"` + vlessURI + `","kind":"invalid_kind","status":"active"}`)
	reqBadKind := httptest.NewRequest("POST", "/api/admin/keys", bodyBadKind)
	recBadKind := httptest.NewRecorder()
	handler.CreateKey(recBadKind, reqBadKind)
	if recBadKind.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad kind, got %d", recBadKind.Code)
	}

	// Update Key -> 200 {"message": "key updated"}
	bodyUpdate := bytes.NewBufferString(`{"label":"Updated Key","raw_url":"` + vlessURI + `","kind":"real","status":"active"}`)
	reqUpdate := httptest.NewRequest("PUT", "/api/admin/keys/1", bodyUpdate)
	reqUpdate.SetPathValue("id", "1")
	recUpdate := httptest.NewRecorder()
	handler.UpdateKey(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("UpdateKey expected 200, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}
	if len(auditLog) != 2 || auditLog[1].eventName != "key.update" || auditLog[1].entityID != "1" {
		t.Fatalf("unexpected update audit log: %#v", auditLog)
	}

	// Update Key Not Found -> 404
	bodyUpdate404 := bytes.NewBufferString(`{"label":"Updated Key","raw_url":"` + vlessURI + `","kind":"real","status":"active"}`)
	reqUpdate404 := httptest.NewRequest("PUT", "/api/admin/keys/999", bodyUpdate404)
	reqUpdate404.SetPathValue("id", "999")
	recUpdate404 := httptest.NewRecorder()
	handler.UpdateKey(recUpdate404, reqUpdate404)
	if recUpdate404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for update non-existent, got %d: %s", recUpdate404.Code, recUpdate404.Body.String())
	}

	// Delete Key -> 200 {"message": "key deleted"}
	reqDelete := httptest.NewRequest("DELETE", "/api/admin/keys/1", nil)
	reqDelete.SetPathValue("id", "1")
	recDelete := httptest.NewRecorder()
	handler.DeleteKey(recDelete, reqDelete)
	if recDelete.Code != http.StatusOK {
		t.Fatalf("DeleteKey expected 200, got %d: %s", recDelete.Code, recDelete.Body.String())
	}
	if len(auditLog) != 3 || auditLog[2].eventName != "key.delete" || auditLog[2].entityID != "1" {
		t.Fatalf("unexpected delete audit log: %#v", auditLog)
	}

	// Delete Key Not Found -> 404
	reqDelete404 := httptest.NewRequest("DELETE", "/api/admin/keys/1", nil)
	reqDelete404.SetPathValue("id", "1")
	recDelete404 := httptest.NewRecorder()
	handler.DeleteKey(recDelete404, reqDelete404)
	if recDelete404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for delete non-existent, got %d", recDelete404.Code)
	}
}
