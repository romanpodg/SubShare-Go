package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
)

type backgroundJob struct {
	ID           int64      `json:"id"`
	Kind         string     `json:"kind"`
	Status       string     `json:"status"`
	TargetType   string     `json:"target_type"`
	TargetID     string     `json:"target_id"`
	ErrorMessage string     `json:"error_message"`
	RunAfter     *time.Time `json:"run_after"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

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

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func (a *App) startTrackedJob(kind, targetType, targetID string) int64 {
	result, err := a.db.Exec(`
		INSERT INTO background_jobs(kind, status, target_type, target_id, started_at)
		VALUES(?, 'running', ?, ?, CURRENT_TIMESTAMP)
	`, kind, targetType, targetID)
	if err != nil {
		return 0
	}
	id, _ := result.LastInsertId()
	return id
}

func (a *App) queueTrackedJob(kind, targetType, targetID string) int64 {
	result, err := a.db.Exec(`
		INSERT INTO background_jobs(kind, status, target_type, target_id, run_after)
		VALUES(?, 'queued', ?, ?, CURRENT_TIMESTAMP)
	`, kind, targetType, targetID)
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

func (a *App) startSourceSyncRun(sourceID int64) int64 {
	result, err := a.db.Exec(`INSERT INTO source_sync_runs(source_id, status) VALUES(?, 'running')`, sourceID)
	if err != nil {
		return 0
	}
	id, _ := result.LastInsertId()
	return id
}

func (a *App) finishSourceSyncRun(id int64, result externalSyncResult, err error) {
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

func marshalExternalCounts(counts externalImportCounts) string {
	payload, err := json.Marshal(counts)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func (a *App) apiV1ListJobs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM background_jobs`).Scan(&total); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "jobs_list_failed", "failed to load jobs")
		return
	}
	rows, err := a.db.Query(`
		SELECT id, kind, status, target_type, target_id, error_message, run_after, started_at, finished_at, created_at
		FROM background_jobs ORDER BY id DESC LIMIT ? OFFSET ?
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "jobs_list_failed", "failed to load jobs")
		return
	}
	defer rows.Close()
	items := []backgroundJob{}
	for rows.Next() {
		var item backgroundJob
		var runAfter, startedAt, finishedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Kind, &item.Status, &item.TargetType, &item.TargetID, &item.ErrorMessage, &runAfter, &startedAt, &finishedAt, &item.CreatedAt); err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "jobs_list_failed", "failed to load jobs")
			return
		}
		item.RunAfter = nullTimePointer(runAfter)
		item.StartedAt = nullTimePointer(startedAt)
		item.FinishedAt = nullTimePointer(finishedAt)
		items = append(items, item)
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "meta": pageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}})
}

func (a *App) apiV1GetJob(w http.ResponseWriter, r *http.Request) {
	jobID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var item backgroundJob
	var runAfter, startedAt, finishedAt sql.NullTime
	err := a.db.QueryRow(`
		SELECT id, kind, status, target_type, target_id, error_message,
		       run_after, started_at, finished_at, created_at
		FROM background_jobs
		WHERE id = ?
	`, jobID).Scan(
		&item.ID, &item.Kind, &item.Status, &item.TargetType, &item.TargetID,
		&item.ErrorMessage, &runAfter, &startedAt, &finishedAt, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "job_load_failed", "failed to load job")
		return
	}
	item.RunAfter = nullTimePointer(runAfter)
	item.StartedAt = nullTimePointer(startedAt)
	item.FinishedAt = nullTimePointer(finishedAt)
	writeJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (a *App) apiV1ListSourceSyncRuns(w http.ResponseWriter, r *http.Request) {
	sourceID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	rows, err := a.db.Query(`
		SELECT id, status, imported_count, skipped_count, result_counts_json, error_message, started_at, finished_at
		FROM source_sync_runs WHERE source_id = ? ORDER BY id DESC LIMIT 50
	`, sourceID)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "sync_runs_list_failed", "failed to load sync history")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var status, resultCountsJSON, errorMessage string
		var imported, skipped int
		var started time.Time
		var finished sql.NullTime
		if err := rows.Scan(&id, &status, &imported, &skipped, &resultCountsJSON, &errorMessage, &started, &finished); err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "sync_runs_list_failed", "failed to load sync history")
			return
		}
		var resultCounts externalImportCounts
		_ = json.Unmarshal([]byte(resultCountsJSON), &resultCounts)
		items = append(items, map[string]any{
			"id": id, "source_id": sourceID, "status": status, "imported_count": imported,
			"skipped_count": skipped, "result_counts": resultCounts, "error_message": errorMessage, "started_at": started,
			"finished_at": nullTimePointer(finished),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "source_id": strconv.FormatInt(sourceID, 10)})
}

func (a *App) apiV1QueueSourceSync(w http.ResponseWriter, r *http.Request) {
	sourceID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := a.getExternalSourceByID(sourceID); errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "source_not_found", "source not found")
		return
	} else if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_load_failed", "failed to load source")
		return
	}
	session, _, _ := a.adminSessionFromRequest(r)
	requestID := requestIDFromRequest(r)
	jobID := a.queueTrackedJob("source_sync", "external_source", strconv.FormatInt(sourceID, 10))
	if jobID == 0 {
		writeV1Error(w, r, http.StatusInternalServerError, "job_queue_failed", "failed to queue source synchronization")
		return
	}
	go a.runQueuedSourceSync(jobID, sourceID, session.AdminID, requestID)
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
}

func (a *App) apiV1QueueKeyHealthCheck(w http.ResponseWriter, r *http.Request) {
	a.keyAdministrationHTTPHandler().QueueHealthCheck(w, r)
}

func (a *App) queueKeyHealthCheck(r *http.Request) int64 {
	session, _, _ := a.adminSessionFromRequest(r)
	jobID := a.queueTrackedJob("keys_health_check", "key", "all")
	if jobID == 0 {
		return 0
	}
	go a.runQueuedKeyHealthCheck(jobID, session.AdminID, requestIDFromRequest(r))
	return jobID
}

func (a *App) runQueuedKeyHealthCheck(jobID, actorAdminID int64, requestID string) {
	a.markTrackedJobRunning(jobID)
	targets, err := a.keyService().ListHealthCheckTargets(context.Background())
	if err != nil {
		a.finishTrackedJob(jobID, err)
		return
	}

	var waitGroup sync.WaitGroup
	semaphore := make(chan struct{}, 10)
	errorCount := 0
	var errorLock sync.Mutex
	for _, target := range targets {
		waitGroup.Add(1)
		semaphore <- struct{}{}
		go func(item keymanagement.HealthCheckTarget) {
			defer waitGroup.Done()
			defer func() { <-semaphore }()
			if checkErr := a.checkAndPersistKey(item.ID, item.URL); checkErr != nil {
				log.Printf("background key check: key_id=%d err=%v", item.ID, checkErr)
				errorLock.Lock()
				errorCount++
				errorLock.Unlock()
			}
		}(target)
	}
	waitGroup.Wait()
	if errorCount > 0 {
		err = fmt.Errorf("%d key checks could not be persisted", errorCount)
		a.finishTrackedJob(jobID, err)
		return
	}
	a.finishTrackedJob(jobID, nil)
	a.recordAuditEventForActor(actorAdminID, requestID, "keys.health_check", "key", "all", map[string]any{
		"checked": len(targets),
		"job_id":  jobID,
	})
}

func (a *App) runQueuedSourceSync(jobID, sourceID, actorAdminID int64, requestID string) {
	a.markTrackedJobRunning(jobID)
	runID := a.startSourceSyncRun(sourceID)
	source, err := a.getExternalSourceByID(sourceID)
	if err != nil {
		a.finishTrackedJob(jobID, err)
		a.finishSourceSyncRun(runID, externalSyncResult{}, err)
		return
	}
	a.markExternalSourceStatus(sourceID, "syncing", "")
	hwidProfile := normalizeExternalHWIDProfile(source.PassHWID, source.HWIDVersion, source.HWIDModelName, source.HWIDValue)
	parsed, err := fetchExternalSubscription(source.SourceURL, hwidProfile, a.externalProfileFingerprintKeys())
	if err != nil {
		a.markExternalSourceStatus(sourceID, "error", err.Error())
		a.finishTrackedJob(jobID, err)
		a.finishSourceSyncRun(runID, externalSyncResult{}, err)
		return
	}
	syncResult, err := a.syncExternalSource(sourceID, parsed)
	if err != nil {
		a.markExternalSourceStatus(sourceID, "error", err.Error())
		a.finishTrackedJob(jobID, err)
		a.finishSourceSyncRun(runID, syncResult, err)
		return
	}
	a.finishTrackedJob(jobID, nil)
	a.finishSourceSyncRun(runID, syncResult, nil)
	a.recordAuditEventForActor(actorAdminID, requestID, "external_source.sync", "external_source", strconv.FormatInt(sourceID, 10), map[string]any{
		"imported_count": syncResult.Imported,
		"skipped_count":  syncResult.Skipped,
		"result_counts":  syncResult.Counts,
		"job_id":         jobID,
	})
}

func (a *App) apiV1RetryJob(w http.ResponseWriter, r *http.Request) {
	jobID, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var kind, status, targetID string
	err := a.db.QueryRow(`SELECT kind, status, target_id FROM background_jobs WHERE id = ?`, jobID).Scan(&kind, &status, &targetID)
	if errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "job_load_failed", "failed to load job")
		return
	}
	if status != "failed" {
		writeV1Error(w, r, http.StatusConflict, "job_not_failed", "only failed jobs can be retried")
		return
	}
	targetType := ""
	var sourceID int64
	switch kind {
	case "source_sync":
		targetType = "external_source"
		sourceID, err = strconv.ParseInt(strings.TrimSpace(targetID), 10, 64)
		if err != nil || sourceID < 1 {
			writeV1Error(w, r, http.StatusConflict, "job_target_invalid", "job target is invalid")
			return
		}
	case "keys_health_check":
		targetType = "key"
	default:
		writeV1Error(w, r, http.StatusConflict, "job_not_retryable", "this job type is not retryable")
		return
	}
	session, _, _ := a.adminSessionFromRequest(r)
	newJobID := a.queueTrackedJob(kind, targetType, targetID)
	if newJobID == 0 {
		writeV1Error(w, r, http.StatusInternalServerError, "job_queue_failed", "failed to retry job")
		return
	}
	switch kind {
	case "source_sync":
		go a.runQueuedSourceSync(newJobID, sourceID, session.AdminID, requestIDFromRequest(r))
	case "keys_health_check":
		go a.runQueuedKeyHealthCheck(newJobID, session.AdminID, requestIDFromRequest(r))
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": newJobID, "retry_of": jobID, "status": "queued"})
}
