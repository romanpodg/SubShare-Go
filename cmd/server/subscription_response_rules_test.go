package main

import (
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestResponseRuleBlockPrecedesBrowserFallbackAcrossSubBody(t *testing.T) {
	app := newIntegrationApp(t)
	seedSubscriptionUser(t, app, model.UserStatusActive)
	{
		_, err := app.db.Exec(`
		INSERT INTO response_rules(
			name, enabled, priority, operator, conditions_json, response_type, headers_json
		) VALUES('block browser', 1, 1, 'AND',
		         '[{"headerName":"user-agent","operator":"CONTAINS","value":"mozilla","caseSensitive":false}]',
		         'block', '[]')
	`)
		requireFixturef(t, err == nil, "insert rule: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token/subbody", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	recorder := httptest.NewRecorder()
	app.handleSubscriptionSubBody(recorder, request)
	requireFixturef(t, recorder.Code == http.StatusForbidden, "block rule bypassed: status=%d subscription-status=%q body=%s", recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())
	requireFixturef(t, recorder.Header().Get("Subscription-Status") == "blocked", "block rule bypassed: status=%d subscription-status=%q body=%s", recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())

}

func TestSubBodyAdaptersReturn503WhenEmptyAndActiveHeadersWhenAvailable(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	for _, handler := range []struct {
		name string
		run  func(http.ResponseWriter, *http.Request)
		path string
	}{
		{"base64", app.handleSubscriptionSubBody, "/sub/subscription-token/subbody"},
		{"plain", app.handleSubscriptionSubBodyPlain, "/sub/subscription-token/subbody/plain"},
	} {
		t.Run(handler.name+" empty", func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, handler.path, nil)
			request.SetPathValue("subscription_id", "subscription-token")
			recorder := httptest.NewRecorder()
			handler.run(recorder, request)
			requireFixturef(t, recorder.Code == http.StatusServiceUnavailable, "status=%d subscription-status=%q body=%s", recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())
			requireFixturef(t, recorder.Header().Get("Subscription-Status") == "empty", "status=%d subscription-status=%q body=%s", recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())

		})
	}

	activeID, activeKey, err := app.profileKeyring.GetActiveEncryptionKey()
	requireFixturef(t, err == nil, "fixture GetActiveEncryptionKey: %v", err)

	_, bikKey, err := app.profileKeyring.GetActiveBlindIndexKey()
	requireFixturef(t, err == nil, "fixture GetActiveBlindIndexKey: %v", err)

	edgeURI := "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls"
	keyResult, err := app.db.Exec(`
		INSERT INTO vless_keys(label, url_blind_index, status, key_kind, health_failure_count)
		VALUES('edge', ?, 'active', 'real', 0)
	`, profilestorage.ComputeBlindIndex(bikKey, edgeURI))
	requireFixture(t, err == nil, err)

	keyID, err := keyResult.LastInsertId()
	requireFixturef(t, err == nil, "fixture LastInsertId: %v", err)

	secEnv, err := profilestorage.Encrypt([]byte(edgeURI), activeID, activeKey, keyID)
	requireFixturef(t, err == nil, "fixture Encrypt: %v", err)

	{
		_, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, secEnv)
		requireFixture(t, err == nil, err)
	}

	{
		_, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, keyID)
		requireFixture(t, err == nil, err)
	}

	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token/subbody/plain", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	recorder := httptest.NewRecorder()
	app.handleSubscriptionSubBodyPlain(recorder, request)
	requireFixturef(t, recorder.Code == http.StatusOK, "status=%d subscription-status=%q body=%s", recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())
	requireFixturef(t, recorder.Header().Get("Subscription-Status") == "active", "status=%d subscription-status=%q body=%s", recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())
	requireFixturef(t, strings.Contains(recorder.Body.String(), "vless://"), "status=%d subscription-status=%q body=%s", recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())

}

func TestPrepareDeliveryRuleNotFoundNoMatchAndInvalidRule(t *testing.T) {
	t.Run("not-found", func(t *testing.T) {
		app := newIntegrationApp(t)
		seedSubscriptionUser(t, app, model.UserStatusActive)
		{
			_, err := app.db.Exec(`
			INSERT INTO response_rules(name, enabled, priority, operator, conditions_json, response_type, headers_json)
			VALUES('hide', 1, 0, 'AND', '[]', 'not-found', '[]')
		`)
			requireFixture(t, err == nil, err)
		}

		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		_, denial, err := app.prepareSubscriptionDelivery(request, "subscription-token", true)
		requireFixturef(t, err == nil, "denial=%+v err=%v", denial, err)
		requireFixturef(t, denial.Code == http.StatusNotFound, "denial=%+v err=%v", denial, err)
		requireFixturef(t, denial.Status == "not-found", "denial=%+v err=%v", denial, err)

	})

	t.Run("no-match", func(t *testing.T) {
		app := newIntegrationApp(t)
		seedSubscriptionUser(t, app, model.UserStatusActive)
		{
			_, err := app.db.Exec(`UPDATE response_rules SET enabled = 0`)
			requireFixture(t, err == nil, err)
		}

		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		prepared, denial, err := app.prepareSubscriptionDelivery(request, "subscription-token", true)
		requireFixturef(t, err == nil, "denial=%+v rule=%#v err=%v", denial, prepared.Rule, err)
		requireFixturef(t, !(denial.denied()), "denial=%+v rule=%#v err=%v", denial, prepared.Rule, err)
		requireFixturef(t, prepared.Rule == nil, "denial=%+v rule=%#v err=%v", denial, prepared.Rule, err)

	})

	t.Run("invalid rule is disabled and audited", func(t *testing.T) {
		app := newIntegrationApp(t)
		seedSubscriptionUser(t, app, model.UserStatusActive)
		result, err := app.db.Exec(`
			INSERT INTO response_rules(name, enabled, priority, operator, conditions_json, response_type, headers_json)
			VALUES('broken', 1, 0, 'AND', '{', 'plain', '[]')
		`)
		requireFixture(t, err == nil, err)

		ruleID, err := result.LastInsertId()
		requireFixturef(t, err == nil, "fixture LastInsertId: %v", err)

		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		{
			_, _, err := app.prepareSubscriptionDelivery(request, "subscription-token", true)
			requireFixture(t, err != nil, "invalid persisted rule did not fail closed")
		}

		var enabled, audits int
		{
			err := app.db.QueryRow(`SELECT enabled FROM response_rules WHERE id = ?`, ruleID).Scan(&enabled)
			requireFixture(t, err == nil, err)
		}

		{
			err := app.db.QueryRow(`
			SELECT COUNT(*) FROM audit_events
			WHERE action = 'response_rule.disabled_invalid' AND target_id = ?
		`, strconv.FormatInt(ruleID, 10)).Scan(&audits)
			requireFixture(t, err == nil, err)
		}

		requireFixturef(t, enabled == 0, "enabled=%d audits=%d", enabled, audits)
		requireFixturef(t, audits == 1, "enabled=%d audits=%d", enabled, audits)

	})
}
