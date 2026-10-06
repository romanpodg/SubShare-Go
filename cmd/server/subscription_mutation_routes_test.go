package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type mutationSubscriptionState struct {
	status, starts, expires, reason, name, info, extra, extraStatus string
	refresh                                                         int
}

func seedMutationSubscription(t *testing.T, app *App) int64 {
	t.Helper()
	id := seedSubscriptionUser(t, app, "blocked")
	execRepositoryFixtureSQL(t, app, `UPDATE users SET starts_at = '2026-01-01T00:00:00Z', expires_at = '2027-01-01T00:00:00Z',
		blocked_reason = 'old reason', subscription_extra_url = 'https://extra.example/old',
		subscription_extra_status = 'old extra', time_zone = 'America/New_York' WHERE id = ?`, id)
	return id
}

func readMutationSubscription(t *testing.T, app *App, id int64) mutationSubscriptionState {
	t.Helper()
	var state mutationSubscriptionState
	var starts, expires sql.NullTime
	err := app.db.QueryRow(`SELECT status, starts_at, expires_at, COALESCE(blocked_reason, ''),
		COALESCE(subscription_name, ''), subscription_refresh_hours, COALESCE(subscription_info_url, ''),
		COALESCE(subscription_extra_url, ''), COALESCE(subscription_extra_status, '') FROM users WHERE id = ?`, id).Scan(
		&state.status, &starts, &expires, &state.reason, &state.name, &state.refresh, &state.info, &state.extra, &state.extraStatus)
	requireRepositorySuccess(t, err)
	state.starts, state.expires = mutationTimeString(starts), mutationTimeString(expires)
	return state
}

func mutationTimeString(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

func TestSubscriptionMutationOmittedNullAndEmptyContracts(t *testing.T) {
	for _, test := range []struct {
		name, method, body string
		want               mutationSubscriptionState
	}{
		{"PATCH omitted", http.MethodPatch, `{}`, mutationSubscriptionState{"blocked", "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", "old reason", "Personal title", "https://user.example/info", "https://extra.example/old", "old extra", 24}},
		{"PUT omitted", http.MethodPut, `{}`, mutationSubscriptionState{status: "active", refresh: 12}},
		{"PATCH null", http.MethodPatch, `{"starts_at":null,"expires_at":null,"blocked_reason":null,"subscription_name":null,"subscription_info_url":null,"subscription_extra_url":null,"subscription_extra_status":null,"subscription_refresh_hours":null}`, mutationSubscriptionState{status: "blocked"}},
		{"PATCH empty", http.MethodPatch, `{"starts_at":"","expires_at":"","blocked_reason":"","subscription_name":"","subscription_info_url":"","subscription_extra_url":"","subscription_extra_status":"","subscription_refresh_hours":0}`, mutationSubscriptionState{status: "blocked"}},
		{"PUT null", http.MethodPut, `{"status":null,"starts_at":null,"expires_at":null,"subscription_name":null,"subscription_refresh_hours":null}`, mutationSubscriptionState{status: "active", refresh: 12}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/subscription"
			assertMutationResponse(t, f.request(test.method, path, test.body), path, mutationResponseWant{status: http.StatusOK, message: "subscription updated"})
			requireRepositoryEqual(t, "endpoint optional-field contract", readMutationSubscription(t, f.app, id), test.want)
		})
	}
}

func TestSubscriptionMutationTimezoneAndDateContracts(t *testing.T) {
	for _, test := range []struct{ name, zone, input, want string }{
		{"Omsk", "Asia/Omsk", "2026-07-29T12:00", "2026-07-29T06:00:00Z"},
		{"DST before jump", "America/New_York", "2026-03-08T01:30", "2026-03-08T06:30:00Z"},
		{"DST after jump", "America/New_York", "2026-03-08T03:30", "2026-03-08T07:30:00Z"},
		{"invalid stored zone falls back", "not-a-timezone", "2026-07-29T12:00", "2026-07-29T12:00:00Z"},
		{"explicit offset", "Asia/Omsk", "2026-07-29T12:00:00+03:00", "2026-07-29T09:00:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			execRepositoryFixtureSQL(t, f.app, "UPDATE users SET time_zone = ? WHERE id = ?", test.zone, id)
			path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/subscription"
			body := mutationJSON(t, map[string]string{"starts_at": test.input, "expires_at": test.input})
			assertMutationResponse(t, f.request(http.MethodPatch, path, body), path, mutationResponseWant{status: http.StatusOK, message: "subscription updated"})
			state := readMutationSubscription(t, f.app, id)
			requireRepositoryEqual(t, "PATCH uses user timezone", state.starts, test.want)
			requireRepositoryEqual(t, "equal start and expiry accepted", state.expires, test.want)
			assertMutationResponse(t, f.request(http.MethodPut, path, body), path, mutationResponseWant{status: http.StatusOK, message: "subscription updated"})
			state = readMutationSubscription(t, f.app, id)
			wantLocal := test.want
			if !strings.Contains(test.input, "+03:00") {
				parsed, err := time.ParseInLocation("2006-01-02T15:04", test.input, time.Local)
				requireRepositorySuccess(t, err)
				wantLocal = parsed.UTC().Format(time.RFC3339)
			}
			requireRepositoryEqual(t, "PUT uses server timezone", state.starts, wantLocal)
		})
	}
}

