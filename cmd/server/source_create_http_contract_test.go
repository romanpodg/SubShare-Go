package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSourceCreateDuplicateAfterFetchPreflightRollsBackTentativeCategories(t *testing.T) {
	f := newUserMutationFixture(t)
	fetch := &jobFetchTransport{make(chan *http.Request, 1), make(chan struct{}), sourceTestLink, http.StatusOK}
	f.app.sourceHTTPClient = &http.Client{Transport: fetch, Timeout: 5 * time.Second}
	sourceURL := "https://provider.example/create-concurrent"
	firstBody := sourceCreateContractBody(t, sourceURL, "")
	var firstPayload map[string]any
	requireRepositorySuccess(t, json.Unmarshal([]byte(firstBody), &firstPayload))
	firstPayload["category"] = "CreateLoserCategory"
	firstPayload["key_category"] = "CreateLoserProfiles"
	firstBody = mutationJSON(t, firstPayload)
	response := make(chan *httptest.ResponseRecorder, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-fetch.release:
		default:
			close(fetch.release)
		}
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("create request did not finish before fixture cleanup")
		}
	})
	go func() { defer close(done); response <- f.request(http.MethodPost, "/api/v1/sources", firstBody) }()
	waitJobFetch(t, fetch)
	second := f.request(http.MethodPost, "/api/v1/sources", sourceCreateContractBody(t, sourceURL, sourceTestLink))
	requireRepositoryEqual(t, "winner source create", second.Code, http.StatusCreated)
	before := sourceCreateMutationCounts(t, f.app)
	close(fetch.release)
	select {
	case first := <-response:
		requireRepositoryEqual(t, "stale preflight duplicate", first.Code, http.StatusConflict)
		payload := decodeJSONMap(t, first)
		requireRepositoryEqual(t, "duplicate code", payload["code"], "source_url_conflict")
		fields := payload["field_errors"].(map[string]any)
		requireRepositoryEqual(t, "duplicate field", fields["source_url"], []any{"source URL already exists"})
	case <-time.After(10 * time.Second):
		t.Fatal("duplicate create did not terminate")
	}
	requireRepositoryEqual(t, "duplicate commits no second unit or audit", sourceCreateMutationCounts(t, f.app), before)
}

func TestSourceCreateSuccessUsesSharedCategoriesAndAtomicAssignments(t *testing.T) {
	f := newUserMutationFixture(t)
	seedSourceCreateAllModeUser(t, f.app)
	for _, sourceURL := range []string{"https://provider.example/create-one", "https://provider.example/create-two"} {
		recorder := f.request(http.MethodPost, "/api/v1/sources", sourceCreateContractBody(t, sourceURL, sourceTestLink))
		requireRepositoryEqual(t, "create success", recorder.Code, http.StatusCreated)
		payload := decodeJSONMap(t, recorder)
		requireRepositoryEqual(t, "create imported count", payload["imported_count"], float64(1))
		requireRepositoryEqual(t, "create warnings array", payload["warnings"], []any{})
		data := payload["data"].(map[string]any)
		requireRepositoryEqual(t, "committed source status", data["import_status"], "ok")
		requireRepositoryEqual(t, "safe device-value projection", data["has_hwid_value"], true)
		requireRepositoryEqual(t, "device value absent", data["hwid_value"], nil)
	}
	requireRepositoryEqual(t, "shared source category", mutationCount(t, f.app, `SELECT COUNT(*) FROM external_source_categories WHERE name = 'CreateFixtureCategory'`), 1)
	requireRepositoryEqual(t, "shared profile category", mutationCount(t, f.app, `SELECT COUNT(*) FROM key_categories WHERE name = 'CreateFixtureProfiles'`), 1)
	requireRepositoryEqual(t, "all-mode assignment count", mutationCount(t, f.app, `SELECT COUNT(*) FROM user_keys`), 2)
	requireRepositoryEqual(t, "committed credential count", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_key_secrets`), 2)
	requireRepositoryEqual(t, "create audit count", mutationCount(t, f.app, `SELECT COUNT(*) FROM audit_events WHERE action = 'external_source.create'`), 2)
}
