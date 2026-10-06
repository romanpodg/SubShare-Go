package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type mutationConcurrentResult struct {
	status  int
	allowed bool
	err     error
}

func runConcurrentMutations(t *testing.T, calls []func() mutationConcurrentResult) []mutationConcurrentResult {
	t.Helper()
	start := make(chan struct{})
	results := make(chan mutationConcurrentResult, len(calls))
	for _, call := range calls {
		go func() { <-start; results <- call() }()
	}
	close(start)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	collected := make([]mutationConcurrentResult, 0, len(calls))
	for range calls {
		select {
		case result := <-results:
			collected = append(collected, result)
		case <-ctx.Done():
			t.Fatal("concurrent mutation timed out")
		}
	}
	return collected
}

func TestUserActivationConcurrentRequestsHaveOneWinner(t *testing.T) {
	f := newUserMutationFixture(t)
	seedSubscriptionUser(t, f.app, "active")
	call := func() mutationConcurrentResult {
		return mutationConcurrentResult{status: f.request(http.MethodPost, "/api/subscription/activate", `{"activation_code":"activation-token"}`).Code}
	}
	results := runConcurrentMutations(t, []func() mutationConcurrentResult{call, call, call, call, call, call, call, call})
	winners := 0
	for _, result := range results {
		if result.status == http.StatusOK {
			winners++
			continue
		}
		requireRepositoryEqual(t, "concurrent activation loser denied", result.status, http.StatusForbidden)
	}
	requireRepositoryEqual(t, "exactly one activation winner", winners, 1)
	requireRepositoryEqual(t, "activation claim persisted", mutationCount(t, f.app, "SELECT COUNT(*) FROM users WHERE activation_used_at IS NOT NULL"), 1)
}

func TestUserDeviceConcurrentRegistrationsRespectLastSlot(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedSubscriptionUser(t, f.app, "active")
	for attempt := 0; attempt < 5; attempt++ {
		execRepositoryFixtureSQL(t, f.app, "DELETE FROM user_devices")
		calls := make([]func() mutationConcurrentResult, 0, 2)
		for _, hwid := range []string{"one", "two"} {
			calls = append(calls, func() mutationConcurrentResult {
				allowed, err := f.app.registerHWID(id, hwid, deviceMeta{})
				return mutationConcurrentResult{allowed: allowed, err: err}
			})
		}
		results := runConcurrentMutations(t, calls)
		accepted := 0
		for _, result := range results {
			requireRepositorySuccess(t, result.err)
			if result.allowed {
				accepted++
			}
		}
		requireRepositoryEqual(t, "one remaining device slot accepted", accepted, 1)
		requireRepositoryEqual(t, "concurrent device count bounded", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices WHERE user_id = ?", id), 1)
	}
}

func TestUserDeviceCommitFailureRetainsPreviousMetadata(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	id := seedSubscriptionUser(t, f.app, "active")
	allowed, err := f.app.registerHWID(id, " Device ", deviceMeta{AppVersion: "old"})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "initial device registered", allowed, true)
	faults.commitErr = errors.New("injected device commit")
	allowed, err = f.app.registerHWID(id, "device", deviceMeta{AppVersion: "new"})
	requireRepositoryEqual(t, "device commit failure denied", allowed, false)
	requireRepositoryEqual(t, "device commit error retained", errors.Is(err, faults.commitErr), true)
	var version string
	requireRepositorySuccess(t, f.app.db.QueryRow("SELECT app_version FROM user_devices WHERE user_id = ?", id).Scan(&version))
	requireRepositoryEqual(t, "device metadata rolled back", version, "old")
	faults.commitErr = nil
	allowed, err = f.app.registerHWID(id, "DEVICE", deviceMeta{AppVersion: "new"})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "existing device works at capacity", allowed, true)
	requireRepositoryEqual(t, "normalized device identity remains unique", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices WHERE user_id = ?", id), 1)
}

// Characterize the existing lost-update defect, not a desired compatibility
// requirement. Both PATCH requests read before either UPDATE is executed.
func TestSubscriptionMutationCharacterizesDisjointPatchLostUpdate(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	var ready atomic.Int32
	gate := make(chan struct{})
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	faults.beforeExec = func(query string) {
		if !strings.Contains(query, "subscription_name = ?") {
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
	path := "/api/v1/users/1/subscription"
	results := runConcurrentMutations(t, []func() mutationConcurrentResult{
		func() mutationConcurrentResult {
			return mutationConcurrentResult{status: f.request(http.MethodPatch, path, `{"status":"paused"}`).Code}
		},
		func() mutationConcurrentResult {
			return mutationConcurrentResult{status: f.request(http.MethodPatch, path, `{"subscription_name":"changed title"}`).Code}
		},
	})
	for _, result := range results {
		requireRepositoryEqual(t, "concurrent PATCH reports success", result.status, http.StatusOK)
	}
	requireRepositoryEqual(t, "PATCH overlap actually synchronized", ready.Load(), int32(2))
	state := readMutationSubscription(t, f.app, id)
	persisted := 0
	if state.status == "paused" {
		persisted++
	}
	if state.name == "changed title" {
		persisted++
	}
	requireRepositoryEqual(t, "current whole-row PATCH loses one disjoint update", persisted, 1)
}
