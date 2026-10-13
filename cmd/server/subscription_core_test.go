package main

import (
	"github.com/romanpodg/SubShare-Go/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPatchSubscriptionPreservesOmittedOverridesAndClearsNull(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/users/1/subscription", strings.NewReader(`{"status":"paused"}`))
	request.SetPathValue("id", "1")
	recorder := httptest.NewRecorder()
	app.apiV1PatchUserSubscription(recorder, request)
	requireFixturef(t, recorder.Code == http.StatusOK, "patch status = %d, body=%s", recorder.Code, recorder.Body.String())

	var status, title, infoURL string
	var refresh int
	{
		err := app.db.QueryRow(`
		SELECT status, subscription_name, subscription_refresh_hours, subscription_info_url
		  FROM users WHERE id = ?
	`, userID).Scan(&status, &title, &refresh, &infoURL)
		requireFixturef(t, err == nil, "load patched user: %v", err)
	}

	requireFixturef(t, status == model.UserStatusPaused, "omitted overrides were changed: status=%q title=%q refresh=%d info=%q", status, title, refresh, infoURL)
	requireFixturef(t, title == "Personal title", "omitted overrides were changed: status=%q title=%q refresh=%d info=%q", status, title, refresh, infoURL)
	requireFixturef(t, refresh == 24, "omitted overrides were changed: status=%q title=%q refresh=%d info=%q", status, title, refresh, infoURL)
	requireFixturef(t, infoURL == "https://user.example/info", "omitted overrides were changed: status=%q title=%q refresh=%d info=%q", status, title, refresh, infoURL)

	request = httptest.NewRequest(http.MethodPatch, "/api/v1/users/1/subscription", strings.NewReader(`{"subscription_name":null,"subscription_refresh_hours":null}`))
	request.SetPathValue("id", "1")
	recorder = httptest.NewRecorder()
	app.apiV1PatchUserSubscription(recorder, request)
	requireFixturef(t, recorder.Code == http.StatusOK, "clear patch status = %d, body=%s", recorder.Code, recorder.Body.String())

	var nullName, nullRefresh any
	{
		err := app.db.QueryRow(`SELECT subscription_name, subscription_refresh_hours FROM users WHERE id = ?`, userID).Scan(&nullName, &nullRefresh)
		requireFixturef(t, err == nil, "load cleared overrides: %v", err)
	}

	requireFixturef(t, nullName == nil, "null did not clear overrides: name=%#v refresh=%#v", nullName, nullRefresh)
	requireFixturef(t, nullRefresh == int64(0), "null did not clear overrides: name=%#v refresh=%#v", nullName, nullRefresh)

}

func TestPatchSubscriptionStoresUserTimezoneInputAsUTC(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	{
		_, err := app.db.Exec(`UPDATE users SET time_zone = 'Asia/Omsk' WHERE id = ?`, userID)
		requireFixture(t, err == nil, err)
	}

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/users/1/subscription", strings.NewReader(`{"starts_at":"2026-07-29 12:00"}`))
	request.SetPathValue("id", "1")
	recorder := httptest.NewRecorder()
	app.apiV1PatchUserSubscription(recorder, request)
	requireFixturef(t, recorder.Code == http.StatusOK, "patch status=%d body=%s", recorder.Code, recorder.Body.String())

	var stored time.Time
	{
		err := app.db.QueryRow(`SELECT starts_at FROM users WHERE id = ?`, userID).Scan(&stored)
		requireFixture(t, err == nil, err)
	}

	{
		got := stored.UTC().Format(time.RFC3339)
		requireFixturef(t, got == "2026-07-29T06:00:00Z", "stored UTC=%s", got)
	}

}
