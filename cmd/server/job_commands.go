package main

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
)

type jobCommandError string

func (e jobCommandError) Error() string { return string(e) }

const (
	errJobNotFound       jobCommandError = "job_not_found"
	errJobLoad           jobCommandError = "job_load_failed"
	errJobNotFailed      jobCommandError = "job_not_failed"
	errJobTargetInvalid  jobCommandError = "job_target_invalid"
	errJobNotRetryable   jobCommandError = "job_not_retryable"
	errJobQueue          jobCommandError = "job_queue_failed"
	errJobSourceNotFound jobCommandError = "source_not_found"
	errJobSourceLoad     jobCommandError = "source_load_failed"
)

type jobRetryRecord struct{ kind, status, targetID string }
type jobDispatchPlan struct {
	kind, targetType, targetID string
	sourceID                   int64
}

// Retry keeps the original target text and never changes the failed original.
func planFailedJobRetry(record jobRetryRecord) (jobDispatchPlan, error) {
	if record.status != "failed" {
		return jobDispatchPlan{}, errJobNotFailed
	}
	plan := jobDispatchPlan{kind: record.kind, targetID: record.targetID}
	switch record.kind {
	case "source_sync":
		plan.targetType = "external_source"
		var err error
		plan.sourceID, err = strconv.ParseInt(strings.TrimSpace(record.targetID), 10, 64)
		if err != nil || plan.sourceID < 1 {
			return jobDispatchPlan{}, errJobTargetInvalid
		}
	case "keys_health_check":
		plan.targetType = "key"
	default:
		return jobDispatchPlan{}, errJobNotRetryable
	}
	return plan, nil
}

func (a *App) prepareJobRetry(jobID int64) (jobDispatchPlan, error) {
	var record jobRetryRecord
	err := a.db.QueryRow(`SELECT kind, status, target_id FROM background_jobs WHERE id = ?`, jobID).Scan(&record.kind, &record.status, &record.targetID)
	if errors.Is(err, sql.ErrNoRows) {
		return jobDispatchPlan{}, errJobNotFound
	}
	if err != nil {
		return jobDispatchPlan{}, errJobLoad
	}
	return planFailedJobRetry(record)
}

func (a *App) prepareSourceSync(sourceID int64) (jobDispatchPlan, error) {
	if _, err := a.getExternalSourceByID(sourceID); errors.Is(err, sql.ErrNoRows) {
		return jobDispatchPlan{}, errJobSourceNotFound
	} else if err != nil {
		return jobDispatchPlan{}, errJobSourceLoad
	}
	plan := jobDispatchPlan{"source_sync", "external_source", strconv.FormatInt(sourceID, 10), sourceID}
	return plan, nil
}

// Persist queued before launching the same detached workers. Tracking failures,
// reconciliation ownership and health-check concurrency keep their prior policy.
func (a *App) enqueueAndDispatchJob(plan jobDispatchPlan, actorAdminID int64, requestID string) (int64, error) {
	id := a.queueTrackedJob(plan.kind, plan.targetType, plan.targetID)
	if id == 0 {
		return 0, errJobQueue
	}
	switch plan.kind {
	case "source_sync":
		go a.runQueuedSourceSync(id, plan.sourceID, actorAdminID, requestID)
	case "keys_health_check":
		go a.runQueuedKeyHealthCheck(id, actorAdminID, requestID)
	}
	return id, nil
}
