package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

const keyHealthCheckConcurrency = 10

type keyHealthCheckJobSummary struct {
	TotalSelected      int `json:"total_selected"`
	Checked            int `json:"checked"`
	Healthy            int `json:"healthy"`
	Unhealthy          int `json:"unhealthy"`
	CheckFailed        int `json:"check_failed"`
	SkippedDisabled    int `json:"skipped_disabled"`
	SkippedUnsupported int `json:"skipped_unsupported"`
	PersistedOK        int `json:"persisted_ok"`
	PersistFailed      int `json:"persist_failed"`
}

type backgroundJob struct {
	ID           int64                     `json:"id"`
	Kind         string                    `json:"kind"`
	Status       string                    `json:"status"`
	TargetType   string                    `json:"target_type"`
	TargetID     string                    `json:"target_id"`
	ErrorMessage string                    `json:"error_message"`
	ResultCounts *keyHealthCheckJobSummary `json:"result_counts,omitempty"`
	RunAfter     *time.Time                `json:"run_after"`
	StartedAt    *time.Time                `json:"started_at"`
	FinishedAt   *time.Time                `json:"finished_at"`
	CreatedAt    time.Time                 `json:"created_at"`
}

type keyHealthCheckExecutor func(ctx context.Context, rawURL string) (status string, detail string, latency int64, err error)
type keyHealthResultPersister func(keyID int64, status string, detail string, latency int64) error

type keyHealthCheckItemResult struct {
	status        string
	checkFailed   bool
	unsupported   bool
	persistFailed bool
}

func executeKeyHealthCheckBatch(
	targets []keymanagement.HealthCheckTarget,
	check keyHealthCheckExecutor,
	persist keyHealthResultPersister,
	concurrency int,
) keyHealthCheckJobSummary {
	if concurrency < 1 {
		concurrency = 1
	}
	summary := keyHealthCheckJobSummary{TotalSelected: len(targets)}
	results := make(chan keyHealthCheckItemResult, len(targets))
	semaphore := make(chan struct{}, concurrency)
	var waitGroup sync.WaitGroup
	var persistenceMu sync.Mutex

	for _, target := range targets {
		targetStatus, _ := model.NormalizeKeyStatus(target.Status)
		if targetStatus != model.KeyStatusActive {
			summary.SkippedDisabled++
			continue
		}
		if kind, _ := model.NormalizeKeyKind(target.Kind); kind == model.KeyKindInformational {
			summary.SkippedUnsupported++
			continue
		}
		if target.Unreadable || strings.TrimSpace(target.URL) == "" {
			summary.CheckFailed++
			continue
		}

		waitGroup.Add(1)
		semaphore <- struct{}{}
		go func(item keymanagement.HealthCheckTarget) {
			defer waitGroup.Done()
			defer func() { <-semaphore }()
			result := keyHealthCheckItemResult{}
			defer func() {
				if recover() != nil {
					result = keyHealthCheckItemResult{checkFailed: true}
				}
				results <- result
			}()

			status, detail, latency, err := check(context.Background(), item.URL)
			if err != nil {
				result.checkFailed = true
				return
			}
			result.status = model.NormalizeCheckStatus(status)
			if result.status == "unsupported_check" {
				result.unsupported = true
			}
			// SQLite has one writer. Keep probes concurrent, but serialize their
			// short metadata updates so the batch does not contend with itself.
			persistenceMu.Lock()
			persistErr := persist(item.ID, result.status, detail, latency)
			persistenceMu.Unlock()
			if persistErr != nil {
				log.Printf(
					"background key check result persistence failed: key_id=%d health_state=%s latency_ms=%d error_type=%T err=%v",
					item.ID,
					result.status,
					latency,
					persistErr,
					persistErr,
				)
				result.persistFailed = true
			}
		}(target)
	}

	waitGroup.Wait()
	close(results)
	for result := range results {
		summary.Checked++
		if result.checkFailed {
			summary.CheckFailed++
			continue
		}
		if result.persistFailed {
			summary.PersistFailed++
		} else {
			summary.PersistedOK++
		}
		if result.unsupported {
			summary.SkippedUnsupported++
		} else if result.status == "up" {
			summary.Healthy++
		} else if result.status == "down" {
			summary.Unhealthy++
		} else {
			summary.CheckFailed++
		}
	}
	return summary
}

func keyHealthCheckJobStatus(summary keyHealthCheckJobSummary, fatalErr error) string {
	if fatalErr != nil {
		return "failed"
	}
	if summary.CheckFailed > 0 || summary.PersistFailed > 0 {
		return "succeeded_with_warnings"
	}
	return "succeeded"
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

func decodeKeyHealthCheckJobCounts(item *backgroundJob, raw string) {
	if item == nil || item.Kind != "keys_health_check" || strings.TrimSpace(raw) == "" {
		return
	}
	var counts keyHealthCheckJobSummary
	if json.Unmarshal([]byte(raw), &counts) == nil {
		item.ResultCounts = &counts
	}
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
		SELECT id, kind, status, target_type, target_id, error_message, result_counts_json, run_after, started_at, finished_at, created_at
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
		var resultCountsJSON string
		var runAfter, startedAt, finishedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Kind, &item.Status, &item.TargetType, &item.TargetID, &item.ErrorMessage, &resultCountsJSON, &runAfter, &startedAt, &finishedAt, &item.CreatedAt); err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "jobs_list_failed", "failed to load jobs")
			return
		}
		item.RunAfter = nullTimePointer(runAfter)
		item.StartedAt = nullTimePointer(startedAt)
		item.FinishedAt = nullTimePointer(finishedAt)
		decodeKeyHealthCheckJobCounts(&item, resultCountsJSON)
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
	var resultCountsJSON string
	var runAfter, startedAt, finishedAt sql.NullTime
	err := a.db.QueryRow(`
		SELECT id, kind, status, target_type, target_id, error_message, result_counts_json,
		       run_after, started_at, finished_at, created_at
		FROM background_jobs
		WHERE id = ?
	`, jobID).Scan(
		&item.ID, &item.Kind, &item.Status, &item.TargetType, &item.TargetID,
		&item.ErrorMessage, &resultCountsJSON, &runAfter, &startedAt, &finishedAt, &item.CreatedAt,
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
	decodeKeyHealthCheckJobCounts(&item, resultCountsJSON)
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
		a.finishKeyHealthCheckJob(jobID, keyHealthCheckJobSummary{}, err)
		return
	}
	summary := executeKeyHealthCheckBatch(targets, func(ctx context.Context, rawURL string) (string, string, int64, error) {
		status, detail, latency := checkConfigurationAvailabilityContext(ctx, rawURL)
		return status, detail, latency, nil
	}, func(keyID int64, status string, detail string, latency int64) error {
		return a.keyService().SaveHealthCheckResult(context.Background(), keyID, status, detail, latency)
	}, keyHealthCheckConcurrency)
	a.finishKeyHealthCheckJob(jobID, summary, nil)
	a.recordAuditEventForActor(actorAdminID, requestID, "keys.health_check", "key", "all", map[string]any{
		"job_id":        jobID,
		"result_counts": summary,
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
