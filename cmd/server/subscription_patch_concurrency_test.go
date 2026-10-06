package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func isSubscriptionPatchWrite(query string) bool {
	return strings.Contains(query, "AND time_zone IS ?")
}

func synchronizeSubscriptionPatchWrites(t *testing.T, faults *mutationSQLFaults) *atomic.Int32 {
	t.Helper()
	var ready atomic.Int32
	gate := make(chan struct{})
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	faults.beforeExec = func(query string) {
		if !isSubscriptionPatchWrite(query) {
			return
		}
		if ready.Add(1) == 2 {
			close(gate)
		}
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	return &ready
}

func TestSubscriptionMutationConcurrentPatchPreservesExplicitClear(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	ready := synchronizeSubscriptionPatchWrites(t, faults)
	results := runConcurrentMutations(t, []func() mutationConcurrentResult{
		func() mutationConcurrentResult {
			return mutationConcurrentResult{status: f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"status":"paused"}`).Code}
		},
		func() mutationConcurrentResult {
			return mutationConcurrentResult{status: f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"subscription_name":null}`).Code}
		},
	})
	for _, result := range results {
		requireRepositoryEqual(t, "concurrent clear PATCH succeeds", result.status, http.StatusOK)
	}
	requireRepositoryEqual(t, "clear PATCH overlap synchronized", ready.Load() >= 2, true)
	state := readMutationSubscription(t, f.app, id)
	requireRepositoryEqual(t, "concurrent pause preserved", state.status, "paused")
	requireRepositoryEqual(t, "concurrent explicit clear preserved", state.name, "")
	requireRepositoryEqual(t, "blocked reason stays cleared", state.reason, "")
}

func TestSubscriptionMutationConcurrentPatchRevalidatesDateRange(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	execRepositoryFixtureSQL(t, f.app, "UPDATE users SET expires_at = '2026-12-01T00:00:00Z' WHERE id = ?", id)
	ready := synchronizeSubscriptionPatchWrites(t, faults)
	results := runConcurrentMutations(t, []func() mutationConcurrentResult{
		func() mutationConcurrentResult {
			return subscriptionPatchResponseResult(f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"starts_at":"2026-09-01T00:00:00Z"}`))
		},
		func() mutationConcurrentResult {
			return subscriptionPatchResponseResult(f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"expires_at":"2026-06-01T00:00:00Z"}`))
		},
	})
	assertSubscriptionDateRaceResults(t, results)
	requireRepositoryEqual(t, "date PATCH overlap synchronized", ready.Load() >= 2, true)
	state := readMutationSubscription(t, f.app, id)
	requireRepositoryEqual(t, "stored date range remains ordered", state.starts <= state.expires, true)
	assertMutationAudit(t, f.app, "user.subscription.update", 1)
}

func subscriptionPatchResponseResult(recorder *httptest.ResponseRecorder) mutationConcurrentResult {
	var payload struct{ Code string }
	err := json.Unmarshal(recorder.Body.Bytes(), &payload)
	return mutationConcurrentResult{status: recorder.Code, code: payload.Code, err: err}
}

func assertSubscriptionDateRaceResults(t *testing.T, results []mutationConcurrentResult) {
	t.Helper()
	winners := 0
	for _, result := range results {
		requireRepositorySuccess(t, result.err)
		if result.status == http.StatusOK {
			winners++
			continue
		}
		requireRepositoryEqual(t, "stale date patch rejected", result.status, http.StatusBadRequest)
		requireRepositoryEqual(t, "fresh date validation error", result.code, "date_range_invalid")
	}
	requireRepositoryEqual(t, "one consistent date patch wins", winners, 1)
}

