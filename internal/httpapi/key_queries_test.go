package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func TestKeyQueryHandler_V1ProjectionFilteringAndIntegrityFailure(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	kr := newTestKeyringForHTTPAPI(t)
	service := keymanagement.NewService(storage.NewRepository(db, kr), nil)
	alphaURL := "vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@example.com:443#alpha-secret"
	betaURL := "vless://bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb@example.com:443#beta-secret"
	alphaID, _, err := service.CreateLegacy(t.Context(), keymanagement.CreateLegacyParams{
		Label: "Alpha", URL: alphaURL, Category: "Primary", Kind: "real", Status: "active",
	})
	if err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	betaID, _, err := service.CreateLegacy(t.Context(), keymanagement.CreateLegacyParams{
		Label: "Beta", URL: betaURL, Category: "Secondary", Kind: "real", Status: "non-active",
	})
	if err != nil {
		t.Fatalf("create beta: %v", err)
	}
	if alphaID == betaID {
		t.Fatal("fixture created duplicate key IDs")
	}

	handler := NewKeyQueryHandler(service)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/keys?query=beta&status=non-active&page=1&page_size=1", nil)
	handler.ListKeys(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list keys: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), alphaURL) || strings.Contains(recorder.Body.String(), betaURL) || strings.Contains(recorder.Body.String(), `"url"`) {
		t.Fatalf("safe v1 list exposed credential material: %s", recorder.Body.String())
	}
	var response struct {
		Data []struct {
			ID                int64  `json:"id"`
			Label             string `json:"label"`
			ClientDisplayName string `json:"client_display_name"`
			Status            string `json:"status"`
		} `json:"data"`
		Meta struct {
			Page       int `json:"page"`
			PageSize   int `json:"page_size"`
			Total      int `json:"total"`
			TotalPages int `json:"total_pages"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode v1 list: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != betaID || response.Data[0].Label != "Beta" || response.Data[0].ClientDisplayName != "beta-secret" || response.Data[0].Status != "non-active" {
		t.Fatalf("unexpected v1 list data: %#v", response.Data)
	}
	if response.Meta.Page != 1 || response.Meta.PageSize != 1 || response.Meta.Total != 1 || response.Meta.TotalPages != 1 {
		t.Fatalf("unexpected v1 list meta: %#v", response.Meta)
	}

	recorder = httptest.NewRecorder()
	handler.ListCategories(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/key-categories", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"data"`) || !strings.Contains(recorder.Body.String(), "Primary") || !strings.Contains(recorder.Body.String(), "Secondary") {
		t.Fatalf("list categories: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	if _, err := db.Exec(`UPDATE vless_key_secrets SET encrypted_url = 'corrupt' WHERE vless_key_id = ?`, betaID); err != nil {
		t.Fatalf("corrupt beta secret: %v", err)
	}
	recorder = httptest.NewRecorder()
	handler.ListKeys(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil))
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), `"code":"keys_list_failed"`) {
		t.Fatalf("integrity failure: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), alphaURL) || strings.Contains(recorder.Body.String(), betaURL) || strings.Contains(recorder.Body.String(), "corrupt") {
		t.Fatalf("integrity response leaked storage or credential material: %s", recorder.Body.String())
	}
}
