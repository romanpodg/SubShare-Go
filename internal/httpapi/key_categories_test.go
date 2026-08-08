package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func TestKeyCategoryHandler_FullSuite(t *testing.T) {
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

	handler := NewKeyCategoryHandler(svc, recordAudit)

	// 1. List Categories (Empty)
	reqList := httptest.NewRequest("GET", "/api/admin/key-categories", nil)
	recList := httptest.NewRecorder()
	handler.ListCategories(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recList.Code)
	}

	// 2. Create Category
	bodyCreate := bytes.NewBufferString(`{"name":"Cat1","color":"#ff0000"}`)
	reqCreate := httptest.NewRequest("POST", "/api/admin/key-categories", bodyCreate)
	recCreate := httptest.NewRecorder()
	handler.CreateCategory(recCreate, reqCreate)
	if recCreate.Code != http.StatusOK {
		t.Fatalf("CreateCategory expected 200, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var createResponse struct {
		Category struct {
			ID    int64  `json:"id"`
			Color string `json:"color"`
		} `json:"category"`
	}
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResponse.Category.ID != 0 || createResponse.Category.Color != "#FF0000" {
		t.Fatalf("create response compatibility changed: %#v", createResponse.Category)
	}

	// 2b. Create Duplicate Category (Upserts color, returns 200)
	bodyCreateDup := bytes.NewBufferString(`{"name":"Cat1","color":"#0000ff"}`)
	reqCreateDup := httptest.NewRequest("POST", "/api/admin/key-categories", bodyCreateDup)
	recCreateDup := httptest.NewRecorder()
	handler.CreateCategory(recCreateDup, reqCreateDup)
	if recCreateDup.Code != http.StatusOK {
		t.Fatalf("CreateCategory duplicate expected 200, got %d: %s", recCreateDup.Code, recCreateDup.Body.String())
	}

	// 2c. Invalid color keeps the legacy lowercase default and zero response ID.
	bodyDefaultColor := bytes.NewBufferString(`{"name":"DefaultColor","color":"invalid"}`)
	reqDefaultColor := httptest.NewRequest("POST", "/api/admin/key-categories", bodyDefaultColor)
	recDefaultColor := httptest.NewRecorder()
	handler.CreateCategory(recDefaultColor, reqDefaultColor)
	if recDefaultColor.Code != http.StatusOK {
		t.Fatalf("CreateCategory default color expected 200, got %d: %s", recDefaultColor.Code, recDefaultColor.Body.String())
	}
	createResponse = struct {
		Category struct {
			ID    int64  `json:"id"`
			Color string `json:"color"`
		} `json:"category"`
	}{}
	if err := json.Unmarshal(recDefaultColor.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("decode default-color response: %v", err)
	}
	if createResponse.Category.ID != 0 || createResponse.Category.Color != "#d8b33d" {
		t.Fatalf("default color response compatibility changed: %#v", createResponse.Category)
	}

	// 3. Create Category Bad Name -> 400
	bodyBadCreate := bytes.NewBufferString(`{"name":"","color":"#ff0000"}`)
	reqBadCreate := httptest.NewRequest("POST", "/api/admin/key-categories", bodyBadCreate)
	recBadCreate := httptest.NewRecorder()
	handler.CreateCategory(recBadCreate, reqBadCreate)
	if recBadCreate.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty category name, got %d", recBadCreate.Code)
	}

	// 4. Update Category
	bodyUpdate := bytes.NewBufferString(`{"old_name":"Cat1","new_name":"Cat1Renamed","color":"#00ff00"}`)
	reqUpdate := httptest.NewRequest("PUT", "/api/admin/key-categories", bodyUpdate)
	recUpdate := httptest.NewRecorder()
	handler.UpdateCategory(recUpdate, reqUpdate)
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("UpdateCategory expected 200, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}

	bodyMissingUpdate := bytes.NewBufferString(`{"old_name":"Missing","new_name":"StillMissing","color":"#00ff00"}`)
	reqMissingUpdate := httptest.NewRequest("PUT", "/api/admin/key-categories", bodyMissingUpdate)
	recMissingUpdate := httptest.NewRecorder()
	handler.UpdateCategory(recMissingUpdate, reqMissingUpdate)
	if recMissingUpdate.Code != http.StatusNotFound {
		t.Fatalf("missing category update expected 404, got %d: %s", recMissingUpdate.Code, recMissingUpdate.Body.String())
	}

	// 5. Rename Category
	bodyRename := bytes.NewBufferString(`{"old_name":"Cat1Renamed","new_name":"Cat1Final"}`)
	reqRename := httptest.NewRequest("PUT", "/api/admin/key-categories/rename", bodyRename)
	recRename := httptest.NewRecorder()
	handler.RenameCategory(recRename, reqRename)
	if recRename.Code != http.StatusOK {
		t.Fatalf("RenameCategory expected 200, got %d: %s", recRename.Code, recRename.Body.String())
	}

	// 6. Reorder Categories (Includes partial and unknown name)
	bodyReorderCat := bytes.NewBufferString(`{"names":["Cat1Final", "UnknownCat"]}`)
	reqReorderCat := httptest.NewRequest("PUT", "/api/admin/key-categories/order", bodyReorderCat)
	recReorderCat := httptest.NewRecorder()
	handler.ReorderCategories(recReorderCat, reqReorderCat)
	if recReorderCat.Code != http.StatusOK {
		t.Fatalf("ReorderCategories expected 200, got %d: %s", recReorderCat.Code, recReorderCat.Body.String())
	}

	// 7. Delete Category
	bodyDeleteCat := bytes.NewBufferString(`{"name":"Cat1Final","mode":"keep_keys"}`)
	reqDeleteCat := httptest.NewRequest("POST", "/api/admin/key-categories/delete", bodyDeleteCat)
	recDeleteCat := httptest.NewRecorder()
	handler.DeleteCategory(recDeleteCat, reqDeleteCat)
	if recDeleteCat.Code != http.StatusOK {
		t.Fatalf("DeleteCategory expected 200, got %d: %s", recDeleteCat.Code, recDeleteCat.Body.String())
	}

	// 8. Reorder Keys (Empty set) -> 200
	bodyReorderKeys := bytes.NewBufferString(`{"ids":[]}`)
	reqReorderKeys := httptest.NewRequest("PUT", "/api/admin/keys/order", bodyReorderKeys)
	recReorderKeys := httptest.NewRecorder()
	handler.ReorderKeys(recReorderKeys, reqReorderKeys)
	if recReorderKeys.Code != http.StatusOK {
		t.Fatalf("ReorderKeys expected 200, got %d: %s", recReorderKeys.Code, recReorderKeys.Body.String())
	}

	// 9. Reorder Keys Unknown ID -> 400
	bodyReorderBadKey := bytes.NewBufferString(`{"ids":[999]}`)
	reqReorderBadKey := httptest.NewRequest("PUT", "/api/admin/keys/order", bodyReorderBadKey)
	recReorderBadKey := httptest.NewRecorder()
	handler.ReorderKeys(recReorderBadKey, reqReorderBadKey)
	if recReorderBadKey.Code != http.StatusBadRequest {
		t.Fatalf("ReorderKeys expected 400 for unknown key, got %d: %s", recReorderBadKey.Code, recReorderBadKey.Body.String())
	}

	if len(auditLog) != 1 || auditLog[0].eventName != "keys.reorder" || auditLog[0].entityID != "multiple" {
		t.Fatalf("unexpected audit log: %#v", auditLog)
	}
}