func TestSubscriptionMutationConcurrentPatchRetryBudget(t *testing.T) {
	for _, conflicts := range []int{4, 5} {
		t.Run(strconv.Itoa(conflicts), func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			var attempts atomic.Int32
			faults.beforeExec = func(query string) {
				if !isSubscriptionPatchWrite(query) {
					return
				}
				attempt := attempts.Add(1)
				if int(attempt) <= conflicts {
					execRepositoryFixtureSQL(t, f.app, "UPDATE users SET subscription_extra_status = ? WHERE id = ?", "writer-"+strconv.Itoa(int(attempt)), id)
				}
			}
			response := f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"subscription_name":"changed title"}`)
			assertSubscriptionRetryOutcome(t, f, response, conflicts)
			requireRepositoryEqual(t, "bounded conditional writes", attempts.Load(), int32(5))
			state := readMutationSubscription(t, f.app, id)
			requireRepositoryEqual(t, "competing changes retained", state.extraStatus, "writer-"+strconv.Itoa(conflicts))
		})
	}
}

func assertSubscriptionRetryOutcome(t *testing.T, f userMutationFixture, response *httptest.ResponseRecorder, conflicts int) {
	t.Helper()
	if conflicts == 4 {
		assertMutationResponse(t, response, "/api/v1/users/1/subscription", mutationResponseWant{http.StatusOK, "subscription updated"})
		assertMutationAudit(t, f.app, "user.subscription.update", 1)
		return
	}
	assertMutationResponse(t, response, "/api/v1/users/1/subscription", mutationResponseWant{http.StatusConflict, "subscription changed; retry the request"})
	requireRepositoryEqual(t, "retry exhaustion error code", decodeJSONMap(t, response)["code"], "subscription_conflict")
	requireRepositoryEqual(t, "exhausted patch does not overwrite title", readMutationSubscription(t, f.app, 1).name, "Personal title")
	assertMutationAudit(t, f.app, "user.subscription.update", 0)
}

func TestSubscriptionMutationConcurrentPatchReloadsTimezone(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	var attempts atomic.Int32
	faults.beforeExec = func(query string) {
		if isSubscriptionPatchWrite(query) && attempts.Add(1) == 1 {
			execRepositoryFixtureSQL(t, f.app, "UPDATE users SET time_zone = 'Asia/Omsk' WHERE id = ?", id)
		}
	}
	response := f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"starts_at":"2026-07-29T12:00"}`)
	assertMutationResponse(t, response, "/api/v1/users/1/subscription", mutationResponseWant{http.StatusOK, "subscription updated"})
	requireRepositoryEqual(t, "timezone change causes a retry", attempts.Load(), int32(2))
	requireRepositoryEqual(t, "latest timezone used for datetime", readMutationSubscription(t, f.app, id).starts, "2026-07-29T06:00:00Z")
}

func TestSubscriptionMutationConcurrentPatchDetectsDeletion(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	var attempted atomic.Bool
	faults.beforeExec = func(query string) {
		if isSubscriptionPatchWrite(query) && attempted.CompareAndSwap(false, true) {
			execRepositoryFixtureSQL(t, f.app, "DELETE FROM users WHERE id = ?", id)
		}
	}
	response := f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"subscription_name":"changed title"}`)
	assertMutationResponse(t, response, "/api/v1/users/1/subscription", mutationResponseWant{http.StatusNotFound, "user not found"})
	requireRepositoryEqual(t, "deleted user error code", decodeJSONMap(t, response)["code"], "user_not_found")
	assertMutationAudit(t, f.app, "user.subscription.update", 0)
}

func TestSubscriptionMutationConcurrentPatchPreservesInterveningPut(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	var attempted atomic.Bool
	faults.beforeExec = func(query string) {
		if !isSubscriptionPatchWrite(query) || !attempted.CompareAndSwap(false, true) {
			return
		}
		response := f.request(http.MethodPut, "/api/admin/users/1/subscription", `{"status":"paused"}`)
		assertMutationResponse(t, response, "/api/admin/users/1/subscription", mutationResponseWant{http.StatusOK, "subscription updated"})
	}
	response := f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"subscription_name":"changed title"}`)
	assertMutationResponse(t, response, "/api/v1/users/1/subscription", mutationResponseWant{http.StatusOK, "subscription updated"})
	state := readMutationSubscription(t, f.app, id)
	requireRepositoryEqual(t, "intervening PUT and PATCH both retained", state, mutationSubscriptionState{status: "paused", name: "changed title", refresh: 12})
	assertMutationAudit(t, f.app, "user.subscription.update", 2)
}
