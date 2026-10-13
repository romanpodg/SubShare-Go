package main

import (
	"github.com/romanpodg/SubShare-Go/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSubscriptionDeliveryStatePolicy(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, app *App)
		subID      string
		wantCode   int
		wantStatus string
		wantReason string
	}{
		{name: "unknown", subID: "missing", wantCode: http.StatusNotFound, wantStatus: "not-found", wantReason: "not found"},
		{name: "paused", subID: "subscription-token", setup: func(t *testing.T, app *App) {
			{
				_, err := app.db.Exec(`UPDATE users SET status = 'paused' WHERE subscription_id = 'subscription-token'`)
				requireFixturef(t, err == nil, "fixture Exec: %v", err)
			}

		}, wantCode: http.StatusForbidden, wantStatus: "paused", wantReason: "paused"},
		{name: "blocked reason", subID: "subscription-token", setup: func(t *testing.T, app *App) {
			{
				_, err := app.db.Exec(`UPDATE users SET status = 'blocked', blocked_reason = 'billing hold' WHERE subscription_id = 'subscription-token'`)
				requireFixturef(t, err == nil, "fixture Exec: %v", err)
			}

		}, wantCode: http.StatusForbidden, wantStatus: "blocked", wantReason: "billing hold"},
		{name: "future", subID: "subscription-token", setup: func(t *testing.T, app *App) {
			{
				_, err := app.db.Exec(`UPDATE users SET starts_at = ? WHERE subscription_id = 'subscription-token'`, time.Now().Add(time.Hour).UTC())
				requireFixturef(t, err == nil, "fixture Exec: %v", err)
			}

		}, wantCode: http.StatusForbidden, wantStatus: "future", wantReason: "not active"},
		{name: "expired", subID: "subscription-token", setup: func(t *testing.T, app *App) {
			{
				_, err := app.db.Exec(`UPDATE users SET expires_at = ? WHERE subscription_id = 'subscription-token'`, time.Now().Add(-time.Hour).UTC())
				requireFixturef(t, err == nil, "fixture Exec: %v", err)
			}

		}, wantCode: http.StatusGone, wantStatus: "expired", wantReason: "expired"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			seedSubscriptionUser(t, app, model.UserStatusActive)
			if test.setup != nil {
				test.setup(t, app)
			}
			request := httptest.NewRequest(http.MethodGet, "/sub/"+test.subID, nil)
			_, denial, err := app.prepareSubscriptionDelivery(request, test.subID, false)
			requireFixturef(t, err == nil, "prepare: %v", err)

			requireFixturef(t, denial.Code == test.wantCode, "denial=%+v", denial)
			requireFixturef(t, denial.Status == test.wantStatus, "denial=%+v", denial)
			requireFixturef(t, strings.Contains(denial.Reason, test.wantReason), "denial=%+v", denial)

		})
	}
}

func TestSubscriptionDevicePolicyMandatoryOptionalAndLimit(t *testing.T) {
	app := newIntegrationApp(t)
	seedSubscriptionUser(t, app, model.UserStatusActive)
	optional := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
	{
		_, denial, err := app.prepareSubscriptionDelivery(optional, "subscription-token", false)
		requireFixturef(t, err == nil, "optional request denied: %+v err=%v", denial, err)
		requireFixturef(t, !(denial.denied()), "optional request denied: %+v err=%v", denial, err)
	}

	{
		_, err := app.db.Exec(`UPDATE subscription_settings SET provider_id = 'ABCDEFGH', happ_mandatory_hwid = 1 WHERE id = 1`)
		requireFixture(t, err == nil, err)
	}

	{
		_, denial, err := app.prepareSubscriptionDelivery(optional, "subscription-token", false)
		requireFixturef(t, err == nil, "mandatory HWID policy: %+v err=%v", denial, err)
		requireFixturef(t, denial.Code == http.StatusForbidden, "mandatory HWID policy: %+v err=%v", denial, err)
		requireFixturef(t, denial.Status == "limited", "mandatory HWID policy: %+v err=%v", denial, err)
		requireFixturef(t, strings.Contains(denial.Reason, "required"), "mandatory HWID policy: %+v err=%v", denial, err)
	}

	firstDevice := httptest.NewRequest(http.MethodGet, "/sub/subscription-token?hwid=device-one", nil)
	{
		_, denial, err := app.prepareSubscriptionDelivery(firstDevice, "subscription-token", false)
		requireFixturef(t, err == nil, "first device denied: %+v err=%v", denial, err)
		requireFixturef(t, !(denial.denied()), "first device denied: %+v err=%v", denial, err)
	}

	secondDevice := httptest.NewRequest(http.MethodGet, "/sub/subscription-token?hwid=device-two", nil)
	{
		_, denial, err := app.prepareSubscriptionDelivery(secondDevice, "subscription-token", false)
		requireFixturef(t, err == nil, "device limit not enforced: %+v err=%v", denial, err)
		requireFixturef(t, denial.Code == http.StatusForbidden, "device limit not enforced: %+v err=%v", denial, err)
		requireFixturef(t, denial.Status == "limited", "device limit not enforced: %+v err=%v", denial, err)
	}

	invalidDevice := httptest.NewRequest(http.MethodGet, "/sub/subscription-token?hwid="+strings.Repeat("x", 129), nil)
	{
		_, denial, err := app.prepareSubscriptionDelivery(invalidDevice, "subscription-token", false)
		requireFixturef(t, err == nil, "invalid HWID policy: %+v err=%v", denial, err)
		requireFixturef(t, denial.Code == http.StatusForbidden, "invalid HWID policy: %+v err=%v", denial, err)
		requireFixturef(t, denial.Reason == "invalid HWID", "invalid HWID policy: %+v err=%v", denial, err)
	}

}

func TestEffectiveSettingsInheritanceAndDenialHeaders(t *testing.T) {
	app := newIntegrationApp(t)
	seedSubscriptionUser(t, app, model.UserStatusActive)
	ctx, err := app.loadSubscriptionContext("subscription-token")
	requireFixture(t, err == nil, err)

	requireFixturef(t, ctx.Settings.Title == "Personal title", "user overrides ignored: %#v", ctx.Settings)
	requireFixturef(t, ctx.Settings.RefreshHours == 24, "user overrides ignored: %#v", ctx.Settings)
	requireFixturef(t, ctx.Settings.InfoURL == "https://user.example/info", "user overrides ignored: %#v", ctx.Settings)

	{
		_, err := app.db.Exec(`
		UPDATE users SET subscription_name = NULL, subscription_refresh_hours = 0, subscription_info_url = NULL
		WHERE subscription_id = 'subscription-token'
	`)
		requireFixture(t, err == nil, err)
	}

	ctx, err = app.loadSubscriptionContext("subscription-token")
	requireFixture(t, err == nil, err)

	requireFixturef(t, ctx.Settings.Title == "AllKeys", "global inheritance failed: %#v", ctx.Settings)
	requireFixturef(t, ctx.Settings.RefreshHours == 12, "global inheritance failed: %#v", ctx.Settings)

	recorder := httptest.NewRecorder()
	writeSubscriptionDenial(recorder, deny(http.StatusGone, "expired", "expired"))
	requireFixturef(t, recorder.Code == http.StatusGone, "denial headers missing: code=%d headers=%v", recorder.Code, recorder.Header())
	requireFixturef(t, recorder.Header().Get("Subscription-Status") == "expired", "denial headers missing: code=%d headers=%v", recorder.Code, recorder.Header())

}
