package main

import (
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"net/http"
)

type jobHTTPFailure struct {
	status  int
	message string
}

var jobHTTPFailures = map[jobCommandError]jobHTTPFailure{
	errJobNotFound:       {http.StatusNotFound, "job not found"},
	errJobLoad:           {http.StatusInternalServerError, "failed to load job"},
	errJobNotFailed:      {http.StatusConflict, "only failed jobs can be retried"},
	errJobTargetInvalid:  {http.StatusConflict, "job target is invalid"},
	errJobNotRetryable:   {http.StatusConflict, "this job type is not retryable"},
	errJobSourceNotFound: {http.StatusNotFound, "source not found"},
	errJobSourceLoad:     {http.StatusInternalServerError, "failed to load source"},
}

func writeJobCommandError(w http.ResponseWriter, r *http.Request, err error, queueMessage string) {
	code, _ := err.(jobCommandError)
	failure, known := jobHTTPFailures[code]
	if !known {
		code = errJobQueue
		failure = jobHTTPFailure{http.StatusInternalServerError, queueMessage}
	}
	httpapi.WriteV1Error(w, r, failure.status, string(code), failure.message)
}

// Read principal only after preparation succeeds, as in the original adapters.
func (a *App) enqueueRequestedJob(w http.ResponseWriter, r *http.Request, plan jobDispatchPlan, queueMessage string) (int64, bool) {
	session, _, _ := a.adminSessionFromRequest(r)
	id, err := a.enqueueAndDispatchJob(plan, session.AdminID, requestIDFromRequest(r))
	if err != nil {
		writeJobCommandError(w, r, err, queueMessage)
		return 0, false
	}
	return id, true
}

func (a *App) respondQueuedJob(w http.ResponseWriter, r *http.Request, prepare func(int64) (jobDispatchPlan, error), queueMessage string, retry bool) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	plan, err := prepare(id)
	if err != nil {
		writeJobCommandError(w, r, err, queueMessage)
		return
	}
	jobID, ok := a.enqueueRequestedJob(w, r, plan, queueMessage)
	if !ok {
		return
	}
	payload := map[string]any{"job_id": jobID, "status": "queued"}
	if retry {
		payload["retry_of"] = id
	}
	httpapi.WriteJSON(w, http.StatusAccepted, payload)
}
