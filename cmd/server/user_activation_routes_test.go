package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUserActivationRegisteredRoutesAreSingleUse(t *testing.T) {
	for _, path := range []string{"/api/subscription/activate", "/api/v1/subscriptions/activate"} {
		t.Run(path, func(t *testing.T) {
			f := newUserMutationFixture(t)
			seedSubscriptionUser(t, f.app, "active")
			f.app.baseURL = "https://subscription.example"
			response := f.request(http.MethodPost, path, `{"activation_code":" activation-token "}`)
			requireRepositoryEqual(t, "activation response status", response.Code, http.StatusOK)
			payload := decodeJSONMap(t, response)
			requireRepositoryEqual(t, "activation delivery URL", payload["subscription_url"], "https://subscription.example/sub/subscription-token")
			assertMutationResponse(t, f.request(http.MethodPost, path, `{"activation_code":"activation-token"}`), "/api/subscription/activate", mutationResponseWant{status: http.StatusForbidden, message: "Ключ уже активирован"})
			requireRepositoryEqual(t, "activation persisted once", mutationCount(t, f.app, "SELECT COUNT(*) FROM users WHERE activation_used_at IS NOT NULL"), 1)
		})
	}
}

func TestUserActivationValidationAndFailureRetainClaim(t *testing.T) {
	for _, test := range []struct {
		name, body, trigger, message string
		status                       int
	}{
		{"malformed", "{", "", "invalid request body", http.StatusBadRequest},
		{"missing code", `{}`, "", "Введите корректный ключ активации", http.StatusBadRequest},
		{"slash code", `{"activation_code":"bad/code"}`, "", "Введите корректный ключ активации", http.StatusBadRequest},
		{"unknown", `{"activation_code":"unknown"}`, "", "Подписка не найдена", http.StatusNotFound},
		{"update failure", `{"activation_code":"activation-token"}`, `CREATE TRIGGER reject_activation BEFORE UPDATE OF activation_used_at ON users BEGIN SELECT RAISE(ABORT, 'injected activation failure'); END`, "failed to activate subscription", http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			seedSubscriptionUser(t, f.app, "active")
			if test.trigger != "" {
				execRepositoryFixtureSQL(t, f.app, test.trigger)
			}
			// Both aliases deliberately retain the legacy error shape: activation
			// is a public route, without the admin v1 envelope adapter.
			for _, path := range []string{"/api/subscription/activate", "/api/v1/subscriptions/activate"} {
				response := f.request(http.MethodPost, path, test.body)
				assertMutationResponse(t, response, "/api/subscription/activate", mutationResponseWant{status: test.status, message: test.message})
			}
			requireRepositoryEqual(t, "failed activation remains unused", mutationCount(t, f.app, "SELECT COUNT(*) FROM users WHERE activation_used_at IS NOT NULL"), 0)
		})
	}
}

func TestUserActivationKeepsAccessPolicySeparate(t *testing.T) {
	for _, test := range []struct {
		name, status, column string
		date                 time.Time
		accessCode           int
	}{
		{"paused", "paused", "", time.Time{}, http.StatusForbidden},
		{"blocked", "blocked", "", time.Time{}, http.StatusForbidden},
		{"future", "active", "starts_at", time.Now().Add(time.Hour).UTC(), http.StatusForbidden},
		{"expired", "active", "expires_at", time.Now().Add(-time.Hour).UTC(), http.StatusGone},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedSubscriptionUser(t, f.app, test.status)
			if test.column != "" {
				execRepositoryFixtureSQL(t, f.app, "UPDATE users SET "+test.column+" = ? WHERE id = ?", test.date, id)
			}
			response := f.request(http.MethodPost, "/api/subscription/activate", `{"activation_code":"activation-token"}`)
			requireRepositoryEqual(t, "activation does not grant subscription access", response.Code, http.StatusOK)
			allowed, _, status, _, err := f.app.subscriptionAccessAllowed("subscription-token")
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "access remains denied after claim", allowed, false)
			requireRepositoryEqual(t, "access denial status retained", status, test.accessCode)
		})
	}
}

func TestUserActivationCryptoFailureKeepsPlainURLFallback(t *testing.T) {
	crypto := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer crypto.Close()
	f := newUserMutationFixture(t)
	seedSubscriptionUser(t, f.app, "active")
	f.app.baseURL, f.app.happCryptoAPIURL = "https://subscription.example", crypto.URL
	response := f.request(http.MethodPost, "/api/subscription/activate", `{"activation_code":"activation-token"}`)
	requireRepositoryEqual(t, "crypto fallback activation status", response.Code, http.StatusOK)
	requireRepositoryEqual(t, "crypto fallback URL", decodeJSONMap(t, response)["subscription_url"], "https://subscription.example/sub/subscription-token")
}
