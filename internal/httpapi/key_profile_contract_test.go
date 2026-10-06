package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

const profileHTTPContractURI = "hysteria2://http-private-auth@edge.example:443?sni=edge.example&insecure=1&x-extra=private-query#Embedded"

type profileHTTPContractFixture struct {
	handler *KeyProfileHandler
	repo    *storage.Repository
	id      int64
	audits  *[]auditRecord
}

func newProfileHTTPContractFixture(t *testing.T) profileHTTPContractFixture {
	t.Helper()
	db := setupTestDB(t)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	repo := storage.NewRepository(db, newTestKeyringForHTTPAPI(t))
	key, _, err := repo.CreateLocal(context.Background(), keymanagement.CreateProfileParams{Label: "Panel", Status: "active", Kind: "real", Protocol: "hysteria2", BuiltURI: profileHTTPContractURI})
	if err != nil {
		t.Fatal(err)
	}
	audits := []auditRecord{}
	handler := NewKeyProfileHandler(keymanagement.NewService(repo, nil), func(_ *http.Request, event, entity, id string, metadata map[string]any) {
		audits = append(audits, auditRecord{event, entity, id, metadata})
	})
	return profileHTTPContractFixture{handler, repo, key.ID, &audits}
}

func profileHTTPContractRequest(id int64, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPut, "/api/v1/keys/"+strconv.FormatInt(id, 10), strings.NewReader(body))
	r.SetPathValue("id", strconv.FormatInt(id, 10))
	return r
}

func assertProfileHTTPSecretFree(t *testing.T, content string) {
	t.Helper()
	for _, secret := range []string{profileHTTPContractURI, "http-private-auth", "private-query", "raw_uri", "encrypted_url", "url_blind_index"} {
		if strings.Contains(content, secret) {
			t.Fatal("profile response or audit disclosed secret material")
		}
	}
}

func profileHTTPEqual(t *testing.T, label string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s differs from contract", label)
	}
}

func profileHTTPSuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertProfileHTTPError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	profileHTTPEqual(t, "HTTP error status", response.Code, status)
	profileHTTPEqual(t, "HTTP error cache policy", response.Header().Get("Cache-Control"), "no-store, private")
	var envelope map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	profileHTTPEqual(t, "error code", envelope["code"], code)
	profileHTTPEqual(t, "error present", envelope["error"] != nil, true)
	profileHTTPEqual(t, "no data on error", envelope["data"], nil)
	profileHTTPEqual(t, "empty field errors", envelope["field_errors"], map[string]any{})
	assertProfileHTTPSecretFree(t, response.Body.String())
}

func TestProfileHTTPPatchFalseVersusOmittedAndSameRevision(t *testing.T) {
	for _, tc := range []struct{ name, patch, query string }{
		{"omitted", `{}`, "1"},
		{"false", `{"hysteria2":{"insecure":{"operation":"set","value":false}}}`, "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newProfileHTTPContractFixture(t)
			body := `{"profile_revision":1,"label":"Renamed","status":"active","kind":"real","patch_mode":"structured","structured_patch":` + tc.patch + `}`
			response := httptest.NewRecorder()
			f.handler.UpdateKeyProfile(response, profileHTTPContractRequest(f.id, body))
			profileHTTPEqual(t, "patch response status", response.Code, http.StatusOK)
			profileHTTPEqual(t, "patch cache policy", response.Header().Get("Cache-Control"), "no-store, private")
			assertProfileHTTPSecretFree(t, response.Body.String())
			key, raw, err := f.repo.GetByID(context.Background(), f.id)
			profileHTTPSuccess(t, err)
			profileHTTPEqual(t, "patch revision", key.ProfileRevision, int64(2))
			profileHTTPEqual(t, "patch label", key.Label, "Renamed")
			profileHTTPEqual(t, "omitted/set boolean", strings.Contains(raw, "insecure="+tc.query), true)
			if tc.name == "omitted" {
				profileHTTPEqual(t, "metadata raw byte parity", raw, profileHTTPContractURI)
			}
			var detail struct {
				Data struct {
					ProfileRevision int64 `json:"profile_revision"`
				} `json:"data"`
			}
			profileHTTPSuccess(t, json.Unmarshal(response.Body.Bytes(), &detail))
			profileHTTPEqual(t, "response revision", detail.Data.ProfileRevision, int64(2))
			conflict := httptest.NewRecorder()
			f.handler.UpdateKeyProfile(conflict, profileHTTPContractRequest(f.id, body))
			assertProfileHTTPError(t, conflict, http.StatusConflict, "profile_revision_conflict")
			profileHTTPEqual(t, "one success audit", len(*f.audits), 1)
			profileHTTPEqual(t, "update audit event", (*f.audits)[0].eventName, "key.update")
			auditJSON, err := json.Marshal((*f.audits)[0].metadata)
			profileHTTPSuccess(t, err)
			assertProfileHTTPSecretFree(t, string(auditJSON))
		})
	}
}

func TestProfileHTTPStrictDTOAndPatchErrorsDoNotWrite(t *testing.T) {
	for _, tc := range []struct{ name, fields, code string }{
		{"unknown field", `"unexpected":true,"structured_patch":{}`, "invalid_body"},
		{"wrong boolean type", `"structured_patch":{"hysteria2":{"insecure":{"operation":"set","value":"false"}}}`, "invalid_body"},
		{"mixed modes", `"patch_mode":"raw","raw_uri":"` + profileHTTPContractURI + `","structured_patch":{}`, "mutually_exclusive_mode"},
		{"missing patch", `"patch_mode":"structured"`, "structured_patch_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newProfileHTTPContractFixture(t)
			body := `{"profile_revision":1,"label":"Panel","status":"active","kind":"real",` + tc.fields + `}`
			response := httptest.NewRecorder()
			f.handler.UpdateKeyProfile(response, profileHTTPContractRequest(f.id, body))
			assertProfileHTTPError(t, response, http.StatusBadRequest, tc.code)
			key, raw, err := f.repo.GetByID(context.Background(), f.id)
			profileHTTPSuccess(t, err)
			profileHTTPEqual(t, "rejected patch revision", key.ProfileRevision, int64(1))
			profileHTTPEqual(t, "rejected patch raw bytes", raw, profileHTTPContractURI)
			profileHTTPEqual(t, "no audit on rejection", len(*f.audits), 0)
		})
	}
}