func TestSubscriptionMutationValidationRetainsState(t *testing.T) {
	for _, test := range []struct{ name, method, body, message string }{
		{"PUT malformed", http.MethodPut, `{`, "invalid request body"},
		{"PUT status", http.MethodPut, `{"status":"unknown"}`, "invalid subscription status"},
		{"PUT starts", http.MethodPut, `{"starts_at":"bad-date"}`, "invalid starts_at datetime"},
		{"PUT expires", http.MethodPut, `{"expires_at":"bad-date"}`, "invalid expires_at datetime"},
		{"PUT range", http.MethodPut, `{"starts_at":"2028-01-01T00:00:00Z","expires_at":"2027-01-01T00:00:00Z"}`, "starts_at must be before expires_at"},
		{"PUT refresh", http.MethodPut, `{"subscription_refresh_hours":721}`, "subscription_refresh_hours must be between 1 and 720"},
		{"PUT extra status", http.MethodPut, mutationJSON(t, map[string]string{"subscription_extra_status": strings.Repeat("x", 256)}), "subscription_extra_status is too long (max 255 characters)"},
		{"status", http.MethodPatch, `{"status":"unknown"}`, "invalid subscription status"},
		{"null status", http.MethodPatch, `{"status":null}`, "status cannot be null"},
		{"date", http.MethodPatch, `{"starts_at":"bad-date"}`, "invalid starts_at datetime"},
		{"range", http.MethodPatch, `{"starts_at":"2028-01-01T00:00:00Z"}`, "starts_at must be before expires_at"},
		{"refresh", http.MethodPatch, `{"subscription_refresh_hours":-1}`, "subscription_refresh_hours must be between 1 and 720"},
		{"URL", http.MethodPatch, `{"subscription_info_url":"javascript:alert(1)"}`, "subscription_info_url must be an absolute http(s) URL"},
		{"userinfo URL", http.MethodPut, `{"subscription_extra_url":"https://user:pass@example.test/"}`, "subscription_extra_url must be an absolute http(s) URL"},
		{"Unicode legacy length", http.MethodPut, mutationJSON(t, map[string]string{"subscription_name": strings.Repeat("界", 50)}), "subscription_name is too long (max 120 characters)"},
		{"PATCH rune length", http.MethodPatch, mutationJSON(t, map[string]string{"subscription_name": strings.Repeat("界", 121)}), "subscription_name is too long"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			before := readMutationSubscription(t, f.app, id)
			path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/subscription"
			assertMutationResponse(t, f.request(test.method, path, test.body), path, mutationResponseWant{status: http.StatusBadRequest, message: test.message})
			requireRepositoryEqual(t, "invalid subscription update is atomic", readMutationSubscription(t, f.app, id), before)
			assertMutationAudit(t, f.app, "user.subscription.update", 0)
		})
	}
}

func TestSubscriptionMutationExplicitOverridesAreNormalized(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			want := readMutationSubscription(t, f.app, id)
			want.reason, want.name, want.refresh = "new reason", "changed title", 720
			want.info, want.extra, want.extraStatus = "https://info.example/new", "https://extra.example/new", "new extra"
			if method == http.MethodPut {
				want.starts, want.expires = "", ""
			}
			path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/subscription"
			body := `{"status":"blocked","blocked_reason":" new reason ","subscription_name":" changed title ","subscription_refresh_hours":720,
				"subscription_info_url":" https://info.example/new ","subscription_extra_url":" https://extra.example/new ","subscription_extra_status":" new extra "}`
			assertMutationResponse(t, f.request(method, path, body), path, mutationResponseWant{http.StatusOK, "subscription updated"})
			requireRepositoryEqual(t, "explicit subscription overrides normalized", readMutationSubscription(t, f.app, id), want)
		})
	}
}

func TestSubscriptionMutationLegacyPutUsesSameReplacementContract(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	path := "/api/admin/users/" + strconv.FormatInt(id, 10) + "/subscription"
	assertMutationResponse(t, f.request(http.MethodPut, path, `{}`), path, mutationResponseWant{http.StatusOK, "subscription updated"})
	requireRepositoryEqual(t, "legacy PUT replacement contract", readMutationSubscription(t, f.app, id), mutationSubscriptionState{status: "active", refresh: 12})
}

func TestUserSettingsRejectsInvalidTimezoneWithoutWrites(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/settings"
	assertMutationResponse(t, f.request(http.MethodPut, path, `{"time_zone":"not-a-timezone"}`), path, mutationResponseWant{http.StatusBadRequest, "time_zone must be a valid IANA timezone"})
	var zone string
	requireRepositorySuccess(t, f.app.db.QueryRow("SELECT time_zone FROM users WHERE id = ?", id).Scan(&zone))
	requireRepositoryEqual(t, "invalid timezone does not replace existing zone", zone, "America/New_York")
}

func TestSubscriptionMutationSQLFailureRetainsState(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	before := readMutationSubscription(t, f.app, id)
	execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER reject_subscription BEFORE UPDATE ON users
		BEGIN SELECT RAISE(ABORT, 'injected subscription failure'); END`)
	path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/subscription"
	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		assertMutationResponse(t, f.request(method, path, `{"status":"paused"}`), path, mutationResponseWant{status: http.StatusInternalServerError, message: "failed to update subscription"})
		requireRepositoryEqual(t, "SQL failure retains subscription", readMutationSubscription(t, f.app, id), before)
	}
	assertMutationAudit(t, f.app, "user.subscription.update", 0)
}
