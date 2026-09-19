package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
)

func healthTarget(id int64, rawURL string) keymanagement.HealthCheckTarget {
	return keymanagement.HealthCheckTarget{ID: id, URL: rawURL, Status: "active", Kind: "real"}
}

func TestExecuteKeyHealthCheckBatchAggregatesOutcomes(t *testing.T) {
	targets := []keymanagement.HealthCheckTarget{
		healthTarget(1, "up"),
		healthTarget(2, "down"),
		{ID: 3, Status: "non-active", Kind: "real"},
		{ID: 4, Status: "active", Kind: "informational"},
		{ID: 5, Status: "active", Kind: "real", Unreadable: true},
	}
	summary := executeKeyHealthCheckBatch(targets, func(_ context.Context, rawURL string) (string, string, int64, error) {
		if rawURL == "down" {
			return "down", "connection refused", 0, nil
		}
		return "up", "", 2, nil
	}, func(_ int64, _ string, _ string, _ int64) error { return nil }, 2)

	want := keyHealthCheckJobSummary{
		TotalSelected: 5, Checked: 2, Healthy: 1, Unhealthy: 1, CheckFailed: 1,
		SkippedDisabled: 1, SkippedUnsupported: 1, PersistedOK: 2,
	}
	if summary != want {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
	if status := keyHealthCheckJobStatus(summary, nil); status != "succeeded_with_warnings" {
		t.Fatalf("status = %q, want succeeded_with_warnings", status)
	}
}

func TestExecuteKeyHealthCheckBatchPersistenceFailuresAreWarnings(t *testing.T) {
	targets := []keymanagement.HealthCheckTarget{healthTarget(1, "up"), healthTarget(2, "down")}
	summary := executeKeyHealthCheckBatch(targets, func(_ context.Context, rawURL string) (string, string, int64, error) {
		return rawURL, "", 5, nil
	}, func(keyID int64, _ string, _ string, _ int64) error {
		if keyID == 2 {
			return errors.New("write failed")
		}
		return nil
	}, 2)

	if summary.Healthy != 1 || summary.Unhealthy != 1 || summary.PersistedOK != 1 || summary.PersistFailed != 1 {
		t.Fatalf("unexpected partial persistence summary: %#v", summary)
	}
	if status := keyHealthCheckJobStatus(summary, nil); status != "succeeded_with_warnings" {
		t.Fatalf("status = %q, want succeeded_with_warnings", status)
	}
}

func TestExecuteKeyHealthCheckBatchSuccessfulPersistenceSucceeds(t *testing.T) {
	targets := []keymanagement.HealthCheckTarget{
		healthTarget(1, "up"),
		healthTarget(2, "connection-refused"),
		healthTarget(3, "tls-error"),
	}
	summary := executeKeyHealthCheckBatch(targets, func(_ context.Context, rawURL string) (string, string, int64, error) {
		if rawURL == "up" {
			return "up", "", 24, nil
		}
		return "down", rawURL, 0, nil
	}, func(_ int64, _ string, _ string, _ int64) error { return nil }, 2)

	if summary.Healthy != 1 || summary.Unhealthy != 2 || summary.PersistedOK != 3 || summary.PersistFailed != 0 {
		t.Fatalf("unexpected successfully persisted summary: %#v", summary)
	}
	if status := keyHealthCheckJobStatus(summary, nil); status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", status)
	}
}

func TestExecuteKeyHealthCheckBatchSerializesPersistence(t *testing.T) {
	targets := make([]keymanagement.HealthCheckTarget, 20)
	for index := range targets {
		targets[index] = healthTarget(int64(index+1), "up")
	}
	var activePersisters atomic.Int32
	summary := executeKeyHealthCheckBatch(targets, func(context.Context, string) (string, string, int64, error) {
		return "up", "", 3, nil
	}, func(_ int64, _ string, _ string, _ int64) error {
		inFlight := activePersisters.Add(1)
		defer activePersisters.Add(-1)
		if inFlight > 1 {
			return errors.New("simulated SQLite writer contention")
		}
		time.Sleep(time.Millisecond)
		return nil
	}, 10)

	if summary.PersistFailed != 0 || summary.PersistedOK != len(targets) {
		t.Fatalf("concurrent persistence caused false failures: %#v", summary)
	}
}

func TestExecuteKeyHealthCheckBatchCheckFailuresAreWarnings(t *testing.T) {
	targets := []keymanagement.HealthCheckTarget{healthTarget(1, "one"), healthTarget(2, "two")}
	var persistCalls atomic.Int32
	summary := executeKeyHealthCheckBatch(targets, func(context.Context, string) (string, string, int64, error) {
		return "", "", 0, errors.New("checker unavailable")
	}, func(_ int64, _ string, _ string, _ int64) error {
		persistCalls.Add(1)
		return nil
	}, 2)

	if summary.Checked != 2 || summary.CheckFailed != 2 || summary.PersistedOK != 0 || persistCalls.Load() != 0 {
		t.Fatalf("unexpected failed-check summary: %#v", summary)
	}
	if status := keyHealthCheckJobStatus(summary, nil); status != "succeeded_with_warnings" {
		t.Fatalf("status = %q, want succeeded_with_warnings", status)
	}
}

