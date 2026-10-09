package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/sources"
)

func TestJobSourceFetchBoundsAndStatusContracts(t *testing.T) {
	cases := []struct {
		name, body, message string
		status              int
	}{
		{"empty", " \n", "subscription body is empty", http.StatusOK},
		{"unsupported", "not a profile", "", http.StatusOK},
		{"http status", externalTestVLESS, "source returned HTTP 503", http.StatusServiceUnavailable},
		{"too large", strings.Repeat("x", sources.MaxBodyBytes+1), "source response is too large (max 10 MB)", http.StatusOK},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f, sourceID, fetch := newJobSourceFixture(t)
			fetch.body, fetch.status = test.body, test.status
			jobID, done := startControlledSourceJob(t, f, sourceID)
			waitJobFetch(t, fetch)
			close(fetch.release)
			waitControlledSourceJob(t, done)
			status, message, _, _ := jobLifecycleState(t, f.app, jobID)
			requireRepositoryEqual(t, "fetch failure job state", status, "failed")
			if test.message != "" {
				requireRepositoryEqual(t, "fetch failure classification", message, test.message)
			}
			requireRepositoryEqual(t, "fetch rejection creates no profile", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys`), 0)
		})
	}
}

func TestJobSourceClientTimeoutFailsWithoutURLSecrets(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	f.app.sourceHTTPClient.Timeout = 50 * time.Millisecond
	execRepositoryFixtureSQL(t, f.app, `UPDATE external_subscription_sources SET source_url = 'https://provider.example/feed?token=fixture-sensitive' WHERE id = ?`, sourceID)
	jobID, done := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, fetch)
	waitControlledSourceJob(t, done)
	status, message, _, _ := jobLifecycleState(t, f.app, jobID)
	requireRepositoryEqual(t, "timeout terminal state", status, "failed")
	requireRepositoryEqual(t, "timeout safe message", message, "failed to fetch source")
}

func TestJobSourceFetchCancellationKeepsSafeError(t *testing.T) {
	_, _, fetch := newJobSourceFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &http.Client{Transport: fetch}
	_, err := sources.Fetch(ctx, client, "https://provider.example/feed?token=fixture-sensitive", sources.HWIDProfile{}, [][]byte{externalTestFingerprintKey})
	if err == nil {
		t.Fatal("cancelled fetch succeeded")
	}
	requireRepositoryEqual(t, "cancelled fetch safe error", err.Error(), "failed to fetch source")
}
