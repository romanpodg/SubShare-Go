package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestJobSourceHTTPShutdownDoesNotJoinDetachedWorker(t *testing.T) {
	f, sourceID, fetch := newJobSourceFixture(t)
	server := httptest.NewServer(f.handler)
	defer server.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/sources/"+strconv.FormatInt(sourceID, 10)+"/sync", nil)
	requireRepositorySuccess(t, err)
	request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: f.session})
	request.Header.Set("X-CSRF-Token", f.csrf)
	response, err := server.Client().Do(request)
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "queue response completed before worker", response.StatusCode, http.StatusAccepted)
	requireRepositorySuccess(t, response.Body.Close())
	workerRequest := waitJobFetch(t, fetch)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	requireRepositorySuccess(t, server.Config.Shutdown(ctx))
	requireRepositoryEqual(t, "HTTP shutdown leaves worker context active", workerRequest.Context().Err(), error(nil))
	var jobID int64
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT id FROM background_jobs`).Scan(&jobID))
	assertJobLifecycleState(t, f.app, jobID, [4]any{"running", "", true, false})
	close(fetch.release)
	requireRepositoryEqual(t, "detached worker completes after HTTP shutdown", waitJobSourceTerminal(t, f.app, jobID), "succeeded")
}
