package main

import (
	"errors"
	"testing"
)

func jobLifecycleState(t *testing.T, app *App, id int64) (string, string, bool, bool) {
	t.Helper()
	var status, message string
	var started, finished bool
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT status, error_message, started_at IS NOT NULL, finished_at IS NOT NULL FROM background_jobs WHERE id = ?`, id).Scan(&status, &message, &started, &finished))
	return status, message, started, finished
}

func assertJobLifecycleState(t *testing.T, app *App, id int64, want [4]any) {
	t.Helper()
	status, message, started, finished := jobLifecycleState(t, app, id)
	requireRepositoryEqual(t, "job lifecycle state", [4]any{status, message, started, finished}, want)
}

func TestJobLifecycleQueuedRunningTerminalContracts(t *testing.T) {
	app := newIntegrationApp(t)
	id := app.queueTrackedJob("fixture", "fixture", "1")
	if id == 0 {
		t.Fatal("job queue failed")
	}
	assertJobLifecycleState(t, app, id, [4]any{"queued", "", false, false})
	app.markTrackedJobRunning(id)
	assertJobLifecycleState(t, app, id, [4]any{"running", "", true, false})
	app.finishTrackedJob(id, nil)
	assertJobLifecycleState(t, app, id, [4]any{"succeeded", "", true, true})
	app.markTrackedJobRunning(id)
	assertJobLifecycleState(t, app, id, [4]any{"succeeded", "", true, true})
	app.finishTrackedJob(id, errors.New("fixture failure"))
	// Existing terminal completion is an unconditional update, not a CAS.
	assertJobLifecycleState(t, app, id, [4]any{"failed", "fixture failure", true, true})
}

func TestJobLifecycleRejectedQueueAndRunningWriteContracts(t *testing.T) {
	app := newIntegrationApp(t)
	execRepositoryFixtureSQL(t, app, `CREATE TRIGGER fixture_job_insert_failure BEFORE INSERT ON background_jobs BEGIN SELECT RAISE(ABORT, 'fixture insert failure'); END`)
	requireRepositoryEqual(t, "queue failure sentinel", app.queueTrackedJob("fixture", "fixture", "1"), int64(0))
	requireRepositoryEqual(t, "start failure sentinel", app.startTrackedJob("fixture", "fixture", "1"), int64(0))
	execRepositoryFixtureSQL(t, app, `DROP TRIGGER fixture_job_insert_failure`)
	id := app.queueTrackedJob("fixture", "fixture", "1")
	execRepositoryFixtureSQL(t, app, `CREATE TRIGGER fixture_job_update_failure BEFORE UPDATE ON background_jobs BEGIN SELECT RAISE(ABORT, 'fixture update failure'); END`)
	app.markTrackedJobRunning(id)
	app.finishTrackedJob(id, nil)
	assertJobLifecycleState(t, app, id, [4]any{"queued", "", false, false})
	app.markTrackedJobRunning(0)
	app.finishTrackedJob(0, errors.New("fixture failure"))
}

func TestJobLifecycleRestartRecoveryIsIdempotent(t *testing.T) {
	app := newIntegrationApp(t)
	queued := app.queueTrackedJob("fixture", "fixture", "1")
	running := app.startTrackedJob("fixture", "fixture", "2")
	succeeded := app.startTrackedJob("fixture", "fixture", "3")
	app.finishTrackedJob(succeeded, nil)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/recovery-fixture")
	runID := app.startSourceSyncRun(sourceID)
	app.markExternalSourceStatus(sourceID, "syncing", "")
	app.recoverInterruptedJobs()
	assertJobLifecycleState(t, app, queued, [4]any{"failed", "interrupted by service restart", false, true})
	assertJobLifecycleState(t, app, running, [4]any{"failed", "interrupted by service restart", true, true})
	assertJobLifecycleState(t, app, succeeded, [4]any{"succeeded", "", true, true})
	var before, after string
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT finished_at FROM background_jobs WHERE id = ?`, queued).Scan(&before))
	app.recoverInterruptedJobs()
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT finished_at FROM background_jobs WHERE id = ?`, queued).Scan(&after))
	requireRepositoryEqual(t, "recovery retained completed timestamp", after, before)
	var runStatus, sourceStatus string
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT status FROM source_sync_runs WHERE id = ?`, runID).Scan(&runStatus))
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT import_status FROM external_subscription_sources WHERE id = ?`, sourceID).Scan(&sourceStatus))
	requireRepositoryEqual(t, "recovered sync run", runStatus, "failed")
	requireRepositoryEqual(t, "recovered source", sourceStatus, "error")
}
