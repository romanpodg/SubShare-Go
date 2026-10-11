package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
)

type apiContractCase struct {
	Method, Path           string
	OpenAPIPath            string `json:"openapi_path"`
	Anonymous              bool
	Status                 int
	Body, Response, Stored json.RawMessage
}

func loadAPIContracts(t *testing.T) map[string]apiContractCase {
	t.Helper()
	data, err := os.ReadFile("../../testdata/api/contracts.json")
	requireRepositorySuccess(t, err)
	var fixture struct{ Cases map[string]apiContractCase }
	requireRepositorySuccess(t, json.Unmarshal(data, &fixture))
	return fixture.Cases
}

func newAPIContractFixture(t *testing.T) userMutationFixture {
	t.Helper()
	f := newUserMutationFixture(t)
	execRepositoryFixtureSQL(t, f.app, `INSERT INTO users(id,name,email,token,activation_code,subscription_id,status,key_assignment_mode,created_at) VALUES(1,'Contract user','contract@example.test','fixture-token','fixture-activation','fixture-subscription','active','selected','2026-01-02 03:04:05')`)
	sourceID := seedExternalProfileSource(t, f.app, "https://provider.example/contract")
	execRepositoryFixtureSQL(t, f.app, `UPDATE external_subscription_sources SET created_at='2026-01-02 03:04:05', updated_at='2026-01-02 03:04:05' WHERE id=?`, sourceID)
	profile := seedStartupEncryptedProfile(t, f.app.db, f.app.profileKeyring)
	execRepositoryFixtureSQL(t, f.app, `UPDATE vless_keys SET created_at='2026-01-02 03:04:05', updated_at='2026-01-02 03:04:05' WHERE id=?`, profile.id)
	info, _, err := f.app.store().CreateLocal(context.Background(), keymanagement.CreateProfileParams{Label: "Contract info", Status: "active", Kind: "informational", TemplateText: "Read <literal> text"})
	requireRepositorySuccess(t, err)
	execRepositoryFixtureSQL(t, f.app, `UPDATE vless_keys SET created_at='2026-01-02 03:04:05', updated_at='2026-01-02 03:04:05' WHERE id=?`, info.ID)
	return f
}

func apiContractRequest(f userMutationFixture, c apiContractCase) *httptest.ResponseRecorder {
	if !c.Anonymous {
		return f.request(c.Method, c.Path, string(c.Body))
	}
	r := httptest.NewRequest(c.Method, c.Path, nil)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}

func apiContractExpected(t *testing.T, name string, raw json.RawMessage) any {
	t.Helper()
	var expected any
	requireRepositorySuccess(t, json.Unmarshal(raw, &expected))
	if name == "source-list" {
		for _, item := range expected.(map[string]any)["data"].([]any) {
			localizeAPIContractSource(item.(map[string]any))
		}
	}
	if name == "source-detail" {
		localizeAPIContractSource(expected.(map[string]any)["data"].(map[string]any))
	}
	return expected
}

// Source timestamps historically use host-local wall-clock text. Check that
// exact conversion without normalizing any other output or timestamp.
func localizeAPIContractSource(source map[string]any) {
	local := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).In(time.Local).Format("2006-01-02 15:04:05")
	source["created_at"], source["updated_at"] = local, local
}

func TestAPIContractRegisteredReadResponses(t *testing.T) {
	f := newAPIContractFixture(t)
	for name, c := range loadAPIContracts(t) {
		if c.Method != http.MethodGet {
			continue
		}
		t.Run(name, func(t *testing.T) {
			w := apiContractRequest(f, c)
			requireRepositoryEqual(t, "status", w.Code, c.Status)
			requireRepositoryEqual(t, "response", decodeJSONMap(t, w), apiContractExpected(t, name, c.Response))
		})
	}
}
