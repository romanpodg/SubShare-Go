package main

import (
	"net/http"
	"testing"
)

func seedSourceFetchHeaders(t *testing.T, app *App, sourceID int64) {
	t.Helper()
	execRepositoryFixtureSQL(t, app, `UPDATE external_subscription_sources SET pass_hwid = 1, hwid_version = 'fixture-v1', hwid_model_name = 'fixture-model1', hwid_value = 'fixture-device1' WHERE id = ?`, sourceID)
}

func TestJobSourceHeaderChangesInvalidateCapturedFeed(t *testing.T) {
	cases := []struct{ name, update string }{
		{"pass", `UPDATE external_subscription_sources SET pass_hwid = 0 WHERE id = ?`},
		{"version", `UPDATE external_subscription_sources SET hwid_version = 'fixture-v2' WHERE id = ?`},
		{"model", `UPDATE external_subscription_sources SET hwid_model_name = 'fixture-model2' WHERE id = ?`},
		{"identity", `UPDATE external_subscription_sources SET hwid_value = 'fixture-device2' WHERE id = ?`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f, sourceID, fetch := newJobSourceFixture(t)
			seedSourceFetchHeaders(t, f.app, sourceID)
			jobID, done := startControlledSourceJob(t, f, sourceID)
			request := waitJobFetch(t, fetch)
			requireRepositoryEqual(t, "captured device header", request.Header.Get("X-HWID"), "fixture-device1")
			execRepositoryFixtureSQL(t, f.app, test.update, sourceID)
			close(fetch.release)
			waitControlledSourceJob(t, done)
			status, _, _, _ := jobLifecycleState(t, f.app, jobID)
			requireRepositoryEqual(t, "changed headers reject old feed", status, "failed")
			requireRepositoryEqual(t, "changed headers create no profile", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys`), 0)
			assertJobSourceStatus(t, f.app, sourceID, "error")
		})
	}
}

func TestJobSourceInactiveHeaderEditKeepsEquivalentRequest(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	jobID, done := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, fetch)
	execRepositoryFixtureSQL(t, f.app, `UPDATE external_subscription_sources SET hwid_version = 'unused-version' WHERE id = ?`, sourceID)
	close(fetch.release)
	waitControlledSourceJob(t, done)
	status, _, _, _ := jobLifecycleState(t, f.app, jobID)
	requireRepositoryEqual(t, "inactive headers retain equivalent fetch", status, "succeeded")
}

func TestJobSourceInvalidatedFailureKeepsCurrentNonSyncingState(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	seedSourceFetchHeaders(t, f.app, sourceID)
	fetch.status = http.StatusServiceUnavailable
	_, done := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, fetch)
	execRepositoryFixtureSQL(t, f.app, `UPDATE external_subscription_sources SET pass_hwid = 0, import_status = 'ok' WHERE id = ?`, sourceID)
	close(fetch.release)
	waitControlledSourceJob(t, done)
	assertJobSourceStatus(t, f.app, sourceID, "ok")
}
