package main

import (
	"fmt"
	"net/http"
	"testing"
)

func TestJobRecoveryFailuresRemainIndependentAndRetryable(t *testing.T) {
	cases := []struct {
		table                           string
		jobState, runState, sourceState string
	}{
		{"background_jobs", "queued", "failed", "error"},
		{"source_sync_runs", "failed", "running", "error"},
		{"external_subscription_sources", "failed", "failed", "syncing"},
	}
	for _, test := range cases {
		t.Run(test.table, func(t *testing.T) {
			f, sourceID, _ := newJobSourceFixture(t)
			f.app.queueTrackedJob("fixture", "fixture", "1")
			f.app.startSourceSyncRun(sourceID)
			f.app.markExternalSourceStatus(sourceID, "syncing", "")
			execRepositoryFixtureSQL(t, f.app, fmt.Sprintf(`CREATE TRIGGER fixture_recovery_failure BEFORE UPDATE ON %s BEGIN SELECT RAISE(ABORT, 'fixture recovery failure'); END`, test.table))
			f.app.recoverInterruptedJobs()
			assertJobRecoveryStates(t, f.app, [3]string{test.jobState, test.runState, test.sourceState})
			execRepositoryFixtureSQL(t, f.app, `DROP TRIGGER fixture_recovery_failure`)
			f.app.recoverInterruptedJobs()
			assertJobRecoveryStates(t, f.app, [3]string{"failed", "failed", "error"})
		})
	}
}

func assertJobRecoveryStates(t *testing.T, app *App, want [3]string) {
	t.Helper()
	var job, run, source string
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT status FROM background_jobs`).Scan(&job))
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT status FROM source_sync_runs`).Scan(&run))
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT import_status FROM external_subscription_sources`).Scan(&source))
	requireRepositoryEqual(t, "independent restart recovery states", [3]string{job, run, source}, want)
}

func TestJobSourceStatusWritesAreBestEffort(t *testing.T) {
	cases := []struct {
		name, transition, before, after, jobStatus string
		responseStatus                             int
	}{
		{"syncing", "syncing", "idle", "ok", "succeeded", http.StatusOK},
		{"error", "error", "syncing", "syncing", "failed", http.StatusServiceUnavailable},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f, sourceID, fetch := newJobSourceFixture(t)
			fetch.status = test.responseStatus
			execRepositoryFixtureSQL(t, f.app, fmt.Sprintf(`CREATE TRIGGER fixture_source_status_failure BEFORE UPDATE ON external_subscription_sources WHEN NEW.import_status = '%s' BEGIN SELECT RAISE(ABORT, 'fixture status failure'); END`, test.transition))
			jobID, done := startControlledSourceJob(t, f, sourceID)
			waitJobFetch(t, fetch)
			assertJobSourceStatus(t, f.app, sourceID, test.before)
			close(fetch.release)
			waitControlledSourceJob(t, done)
			assertJobSourceStatus(t, f.app, sourceID, test.after)
			status, _, _, _ := jobLifecycleState(t, f.app, jobID)
			requireRepositoryEqual(t, "best-effort source status worker result", status, test.jobStatus)
		})
	}
}

func assertJobSourceStatus(t *testing.T, app *App, sourceID int64, want string) {
	t.Helper()
	var got string
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT import_status FROM external_subscription_sources WHERE id = ?`, sourceID).Scan(&got))
	requireRepositoryEqual(t, "source status write outcome", got, want)
}
