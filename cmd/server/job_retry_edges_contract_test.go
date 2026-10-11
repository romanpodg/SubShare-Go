package main

import (
	"fmt"
	"net/http"
	"testing"
)

func TestJobRetryAdditionalEligibilityContracts(t *testing.T) {
	cases := []struct{ name, kind, status, target, code string }{
		{"warnings", "keys_health_check", "succeeded_with_warnings", "all", "job_not_failed"},
		{"empty source target", "source_sync", "failed", "", "job_target_invalid"},
		{"negative source target", "source_sync", "failed", "-1", "job_target_invalid"},
		{"overflow source target", "source_sync", "failed", "9223372036854775808", "job_target_invalid"},
		{"hex source target", "source_sync", "failed", "0x1", "job_target_invalid"},
		{"status precedes kind", "fixture", "running", "invalid", "job_not_failed"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := f.app.queueTrackedJob(test.kind, "fixture", test.target)
			execRepositoryFixtureSQL(t, f.app, `UPDATE background_jobs SET status = ? WHERE id = ?`, test.status, id)
			recorder := jobRetryRequest(f, id)
			requireRepositoryEqual(t, "retry rejected status", recorder.Code, http.StatusConflict)
			requireRepositoryEqual(t, "retry rejected code", decodeJSONMap(t, recorder)["code"], test.code)
			requireRepositoryEqual(t, "rejection retains original only", mutationCount(t, f.app, `SELECT COUNT(*) FROM background_jobs`), 1)
		})
	}
}

func TestJobRetrySourceTargetParsingRetainsPersistedText(t *testing.T) {
	for _, format := range []string{"  %d\t", "+%d", "00%d"} {
		t.Run(format, func(t *testing.T) {
			f, sourceID, fetch := newJobSourceFixture(t)
			target := fmt.Sprintf(format, sourceID)
			original := f.app.queueTrackedJob("source_sync", "external_source", target)
			execRepositoryFixtureSQL(t, f.app, `UPDATE background_jobs SET status = 'failed' WHERE id = ?`, original)
			recorder := jobRetryRequest(f, original)
			requireRepositoryEqual(t, "parsed source retry accepted", recorder.Code, http.StatusAccepted)
			payload := decodeJSONMap(t, recorder)
			requireRepositoryEqual(t, "retry envelope", len(payload), 3)
			requireRepositoryEqual(t, "retry original identity", payload["retry_of"], float64(original))
			requireRepositoryEqual(t, "retry accepted state", payload["status"], "queued")
			id := int64(payload["job_id"].(float64))
			waitJobFetch(t, fetch)
			close(fetch.release)
			requireRepositoryEqual(t, "parsed source completes", waitJobSourceTerminal(t, f.app, id), "succeeded")
			var targetType, storedTarget string
			requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT target_type,target_id FROM background_jobs WHERE id = ?`, id).Scan(&targetType, &storedTarget))
			requireRepositoryEqual(t, "source target type", targetType, "external_source")
			requireRepositoryEqual(t, "source text preserved", storedTarget, target)
			originalStatus, _, _, _ := jobLifecycleState(t, f.app, original)
			requireRepositoryEqual(t, "original remains retryable", originalStatus, "failed")
		})
	}
}
