package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/sources"
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
		if summary.skipTarget(target) {
			continue
		}

		waitGroup.Add(1)
		semaphore <- struct{}{}
		go func(item keymanagement.HealthCheckTarget) {
			defer waitGroup.Done()
			defer func() { <-semaphore }()
			results <- checkKeyHealthTarget(item, check, persist, &persistenceMu)
		}(target)
	}

	waitGroup.Wait()
	close(results)
	for result := range results {
		summary.record(result)
	}
	return summary
}

// skipTarget counts a target that is not probed at all and reports whether it
// was skipped.
func (s *keyHealthCheckJobSummary) skipTarget(target keymanagement.HealthCheckTarget) bool {
	targetStatus, _ := model.NormalizeKeyStatus(target.Status)
	if targetStatus != model.KeyStatusActive {
		s.SkippedDisabled++
		return true
	}
	if kind, _ := model.NormalizeKeyKind(target.Kind); kind == model.KeyKindInformational {
		s.SkippedUnsupported++
		return true
	}
	if target.Unreadable || strings.TrimSpace(target.URL) == "" {
		s.CheckFailed++
		return true
	}
	return false
}

// record tallies one probed target.
func (s *keyHealthCheckJobSummary) record(result keyHealthCheckItemResult) {
	s.Checked++
	if result.checkFailed {
		s.CheckFailed++
		return
	}
	if result.persistFailed {
		s.PersistFailed++
	} else {
		s.PersistedOK++
	}
	switch {
	case result.unsupported:
		s.SkippedUnsupported++
	case result.status == "up":
		s.Healthy++
	case result.status == "down":
		s.Unhealthy++
	default:
		s.CheckFailed++
	}
}

// checkKeyHealthTarget probes one key and persists the outcome. A panic in the
// checker or persister is reported as a failed check.
func checkKeyHealthTarget(
	item keymanagement.HealthCheckTarget,
	check keyHealthCheckExecutor,
	persist keyHealthResultPersister,
	persistenceMu *sync.Mutex,
) (result keyHealthCheckItemResult) {
	defer func() {
		if recover() != nil {
			result = keyHealthCheckItemResult{checkFailed: true}
		}
	}()

	status, detail, latency, err := check(context.Background(), item.URL)
	if err != nil {
		result.checkFailed = true
		return result
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
	return result
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

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
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

func (a *App) apiV1ListJobs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM background_jobs`).Scan(&total); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "jobs_list_failed", "failed to load jobs")
		return
	}
	rows, err := a.db.Query(`
		SELECT id, kind, status, target_type, target_id, error_message, result_counts_json, run_after, started_at, finished_at, created_at
		FROM background_jobs ORDER BY id DESC LIMIT ? OFFSET ?
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "jobs_list_failed", "failed to load jobs")
		return
	}
	defer rows.Close()
	items := []backgroundJob{}
	for rows.Next() {
		var item backgroundJob
		var resultCountsJSON string
		var runAfter, startedAt, finishedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Kind, &item.Status, &item.TargetType, &item.TargetID, &item.ErrorMessage, &resultCountsJSON, &runAfter, &startedAt, &finishedAt, &item.CreatedAt); err != nil {
			httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "jobs_list_failed", "failed to load jobs")
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
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": items, "meta": pageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}})
}

func (a *App) apiV1GetJob(w http.ResponseWriter, r *http.Request) {
	jobID, ok := httpapi.PathID(w, r, "id")
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
		httpapi.WriteV1Error(w, r, http.StatusNotFound, "job_not_found", "job not found")
		return
	}
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "job_load_failed", "failed to load job")
		return
	}
	item.RunAfter = nullTimePointer(runAfter)
	item.StartedAt = nullTimePointer(startedAt)
	item.FinishedAt = nullTimePointer(finishedAt)
	decodeKeyHealthCheckJobCounts(&item, resultCountsJSON)
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": item})
}

func (a *App) apiV1ListSourceSyncRuns(w http.ResponseWriter, r *http.Request) {
	sourceID, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	rows, err := a.db.Query(`
		SELECT id, status, imported_count, skipped_count, result_counts_json, error_message, started_at, finished_at
		FROM source_sync_runs WHERE source_id = ? ORDER BY id DESC LIMIT 50
	`, sourceID)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "sync_runs_list_failed", "failed to load sync history")
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
			httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "sync_runs_list_failed", "failed to load sync history")
			return
		}
		var resultCounts sources.Counts
		_ = json.Unmarshal([]byte(resultCountsJSON), &resultCounts)
		items = append(items, map[string]any{
			"id": id, "source_id": sourceID, "status": status, "imported_count": imported,
			"skipped_count": skipped, "result_counts": resultCounts, "error_message": errorMessage, "started_at": started,
			"finished_at": nullTimePointer(finished),
		})
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": items, "source_id": strconv.FormatInt(sourceID, 10)})
}

func (a *App) apiV1QueueSourceSync(w http.ResponseWriter, r *http.Request) {
	a.respondQueuedJob(w, r, a.prepareSourceSync, "failed to queue source synchronization", false)
}

func (a *App) queueKeyHealthCheck(r *http.Request) int64 {
	session, _, _ := a.adminSessionFromRequest(r)
	jobID, _ := a.enqueueAndDispatchJob(jobDispatchPlan{kind: "keys_health_check", targetType: "key", targetID: "all"}, session.AdminID, requestIDFromRequest(r))
	return jobID
}

func (a *App) apiV1RetryJob(w http.ResponseWriter, r *http.Request) {
	a.respondQueuedJob(w, r, a.prepareJobRetry, "failed to retry job", true)
}
