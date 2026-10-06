package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func isActivationClaimWrite(query string) bool {
	return strings.Contains(query, "activation_used_at = CURRENT_TIMESTAMP")
}

func synchronizeActivationClaims(t *testing.T, faults *mutationSQLFaults) *atomic.Int32 {
	t.Helper()
	var ready atomic.Int32
	gate := make(chan struct{})
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	faults.beforeExec = func(query string) {
		if !isActivationClaimWrite(query) {
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

func TestUserActivationConcurrentClaimsPreserveWinner(t *testing.T) {
	for _, stored := range []any{nil, " subscription-token "} {
		t.Run(mutationJSON(t, stored), func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			id := seedSubscriptionUser(t, f.app, "active")
			execRepositoryFixtureSQL(t, f.app, "UPDATE users SET subscription_id = ? WHERE id = ?", stored, id)
			f.app.baseURL = "https://subscription.example"
			ready := synchronizeActivationClaims(t, faults)
			calls := make([]func() mutationConcurrentResult, 0, 2)
			for _, path := range []string{"/api/subscription/activate", "/api/v1/subscriptions/activate"} {
				calls = append(calls, func() mutationConcurrentResult {
					response := f.request(http.MethodPost, path, `{"activation_code":"activation-token"}`)
					var payload struct {
						SubscriptionURL string `json:"subscription_url"`
						Error           string `json:"error"`
					}
					err := json.Unmarshal(response.Body.Bytes(), &payload)
					return mutationConcurrentResult{status: response.Code, code: payload.SubscriptionURL, err: err}
				})
			}
			results := runConcurrentMutations(t, calls)
			requireRepositoryEqual(t, "both requests reached the conditional claim", ready.Load(), int32(2))
			var subscriptionID string
			var usedAt sql.NullTime
			requireRepositorySuccess(t, f.app.db.QueryRow("SELECT subscription_id, activation_used_at FROM users WHERE id = ?", id).Scan(&subscriptionID, &usedAt))
			requireRepositoryEqual(t, "winning identity persisted", subscriptionID != "", true)
			requireRepositoryEqual(t, "winning claim timestamp persisted", usedAt.Valid, true)
			winners := 0
			for _, result := range results {
				requireRepositorySuccess(t, result.err)
				if result.status == http.StatusOK {
					winners++
					requireRepositoryEqual(t, "winner returns only the persisted identity", result.code, "https://subscription.example/sub/"+subscriptionID)
					continue
				}
				requireRepositoryEqual(t, "stale claim denied", result.status, http.StatusForbidden)
				requireRepositoryEqual(t, "losing identity withheld", result.code, "")
			}
			requireRepositoryEqual(t, "exactly one synchronized winner", winners, 1)
			claimed, err := f.app.claimActivation(id, "replacement")
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "later claim cannot overwrite winner", claimed, false)
			var retainedID string
			var retainedTime sql.NullTime
			requireRepositorySuccess(t, f.app.db.QueryRow("SELECT subscription_id, activation_used_at FROM users WHERE id = ?", id).Scan(&retainedID, &retainedTime))
			requireRepositoryEqual(t, "winner identity retained", retainedID, subscriptionID)
			requireRepositoryEqual(t, "winner timestamp retained", retainedTime, usedAt)
		})
	}
}

func TestUserActivationUnknownClaimOutcomeWithholdsLink(t *testing.T) {
	for _, path := range []string{"/api/subscription/activate", "/api/v1/subscriptions/activate"} {
		t.Run(path, func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			seedSubscriptionUser(t, f.app, "active")
			faults.activationRowsAffectedErr = errors.New("private affected-row diagnostic")
			response := f.request(http.MethodPost, path, `{"activation_code":"activation-token"}`)
			assertMutationResponse(t, response, "/api/subscription/activate", mutationResponseWant{http.StatusInternalServerError, "failed to activate subscription"})
			requireRepositoryEqual(t, "private persistence cause is not serialized", strings.Contains(response.Body.String(), "private"), false)
			// The UPDATE has committed before its result cannot be confirmed.
			requireRepositoryEqual(t, "unknown reporting outcome does not imply rollback", mutationCount(t, f.app, "SELECT COUNT(*) FROM users WHERE activation_used_at IS NOT NULL"), 1)
			faults.activationRowsAffectedErr = nil
			assertMutationResponse(t, f.request(http.MethodPost, path, `{"activation_code":"activation-token"}`), "/api/subscription/activate", mutationResponseWant{http.StatusForbidden, "Ключ уже активирован"})
		})
	}
}

func TestUserActivationIdentityConflictLeavesClaimUnused(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedSubscriptionUser(t, f.app, "active")
	execRepositoryFixtureSQL(t, f.app, "UPDATE users SET subscription_id = ' occupied ' WHERE id = ?", id)
	execRepositoryFixtureSQL(t, f.app, "INSERT INTO users(name, token, activation_code, subscription_id, status) VALUES('other', 'other-token', 'other-code', 'occupied', 'active')")
	for _, path := range []string{"/api/subscription/activate", "/api/v1/subscriptions/activate"} {
		assertMutationResponse(t, f.request(http.MethodPost, path, `{"activation_code":"activation-token"}`), "/api/subscription/activate", mutationResponseWant{http.StatusInternalServerError, "failed to activate subscription"})
	}
	requireRepositoryEqual(t, "identity conflict leaves activation unused", mutationCount(t, f.app, "SELECT COUNT(*) FROM users WHERE activation_used_at IS NOT NULL"), 0)
	var stored string
	requireRepositorySuccess(t, f.app.db.QueryRow("SELECT subscription_id FROM users WHERE id = ?", id).Scan(&stored))
	requireRepositoryEqual(t, "failed atomic claim retains original identity", stored, " occupied ")
}

func TestUserActivationCommandWithholdsUnknownIdentity(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	seedSubscriptionUser(t, f.app, "active")
	faults.activationRowsAffectedErr = errors.New("injected confirmation failure")
	id, err := f.app.redeemActivationCode("activation-token")
	requireRepositoryEqual(t, "unknown command outcome withholds identity", id, "")
	requireRepositoryEqual(t, "command preserves persistence cause", errors.Is(err, faults.activationRowsAffectedErr), true)
	requireRepositoryEqual(t, "unknown outcome is not a missing code", errors.Is(err, errActivationNotFound), false)
	requireRepositoryEqual(t, "unknown outcome is not a known stale claim", errors.Is(err, errActivationAlreadyUsed), false)
}

func TestUserActivationPolicyRetainsValidationContract(t *testing.T) {
	for _, test := range []struct {
		name, raw, want string
		invalid         bool
	}{
		{"trim", "\t code \n", "code", false},
		{"unicode whitespace", "\u2003code\u00a0", "code", false},
		{"empty", "", "", true},
		{"blank", " \t\n", "", true},
		{"slash", "bad/code", "", true},
		{"internal space retained", "two words", "two words", false},
		{"no new length limit", strings.Repeat("x", 129), strings.Repeat("x", 129), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, err := validateActivationCode(test.raw)
			requireRepositoryEqual(t, "normalized code", code, test.want)
			requireRepositoryEqual(t, "invalid code outcome", errors.Is(err, errActivationCodeInvalid), test.invalid)
		})
	}
}
