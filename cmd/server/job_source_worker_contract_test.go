package main

import (
	"net/http"
	"strconv"
	"testing"
)

func TestJobSourceWorkerQueuedRunningSuccessContracts(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	jobID := queueJobSource(t, f, sourceID)
	request := waitJobFetch(t, fetch)
	requireRepositoryEqual(t, "worker fetched selected source", request.URL.String(), "https://provider.example/job-fixture")
	assertJobLifecycleState(t, f.app, jobID, [4]any{"running", "", true, false})
	var status string
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT import_status FROM external_subscription_sources WHERE id = ?`, sourceID).Scan(&status))
	requireRepositoryEqual(t, "source marked syncing before fetch", status, "syncing")
	close(fetch.release)
	requireRepositoryEqual(t, "terminal source job", waitJobSourceTerminal(t, f.app, jobID), "succeeded")
	requireRepositoryEqual(t, "persisted imported source profile", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID), 1)
	requireRepositoryEqual(t, "successful sync run", mutationCount(t, f.app, `SELECT COUNT(*) FROM source_sync_runs WHERE source_id = ? AND status = 'succeeded' AND imported_count = 1`, sourceID), 1)
}

func TestJobSourceWorkerFetchFailureContracts(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	fetch.status = http.StatusServiceUnavailable
	jobID := queueJobSource(t, f, sourceID)
	waitJobFetch(t, fetch)
	close(fetch.release)
	requireRepositoryEqual(t, "failed source job", waitJobSourceTerminal(t, f.app, jobID), "failed")
	requireRepositoryEqual(t, "failure creates no profile", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID), 0)
	requireRepositoryEqual(t, "failed source run", mutationCount(t, f.app, `SELECT COUNT(*) FROM source_sync_runs WHERE source_id = ? AND status = 'failed'`, sourceID), 1)
	var status string
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT import_status FROM external_subscription_sources WHERE id = ?`, sourceID).Scan(&status))
	requireRepositoryEqual(t, "failed source marked error", status, "error")
}

func TestJobSourceQueueDatabaseFailureWithholdsAcceptedJob(t *testing.T) {
	f, sourceID, _ := newJobSourceFixture(t)
	execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER fixture_queue_failure BEFORE INSERT ON background_jobs BEGIN SELECT RAISE(ABORT, 'fixture queue failure'); END`)
	recorder := f.request(http.MethodPost, "/api/v1/sources/"+strconv.FormatInt(sourceID, 10)+"/sync", "")
	requireRepositoryEqual(t, "queue failure status", recorder.Code, http.StatusInternalServerError)
	requireRepositoryEqual(t, "queue failure code", decodeJSONMap(t, recorder)["code"], "job_queue_failed")
	requireRepositoryEqual(t, "queue failure creates no job", mutationCount(t, f.app, `SELECT COUNT(*) FROM background_jobs`), 0)
	requireRepositoryEqual(t, "queue failure starts no run", mutationCount(t, f.app, `SELECT COUNT(*) FROM source_sync_runs`), 0)
}
