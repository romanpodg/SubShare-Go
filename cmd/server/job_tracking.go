package main

import (
	"encoding/json"
	"github.com/romanpodg/SubShare-Go/internal/sources"
)

// Tracking writes intentionally retain their existing best-effort classification.
func (a *App) recoverInterruptedJobs() {
	_, _ = a.db.Exec(`
		UPDATE background_jobs
		SET status = 'failed',
		    error_message = 'interrupted by service restart',
		    finished_at = CURRENT_TIMESTAMP
		WHERE status IN ('queued', 'running')
	`)
	_, _ = a.db.Exec(`
		UPDATE source_sync_runs
		SET status = 'failed',
		    error_message = 'interrupted by service restart',
		    finished_at = CURRENT_TIMESTAMP
		WHERE status = 'running'
	`)
	_, _ = a.db.Exec(`
		UPDATE external_subscription_sources
		SET import_status = 'error',
		    last_error = 'synchronization interrupted by service restart',
		    updated_at = CURRENT_TIMESTAMP
		WHERE import_status = 'syncing'
	`)
}

func (a *App) startTrackedJob(kind, targetType, targetID string) int64 {
	return a.insertTrackedJob(`
		INSERT INTO background_jobs(kind, status, target_type, target_id, started_at)
		VALUES(?, 'running', ?, ?, CURRENT_TIMESTAMP)
	`, jobDispatchPlan{kind: kind, targetType: targetType, targetID: targetID})
}

func (a *App) queueTrackedJob(kind, targetType, targetID string) int64 {
	return a.insertTrackedJob(`
		INSERT INTO background_jobs(kind, status, target_type, target_id, run_after)
		VALUES(?, 'queued', ?, ?, CURRENT_TIMESTAMP)
	`, jobDispatchPlan{kind: kind, targetType: targetType, targetID: targetID})
}

func (a *App) insertTrackedJob(query string, target jobDispatchPlan) int64 {
	result, err := a.db.Exec(query, target.kind, target.targetType, target.targetID)
	if err != nil {
		return 0
	}
	id, _ := result.LastInsertId()
	return id
}

func (a *App) markTrackedJobRunning(id int64) {
	if id == 0 {
		return
	}
	_, _ = a.db.Exec(`
		UPDATE background_jobs
		SET status = 'running', started_at = CURRENT_TIMESTAMP, error_message = ''
		WHERE id = ? AND status = 'queued'
	`, id)
}

func (a *App) finishTrackedJob(id int64, err error) {
	if id == 0 {
		return
	}
	if err != nil {
		_, _ = a.db.Exec(`UPDATE background_jobs SET status = 'failed', error_message = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`, err.Error(), id)
		return
	}
	_, _ = a.db.Exec(`UPDATE background_jobs SET status = 'succeeded', error_message = '', finished_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
}

func (a *App) finishKeyHealthCheckJob(id int64, summary keyHealthCheckJobSummary, fatalErr error) {
	if id == 0 {
		return
	}
	countsJSON, err := json.Marshal(summary)
	if err != nil {
		countsJSON = []byte("{}")
	}
	status := keyHealthCheckJobStatus(summary, fatalErr)
	errorMessage := ""
	if fatalErr != nil {
		errorMessage = fatalErr.Error()
	}
	_, _ = a.db.Exec(`
		UPDATE background_jobs
		SET status = ?, error_message = ?, result_counts_json = ?, finished_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, errorMessage, string(countsJSON), id)
}

func (a *App) startSourceSyncRun(sourceID int64) int64 {
	result, err := a.db.Exec(`INSERT INTO source_sync_runs(source_id, status) VALUES(?, 'running')`, sourceID)
	if err != nil {
		return 0
	}
	id, _ := result.LastInsertId()
	return id
}

func (a *App) finishSourceSyncRun(id int64, result sources.SyncResult, err error) {
	if id == 0 {
		return
	}
	if err != nil {
		_, _ = a.db.Exec(`UPDATE source_sync_runs SET status = 'failed', error_message = ?, result_counts_json = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`, err.Error(), marshalExternalCounts(result.Counts), id)
		return
	}
	_, _ = a.db.Exec(`
		UPDATE source_sync_runs SET status = 'succeeded', imported_count = ?, skipped_count = ?,
		       result_counts_json = ?, error_message = '', finished_at = CURRENT_TIMESTAMP WHERE id = ?
	`, result.Imported, result.Skipped, marshalExternalCounts(result.Counts), id)
}

func marshalExternalCounts(counts sources.Counts) string {
	payload, err := json.Marshal(counts)
	if err != nil {
		return "{}"
	}
	return string(payload)
}
