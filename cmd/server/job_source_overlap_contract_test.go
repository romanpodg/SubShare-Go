package main

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

type jobOverlapTransport struct {
	next    atomic.Int32
	fetches []*jobFetchTransport
}

func (fetch *jobOverlapTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return fetch.fetches[int(fetch.next.Add(1))-1].RoundTrip(request)
}

func assertSourceProfileRaw(t *testing.T, app *App, sourceID int64, want string) {
	t.Helper()
	var id int64
	var envelope string
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT k.id, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON s.vless_key_id = k.id WHERE k.external_source_id = ?`, sourceID).Scan(&id, &envelope))
	secret, err := profilestorage.Decrypt(envelope, app.profileKeyring, id)
	requireRepositorySuccess(t, err)
	if secret.Reveal() != want {
		t.Fatal("source profile differs from the expected captured feed")
	}
}

func TestJobSourceOverlapKeepsNewestFetchResult(t *testing.T) {
	f, sourceID, first := newJobSourceFixture(t)
	first.body = strings.Replace(externalTestVLESS, "vless.example", "first.example", 1)
	second := &jobFetchTransport{make(chan *http.Request, 1), make(chan struct{}), strings.Replace(externalTestVLESS, "vless.example", "second.example", 1), http.StatusOK}
	f.app.sourceHTTPClient.Transport = &jobOverlapTransport{fetches: []*jobFetchTransport{first, second}}
	firstJobID, firstDone := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, first)
	_, secondDone := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, second)
	close(second.release)
	waitControlledSourceJob(t, secondDone)
	assertSourceProfileRaw(t, f.app, sourceID, second.body)
	close(first.release)
	waitControlledSourceJob(t, firstDone)
	assertSourceProfileRaw(t, f.app, sourceID, second.body)
	status, _, _, _ := jobLifecycleState(t, f.app, firstJobID)
	requireRepositoryEqual(t, "superseded fetch job fails safely", status, "failed")
	assertJobSourceStatus(t, f.app, sourceID, "ok")
	requireRepositoryEqual(t, "overlap retains one source profile", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID), 1)
}

func TestJobSourceURLChangeDuringFetchWithholdsCapturedBody(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	jobID, done := startControlledSourceJob(t, f, sourceID)
	request := waitJobFetch(t, fetch)
	execRepositoryFixtureSQL(t, f.app, `UPDATE external_subscription_sources SET source_url = 'https://provider.example/new-feed' WHERE id = ?`, sourceID)
	close(fetch.release)
	waitControlledSourceJob(t, done)
	status, _, _, _ := jobLifecycleState(t, f.app, jobID)
	requireRepositoryEqual(t, "changed URL rejects old fetch", status, "failed")
	requireRepositoryEqual(t, "fetch used old URL", request.URL.String(), "https://provider.example/job-fixture")
	requireRepositoryEqual(t, "old URL creates no current profile", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID), 0)
	var currentURL string
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT source_url FROM external_subscription_sources WHERE id = ?`, sourceID).Scan(&currentURL))
	requireRepositoryEqual(t, "source keeps new URL", currentURL, "https://provider.example/new-feed")
}

func TestJobSourceMetadataChangesDuringFetchUseCurrentTarget(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	_, done := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, fetch)
	execRepositoryFixtureSQL(t, f.app, `UPDATE external_subscription_sources SET enabled = 0, key_category = 'Changed category' WHERE id = ?`, sourceID)
	close(fetch.release)
	waitControlledSourceJob(t, done)
	var category, status string
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT category, status FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&category, &status))
	requireRepositoryEqual(t, "current source category applied", category, "Changed category")
	requireRepositoryEqual(t, "current source disabled state applied", status, "non-active")
}

func TestJobSourceDeletionDuringFetchFailsWithoutRecreatingProfiles(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	jobID, done := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, fetch)
	execRepositoryFixtureSQL(t, f.app, `DELETE FROM external_subscription_sources WHERE id = ?`, sourceID)
	close(fetch.release)
	waitControlledSourceJob(t, done)
	status, _, _, _ := jobLifecycleState(t, f.app, jobID)
	requireRepositoryEqual(t, "deleted source job fails", status, "failed")
	requireRepositoryEqual(t, "deleted source is not recreated", mutationCount(t, f.app, `SELECT COUNT(*) FROM external_subscription_sources`), 0)
	requireRepositoryEqual(t, "deleted source has no profiles", mutationCount(t, f.app, `SELECT COUNT(*) FROM vless_keys`), 0)
}

func TestJobSourceOlderFetchFailureDoesNotReplaceNewerSuccessState(t *testing.T) {
	f, sourceID, first := newJobSourceFixture(t)
	first.status = http.StatusServiceUnavailable
	second := &jobFetchTransport{make(chan *http.Request, 1), make(chan struct{}), externalTestVLESS, http.StatusOK}
	f.app.sourceHTTPClient.Transport = &jobOverlapTransport{fetches: []*jobFetchTransport{first, second}}
	_, firstDone := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, first)
	_, secondDone := startControlledSourceJob(t, f, sourceID)
	waitJobFetch(t, second)
	close(second.release)
	waitControlledSourceJob(t, secondDone)
	assertJobSourceStatus(t, f.app, sourceID, "ok")
	close(first.release)
	waitControlledSourceJob(t, firstDone)
	assertJobSourceStatus(t, f.app, sourceID, "ok")
	assertSourceProfileRaw(t, f.app, sourceID, second.body)
}
