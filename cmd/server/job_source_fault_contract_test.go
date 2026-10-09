package main

import (
	"strconv"
	"testing"
	"time"
)

type sourceJobFaultWant struct {
	jobStatus, runStatus string
	started              bool
	audits               int
}

type sourceJobIdentity struct{ jobID, sourceID int64 }

func TestJobSourceBestEffortPersistenceFailureContracts(t *testing.T) {
	cases := []struct {
		name, trigger string
		want          sourceJobFaultWant
	}{
		{"running", `CREATE TRIGGER fixture_running_failure BEFORE UPDATE ON background_jobs WHEN NEW.status = 'running' BEGIN SELECT RAISE(ABORT, 'fixture running failure'); END`, sourceJobFaultWant{"succeeded", "succeeded", false, 1}},
		{"run start", `CREATE TRIGGER fixture_run_start_failure BEFORE INSERT ON source_sync_runs BEGIN SELECT RAISE(ABORT, 'fixture run failure'); END`, sourceJobFaultWant{"succeeded", "", true, 1}},
		{"job finish", `CREATE TRIGGER fixture_job_finish_failure BEFORE UPDATE ON background_jobs WHEN NEW.status = 'succeeded' BEGIN SELECT RAISE(ABORT, 'fixture job finish failure'); END`, sourceJobFaultWant{"running", "succeeded", true, 1}},
		{"run finish", `CREATE TRIGGER fixture_run_finish_failure BEFORE UPDATE ON source_sync_runs WHEN NEW.status = 'succeeded' BEGIN SELECT RAISE(ABORT, 'fixture run finish failure'); END`, sourceJobFaultWant{"succeeded", "running", true, 1}},
		{"audit", `CREATE TRIGGER fixture_audit_failure BEFORE INSERT ON audit_events WHEN NEW.action = 'external_source.sync' BEGIN SELECT RAISE(ABORT, 'fixture audit failure'); END`, sourceJobFaultWant{"succeeded", "succeeded", true, 0}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f, sourceID, fetch := newJobSourceFixture(t)
			execRepositoryFixtureSQL(t, f.app, test.trigger)
			jobID, done := startControlledSourceJob(t, f, sourceID)
			waitJobFetch(t, fetch)
			close(fetch.release)
			waitControlledSourceJob(t, done)
			assertSourceJobFaultState(t, f.app, sourceJobIdentity{jobID, sourceID}, test.want)
		})
	}
}

func startControlledSourceJob(t *testing.T, f userMutationFixture, sourceID int64) (int64, <-chan struct{}) {
	t.Helper()
	var actorID int64
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT id FROM admins WHERE username = 'admin-owner'`).Scan(&actorID))
	id := f.app.queueTrackedJob("source_sync", "external_source", strconv.FormatInt(sourceID, 10))
	if id == 0 {
		t.Fatal("controlled job queue failed")
	}
	done := make(chan struct{})
	go func() {
		f.app.runQueuedSourceSync(id, sourceID, actorID, "job-fixture")
		close(done)
	}()
	return id, done
}

func waitControlledSourceJob(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("controlled source job did not return")
	}
}

func assertSourceJobFaultState(t *testing.T, app *App, identity sourceJobIdentity, want sourceJobFaultWant) {
	t.Helper()
	status, _, started, _ := jobLifecycleState(t, app, identity.jobID)
	requireRepositoryEqual(t, "best-effort job status", status, want.jobStatus)
	requireRepositoryEqual(t, "best-effort job start timestamp", started, want.started)
	var runStatus string
	if want.runStatus == "" {
		requireRepositoryEqual(t, "failed run start creates no record", mutationCount(t, app, `SELECT COUNT(*) FROM source_sync_runs WHERE source_id = ?`, identity.sourceID), 0)
	} else {
		requireRepositorySuccess(t, app.db.QueryRow(`SELECT status FROM source_sync_runs WHERE source_id = ?`, identity.sourceID).Scan(&runStatus))
		requireRepositoryEqual(t, "best-effort run status", runStatus, want.runStatus)
	}
	requireRepositoryEqual(t, "best-effort audit outcome", mutationCount(t, app, `SELECT COUNT(*) FROM audit_events WHERE action = 'external_source.sync'`), want.audits)
	requireRepositoryEqual(t, "best-effort writes retain imported profile", mutationCount(t, app, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, identity.sourceID), 1)
}