func TestExecuteKeyHealthCheckBatchDisabledTargetsAreSuccessfulSkips(t *testing.T) {
	summary := executeKeyHealthCheckBatch([]keymanagement.HealthCheckTarget{{
		ID: 1, Status: "non-active", Kind: "real",
	}}, func(context.Context, string) (string, string, int64, error) {
		t.Fatal("disabled target must not be checked")
		return "", "", 0, nil
	}, func(_ int64, _ string, _ string, _ int64) error {
		t.Fatal("disabled target must not be persisted")
		return nil
	}, 1)

	if summary.SkippedDisabled != 1 || summary.Checked != 0 {
		t.Fatalf("unexpected disabled summary: %#v", summary)
	}
	if status := keyHealthCheckJobStatus(summary, nil); status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", status)
	}
}

func TestExecuteKeyHealthCheckBatchPreservesUnsupportedProbeResult(t *testing.T) {
	var persistCalls atomic.Int32
	summary := executeKeyHealthCheckBatch([]keymanagement.HealthCheckTarget{healthTarget(1, "hysteria-v1")}, func(context.Context, string) (string, string, int64, error) {
		return "unsupported_check", "protocol_health_check_unsupported", 0, nil
	}, func(_ int64, status string, detail string, _ int64) error {
		persistCalls.Add(1)
		if status != "unsupported_check" || detail != "protocol_health_check_unsupported" {
			return errors.New("unsupported probe result changed")
		}
		return nil
	}, 1)

	if summary.Checked != 1 || summary.SkippedUnsupported != 1 || summary.PersistedOK != 1 || persistCalls.Load() != 1 {
		t.Fatalf("unexpected unsupported-probe summary: %#v", summary)
	}
	if status := keyHealthCheckJobStatus(summary, nil); status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", status)
	}
}

func TestKeyHealthCheckJobStatusFatalInitializationFailure(t *testing.T) {
	if status := keyHealthCheckJobStatus(keyHealthCheckJobSummary{}, errors.New("load failed")); status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
}

func TestExecuteKeyHealthCheckBatchPersistsTheStatesReportedByTheSummary(t *testing.T) {
	app := newIntegrationApp(t)
	service := app.keyService()
	upID, _, err := service.CreateLegacy(t.Context(), keymanagement.CreateLegacyParams{
		Label: "reachable", URL: "vless://11111111-1111-1111-1111-111111111111@up.example:443", Kind: "real", Status: "active",
	})
	if err != nil {
		t.Fatalf("create reachable profile: %v", err)
	}
	downID, _, err := service.CreateLegacy(t.Context(), keymanagement.CreateLegacyParams{
		Label: "unreachable", URL: "vless://22222222-2222-2222-2222-222222222222@down.example:443", Kind: "real", Status: "active",
	})
	if err != nil {
		t.Fatalf("create unreachable profile: %v", err)
	}
	targets, err := service.ListHealthCheckTargets(t.Context())
	if err != nil {
		t.Fatalf("list health targets: %v", err)
	}

	summary := executeKeyHealthCheckBatch(targets, func(_ context.Context, rawURL string) (string, string, int64, error) {
		if strings.Contains(rawURL, "up.example") {
			return "up", "", 18, nil
		}
		return "down", "connection refused", 0, nil
	}, func(keyID int64, status string, detail string, latency int64) error {
		return service.SaveHealthCheckResult(t.Context(), keyID, status, detail, latency)
	}, 10)

	if summary.Healthy != 1 || summary.Unhealthy != 1 || summary.PersistedOK != 2 || summary.PersistFailed != 0 {
		t.Fatalf("summary does not match persisted results: %#v", summary)
	}
	if status := keyHealthCheckJobStatus(summary, nil); status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", status)
	}
	upResult, err := service.GetHealthCheckResult(t.Context(), upID)
	if err != nil || upResult.Status != "up" || upResult.Latency != 18 || upResult.LastCheckedAt == nil {
		t.Fatalf("reachable card state = %#v, err=%v", upResult, err)
	}
	downResult, err := service.GetHealthCheckResult(t.Context(), downID)
	if err != nil || downResult.Status != "down" || downResult.Error != "connection refused" || downResult.LastCheckedAt == nil {
		t.Fatalf("unreachable card state = %#v, err=%v", downResult, err)
	}
}

func TestFinishKeyHealthCheckJobPersistsWarningStatusAndCounts(t *testing.T) {
	app := newIntegrationApp(t)
	db := app.db
	result, err := db.Exec(`INSERT INTO background_jobs(kind, status) VALUES('keys_health_check', 'running')`)
	if err != nil {
		t.Fatalf("insert job: %v", err)
	}
	jobID, _ := result.LastInsertId()
	summary := keyHealthCheckJobSummary{TotalSelected: 100, Checked: 100, PersistedOK: 79, PersistFailed: 21}
	app.finishKeyHealthCheckJob(jobID, summary, nil)

	var status, message, countsJSON string
	if err := db.QueryRow(`SELECT status, error_message, result_counts_json FROM background_jobs WHERE id = ?`, jobID).Scan(&status, &message, &countsJSON); err != nil {
		t.Fatalf("read job: %v", err)
	}
	var stored keyHealthCheckJobSummary
	if err := json.Unmarshal([]byte(countsJSON), &stored); err != nil {
		t.Fatalf("decode stored counts: %v", err)
	}
	if status != "succeeded_with_warnings" || message != "" || stored != summary {
		t.Fatalf("stored job status=%q message=%q counts=%#v", status, message, stored)
	}
}
