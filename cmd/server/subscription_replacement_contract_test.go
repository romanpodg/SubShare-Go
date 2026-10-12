package main

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// Prepared against the existing route seam; land and run before R24a extraction.
func TestSubscriptionReplacementMissingUserCompatibility(t *testing.T) {
	for _, test := range []struct {
		name, method, path string
		status, audits     int
		message            string
	}{
		{"v1 PUT", http.MethodPut, "/api/v1/users/999999/subscription", http.StatusOK, 1, "subscription updated"},
		{"legacy PUT", http.MethodPut, "/api/admin/users/999999/subscription", http.StatusOK, 1, "subscription updated"},
		{"v1 PATCH", http.MethodPatch, "/api/v1/users/999999/subscription", http.StatusNotFound, 0, "user not found"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			assertMutationResponse(t, f.request(test.method, test.path, `{}`), test.path, mutationResponseWant{test.status, test.message})
			requireRepositoryEqual(t, "missing target remains absent", mutationCount(t, f.app, "SELECT COUNT(*) FROM users WHERE id = 999999"), 0)
			assertMutationAudit(t, f.app, "user.subscription.update", test.audits)
		})
	}
}

func TestSubscriptionReplacementValidationPriority(t *testing.T) {
	for _, test := range []struct{ name, body, message string }{
		{"status before dates", `{"status":"invalid","starts_at":"bad","expires_at":"bad"}`, "invalid subscription status"},
		{"start before expiry", `{"starts_at":"bad","expires_at":"bad"}`, "invalid starts_at datetime"},
		{"name before refresh", `{"subscription_name":"` + strings.Repeat("界", 41) + `","subscription_refresh_hours":721}`, "subscription_name is too long (max 120 characters)"},
		{"refresh before info URL", `{"subscription_refresh_hours":721,"subscription_info_url":"javascript:fixture"}`, "subscription_refresh_hours must be between 1 and 720"},
		{"info before extra URL", `{"subscription_info_url":"javascript:fixture","subscription_extra_url":"javascript:fixture"}`, "subscription_info_url must be an absolute http(s) URL"},
		{"extra URL before extra status", `{"subscription_extra_url":"javascript:fixture","subscription_extra_status":"` + strings.Repeat("x", 256) + `"}`, "subscription_extra_url must be an absolute http(s) URL"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			before := readMutationSubscription(t, f.app, id)
			path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/subscription"
			assertMutationResponse(t, f.request(http.MethodPut, path, test.body), path, mutationResponseWant{http.StatusBadRequest, test.message})
			requireRepositoryEqual(t, "validation retains replacement state", readMutationSubscription(t, f.app, id), before)
			assertMutationAudit(t, f.app, "user.subscription.update", 0)
		})
	}
}

func TestSubscriptionReplacementBlockedReasonCompatibility(t *testing.T) {
	for _, test := range []struct {
		method  string
		status  int
		message string
	}{
		{http.MethodPut, http.StatusOK, "subscription updated"},
		{http.MethodPatch, http.StatusBadRequest, "blocked_reason is too long"},
	} {
		t.Run(test.method, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			before := readMutationSubscription(t, f.app, id)
			reason := strings.Repeat("界", 256)
			path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/subscription"
			body := mutationJSON(t, map[string]string{"status": "blocked", "blocked_reason": " " + reason + " "})
			assertMutationResponse(t, f.request(test.method, path, body), path, mutationResponseWant{test.status, test.message})
			if test.status == http.StatusOK {
				requireRepositoryEqual(t, "PUT retains unlimited trimmed reason", readMutationSubscription(t, f.app, id).reason, reason)
				assertMutationAudit(t, f.app, "user.subscription.update", 1)
				return
			}
			requireRepositoryEqual(t, "PATCH validation retains all state", readMutationSubscription(t, f.app, id), before)
			assertMutationAudit(t, f.app, "user.subscription.update", 0)
		})
	}
}
