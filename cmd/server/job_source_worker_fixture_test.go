package main

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type jobFetchTransport struct {
	entered chan *http.Request
	release chan struct{}
	body    string
	status  int
}

func (fetch *jobFetchTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	fetch.entered <- request
	select {
	case <-fetch.release:
	case <-request.Context().Done():
		return nil, request.Context().Err()
	}
	return &http.Response{StatusCode: fetch.status, Header: http.Header{"Content-Type": {"text/plain"}}, Body: io.NopCloser(strings.NewReader(fetch.body)), Request: request}, nil
}

func newJobSourceFixture(t *testing.T) (userMutationFixture, int64, *jobFetchTransport) {
	t.Helper()
	f := newUserMutationFixture(t)
	id := seedExternalProfileSource(t, f.app, "https://provider.example/job-fixture")
	fetch := &jobFetchTransport{make(chan *http.Request, 1), make(chan struct{}), externalTestVLESS, http.StatusOK}
	f.app.sourceHTTPClient = &http.Client{Transport: fetch, Timeout: 5 * time.Second}
	t.Cleanup(func() {
		select {
		case <-fetch.release:
		default:
			close(fetch.release)
		}
	})
	return f, id, fetch
}

func waitJobFetch(t *testing.T, fetch *jobFetchTransport) *http.Request {
	t.Helper()
	select {
	case request := <-fetch.entered:
		return request
	case <-time.After(5 * time.Second):
		t.Fatal("source worker did not reach the controlled fetch")
		return nil
	}
}

func waitJobSourceTerminal(t *testing.T, app *App, id int64) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		requireRepositorySuccess(t, app.db.QueryRow(`SELECT status FROM background_jobs WHERE id = ?`, id).Scan(&status))
		if jobSourceTerminalWritesComplete(t, app, id, status) {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("source worker did not reach its terminal write boundary")
	return ""
}

func jobSourceTerminalWritesComplete(t *testing.T, app *App, id int64, status string) bool {
	t.Helper()
	switch status {
	case "succeeded":
		return mutationCount(t, app, `SELECT COUNT(*) FROM audit_events WHERE action = 'external_source.sync' AND json_extract(metadata_json, '$.job_id') = ?`, id) == 1
	case "failed":
		return mutationCount(t, app, `SELECT COUNT(*) FROM source_sync_runs WHERE status = 'running'`) == 0
	default:
		return false
	}
}

func queueJobSource(t *testing.T, f userMutationFixture, sourceID int64) int64 {
	t.Helper()
	recorder := f.request(http.MethodPost, "/api/v1/sources/"+strconv.FormatInt(sourceID, 10)+"/sync", "")
	requireRepositoryEqual(t, "source queue status", recorder.Code, http.StatusAccepted)
	payload := decodeJSONMap(t, recorder)
	requireRepositoryEqual(t, "source queue envelope", len(payload), 2)
	requireRepositoryEqual(t, "source queue response state", payload["status"], "queued")
	return int64(payload["job_id"].(float64))
}
