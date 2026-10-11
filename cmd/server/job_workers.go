package main

import (
	"context"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"strconv"
)

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
		a.finishSourceSyncRun(runID, sources.SyncResult{}, err)
		return
	}
	lease := a.beginSourceFetch(source)
	defer a.releaseSourceFetch(lease)
	a.markSourceFetchStatus(lease, "syncing", "")
	hwidProfile := sources.NormalizeHWIDProfile(source.PassHWID, source.HWIDVersion, source.HWIDModelName, source.HWIDValue)
	parsed, err := sources.Fetch(context.Background(), a.sourceClient(), source.SourceURL, hwidProfile, a.externalProfileFingerprintKeys())
	if err != nil {
		a.markSourceFetchStatus(lease, "error", err.Error())
		a.finishTrackedJob(jobID, err)
		a.finishSourceSyncRun(runID, sources.SyncResult{}, err)
		return
	}
	syncResult, err := a.syncOwnedSourceResult(lease, parsed)
	if err != nil {
		a.markSourceFetchStatus(lease, "error", err.Error())
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
