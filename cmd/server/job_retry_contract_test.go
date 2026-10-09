package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func jobRetryRequest(f userMutationFixture, id int64) *httptest.ResponseRecorder {
	return f.request(http.MethodPost, "/api/v1/jobs/"+strconv.FormatInt(id, 10)+"/retry", "")
}

func TestJobRetryEligibilityContracts(t *testing.T) {
	cases := []struct{ name, kind, status, target, code string }{
		{"queued", "source_sync", "queued", "1", "job_not_failed"},
		{"running", "source_sync", "running", "1", "job_not_failed"},
		{"succeeded", "source_sync", "succeeded", "1", "job_not_failed"},
		{"unknown kind", "fixture", "failed", "1", "job_not_retryable"},
		{"malformed target", "source_sync", "failed", "not-an-id", "job_target_invalid"},
		{"zero target", "source_sync", "failed", "0", "job_target_invalid"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := f.app.queueTrackedJob(test.kind, "fixture", test.target)
			execRepositoryFixtureSQL(t, f.app, `UPDATE background_jobs SET status = ? WHERE id = ?`, test.status, id)
			recorder := jobRetryRequest(f, id)
			requireRepositoryEqual(t, "retry rejected status", recorder.Code, http.StatusConflict)
			requireRepositoryEqual(t, "retry rejected code", decodeJSONMap(t, recorder)["code"], test.code)
			requireRepositoryEqual(t, "retry rejection creates no job", mutationCount(t, f.app, `SELECT COUNT(*) FROM background_jobs`), 1)
		})
	}
}

func TestJobRetryDeletedSourceAcceptedThenFails(t *testing.T) {
	f, sourceID, _ := newJobSourceFixture(t)
	id := f.app.queueTrackedJob("source_sync", "external_source", strconv.FormatInt(sourceID, 10))
	execRepositoryFixtureSQL(t, f.app, `UPDATE background_jobs SET status = 'failed' WHERE id = ?`, id)
	execRepositoryFixtureSQL(t, f.app, `DELETE FROM external_subscription_sources WHERE id = ?`, sourceID)
	recorder := jobRetryRequest(f, id)
	requireRepositoryEqual(t, "deleted target retry accepted", recorder.Code, http.StatusAccepted)
	payload := decodeJSONMap(t, recorder)
	requireRepositoryEqual(t, "retry source identity", payload["retry_of"], float64(id))
	newID := int64(payload["job_id"].(float64))
	requireRepositoryEqual(t, "deleted target worker fails", waitJobSourceTerminal(t, f.app, newID), "failed")
}

func TestJobRetryRepeatedFailedSourceCreatesIndependentJobs(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	id := f.app.queueTrackedJob("source_sync", "external_source", strconv.FormatInt(sourceID, 10))
	execRepositoryFixtureSQL(t, f.app, `UPDATE background_jobs SET status = 'failed' WHERE id = ?`, id)
	close(fetch.release)
	var retries []int64
	for count := 0; count < 2; count++ {
		recorder := jobRetryRequest(f, id)
		requireRepositoryEqual(t, "repeated retry accepted", recorder.Code, http.StatusAccepted)
		retries = append(retries, int64(decodeJSONMap(t, recorder)["job_id"].(float64)))
		waitJobFetch(t, fetch)
		// Repeated retry acceptance is independent of the original failed job.
		// Join each completion here; concurrent fetch behavior has its own gated cases.
		requireRepositoryEqual(t, "repeated retry terminal state", waitJobSourceTerminal(t, f.app, retries[count]), "succeeded")
	}
	requireRepositoryEqual(t, "retry IDs are distinct", retries[0] != retries[1], true)
	requireRepositoryEqual(t, "repeated retry avoids duplicate profile", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID), 1)
}

func TestJobRetryMissingAndDatabaseReadFailureContracts(t *testing.T) {
	f := newUserMutationFixture(t)
	missing := jobRetryRequest(f, 900000)
	requireRepositoryEqual(t, "missing retry status", missing.Code, http.StatusNotFound)
	requireRepositoryEqual(t, "missing retry code", decodeJSONMap(t, missing)["code"], "job_not_found")
	execRepositoryFixtureSQL(t, f.app, `DROP TABLE background_jobs`)
	failed := jobRetryRequest(f, 900000)
	requireRepositoryEqual(t, "retry read failure status", failed.Code, http.StatusInternalServerError)
	requireRepositoryEqual(t, "retry read failure code", decodeJSONMap(t, failed)["code"], "job_load_failed")
}

func TestJobRetryQueueFailureWithholdsAcceptedJob(t *testing.T) {
	f := newUserMutationFixture(t)
	id := f.app.queueTrackedJob("keys_health_check", "key", "all")
	execRepositoryFixtureSQL(t, f.app, `UPDATE background_jobs SET status = 'failed' WHERE id = ?`, id)
	execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER fixture_retry_queue_failure BEFORE INSERT ON background_jobs BEGIN SELECT RAISE(ABORT, 'fixture retry queue failure'); END`)
	recorder := jobRetryRequest(f, id)
	requireRepositoryEqual(t, "retry queue failure status", recorder.Code, http.StatusInternalServerError)
	requireRepositoryEqual(t, "retry queue failure code", decodeJSONMap(t, recorder)["code"], "job_queue_failed")
	requireRepositoryEqual(t, "retry queue failure retains original only", mutationCount(t, f.app, `SELECT COUNT(*) FROM background_jobs`), 1)
}

func TestJobRetryKeyHealthCheckUsesAllTargetsForLegacyTargetText(t *testing.T) {
	f := newUserMutationFixture(t)
	id := f.app.queueTrackedJob("keys_health_check", "key", "legacy-target")
	execRepositoryFixtureSQL(t, f.app, `UPDATE background_jobs SET status = 'failed' WHERE id = ?`, id)
	recorder := jobRetryRequest(f, id)
	requireRepositoryEqual(t, "health retry accepted", recorder.Code, http.StatusAccepted)
	payload := decodeJSONMap(t, recorder)
	newID := int64(payload["job_id"].(float64))
	deadline := time.Now().Add(10 * time.Second)
	for mutationCount(t, f.app, `SELECT COUNT(*) FROM audit_events WHERE action = 'keys.health_check' AND json_extract(metadata_json, '$.job_id') = ?`, newID) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("health retry did not complete its audit write")
		}
		time.Sleep(10 * time.Millisecond)
	}
	assertJobLifecycleState(t, f.app, newID, [4]any{"succeeded", "", true, true})
	var target string
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT target_id FROM background_jobs WHERE id = ?`, newID).Scan(&target))
	requireRepositoryEqual(t, "health retry preserves legacy target text", target, "legacy-target")
}
