package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func sourceCreateContractBody(t *testing.T, sourceURL, rawBody string) string {
	t.Helper()
	var payload map[string]any
	requireRepositorySuccess(t, json.Unmarshal(sourceRequestBody(t, sourceURL, rawBody, "fixture-device"), &payload))
	payload["category"] = "CreateFixtureCategory"
	payload["key_category"] = "CreateFixtureProfiles"
	body, err := json.Marshal(payload)
	requireRepositorySuccess(t, err)
	return string(body)
}
func sourceCreateContractRequest(t *testing.T, f userMutationFixture) *httptest.ResponseRecorder {
	t.Helper()
	body := sourceTestLink + "\n" + "vless://22222222-2222-4222-8222-222222222222@second.example:443?security=tls&type=tcp#Second"
	return f.request(http.MethodPost, "/api/v1/sources", sourceCreateContractBody(t, "https://provider.example/create-contract", body))
}
func sourceCreateMutationCounts(t *testing.T, app *App) [7]int {
	t.Helper()
	var result [7]int
	for i, table := range []string{"external_subscription_sources", "external_source_categories", "key_categories", "vless_keys", "vless_key_secrets", "user_keys", "audit_events WHERE action = 'external_source.create'"} {
		result[i] = mutationCount(t, app, "SELECT COUNT(*) FROM "+table)
	}
	return result
}
func seedSourceCreateAllModeUser(t *testing.T, app *App) {
	t.Helper()
	execRepositoryFixtureSQL(t, app, `INSERT INTO users(name,token,status,key_assignment_mode) VALUES('create-fixture-user','create-fixture-token','active','all')`)
}
