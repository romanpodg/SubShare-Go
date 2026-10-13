package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

type sourceUpdateFixture struct {
	userMutationFixture
	id        int64
	url, path string
}

func newSourceUpdateFixture(t *testing.T, f userMutationFixture) sourceUpdateFixture {
	t.Helper()
	sourceURL := "https://provider.example/update-contract"
	response := f.request(http.MethodPost, "/api/v1/sources", sourceCreateContractBody(t, sourceURL, sourceTestLink))
	requireRepositoryEqual(t, "seed source", response.Code, http.StatusCreated)
	var id int64
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT id FROM external_subscription_sources WHERE source_url = ?`, sourceURL).Scan(&id))
	return sourceUpdateFixture{f, id, sourceURL, "/api/v1/sources/" + strconv.FormatInt(id, 10)}
}

func (f sourceUpdateFixture) update(t *testing.T, overrides map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload := map[string]any{
		"name": "Updated source", "category": "CreateFixtureCategory", "key_category": "CreateFixtureProfiles",
		"key_insert_mode": "bottom", "source_url": f.url, "enabled": true, "pass_hwid": true,
		"apply_remote_metadata": false, "hwid_version": " 2.0 ", "hwid_model_name": " Updated ",
	}
	for key, value := range overrides {
		payload[key] = value
	}
	return f.request(http.MethodPut, f.path, mutationJSON(t, payload))
}

type sourceUpdateSnapshot struct {
	source, keys, credentials string
	audit                     int
}

func (f sourceUpdateFixture) snapshot(t *testing.T) sourceUpdateSnapshot {
	t.Helper()
	var state sourceUpdateSnapshot
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT json_array(name, category, key_category, key_insert_mode, source_url, enabled,
		apply_remote_metadata, pass_hwid, hwid_version, hwid_model_name, hwid_value, updated_at)
		FROM external_subscription_sources WHERE id = ?`, f.id).Scan(&state.source))
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT json_group_array(json_array(id,label,status,profile_revision))
		FROM vless_keys WHERE external_source_id = ?`, f.id).Scan(&state.keys))
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT json_group_array(json_array(vless_key_id, encrypted_url))
		FROM vless_key_secrets WHERE vless_key_id IN (SELECT id FROM vless_keys WHERE external_source_id = ?)`, f.id).Scan(&state.credentials))
	state.audit = mutationCount(t, f.app, `SELECT COUNT(*) FROM audit_events WHERE action = 'external_source.update'`)
	return state
}

type sourceErrorWant struct {
	status               int
	code, message, field string
}

func assertSourceError(t *testing.T, response *httptest.ResponseRecorder, want sourceErrorWant) {
	t.Helper()
	requireRepositoryEqual(t, "source HTTP status", response.Code, want.status)
	payload := decodeJSONMap(t, response)
	requireRepositoryEqual(t, "source error code", payload["code"], want.code)
	requireRepositoryEqual(t, "source safe message", payload["error"], want.message)
	requireRepositoryEqual(t, "source v1 message", payload["message"], want.message)
	requireRepositoryEqual(t, "source request ID", payload["request_id"], "mutation-fixture")
	fields := map[string]any{}
	if want.field != "" {
		fields[want.field] = []any{want.message}
	}
	requireRepositoryEqual(t, "source field errors", payload["field_errors"], fields)
}
