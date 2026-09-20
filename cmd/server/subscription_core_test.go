package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

func seedSubscriptionUser(t *testing.T, app *App, status string) int64 {
	t.Helper()
	result, err := app.db.Exec(`
		INSERT INTO users(
			name, email, token, activation_code, subscription_id, status,
			starts_at, expires_at, max_devices, subscription_name,
			subscription_refresh_hours, subscription_info_url
		) VALUES('Alice', 'alice@example.test', 'legacy-token', 'activation-token',
		         'subscription-token', ?, ?, ?, 1, 'Personal title', 24,
		         'https://user.example/info')
	`, status, time.Now().Add(-time.Hour).UTC(), time.Now().Add(time.Hour).UTC())
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

func TestPatchSubscriptionPreservesOmittedOverridesAndClearsNull(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/users/1/subscription", strings.NewReader(`{"status":"paused"}`))
	request.SetPathValue("id", "1")
	recorder := httptest.NewRecorder()
	app.apiV1PatchUserSubscription(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body=%s", recorder.Code, recorder.Body.String())
	}

	var status, title, infoURL string
	var refresh int
	if err := app.db.QueryRow(`
		SELECT status, subscription_name, subscription_refresh_hours, subscription_info_url
		  FROM users WHERE id = ?
	`, userID).Scan(&status, &title, &refresh, &infoURL); err != nil {
		t.Fatalf("load patched user: %v", err)
	}
	if status != model.UserStatusPaused || title != "Personal title" || refresh != 24 || infoURL != "https://user.example/info" {
		t.Fatalf("omitted overrides were changed: status=%q title=%q refresh=%d info=%q", status, title, refresh, infoURL)
	}

	request = httptest.NewRequest(http.MethodPatch, "/api/v1/users/1/subscription", strings.NewReader(`{"subscription_name":null,"subscription_refresh_hours":null}`))
	request.SetPathValue("id", "1")
	recorder = httptest.NewRecorder()
	app.apiV1PatchUserSubscription(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("clear patch status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var nullName, nullRefresh any
	if err := app.db.QueryRow(`SELECT subscription_name, subscription_refresh_hours FROM users WHERE id = ?`, userID).Scan(&nullName, &nullRefresh); err != nil {
		t.Fatalf("load cleared overrides: %v", err)
	}
	if nullName != nil || nullRefresh != int64(0) {
		t.Fatalf("null did not clear overrides: name=%#v refresh=%#v", nullName, nullRefresh)
	}
}

func TestPatchSubscriptionStoresUserTimezoneInputAsUTC(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	if _, err := app.db.Exec(`UPDATE users SET time_zone = 'Asia/Omsk' WHERE id = ?`, userID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/users/1/subscription", strings.NewReader(`{"starts_at":"2026-07-29 12:00"}`))
	request.SetPathValue("id", "1")
	recorder := httptest.NewRecorder()
	app.apiV1PatchUserSubscription(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var stored time.Time
	if err := app.db.QueryRow(`SELECT starts_at FROM users WHERE id = ?`, userID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if got := stored.UTC().Format(time.RFC3339); got != "2026-07-29T06:00:00Z" {
		t.Fatalf("stored UTC=%s", got)
	}
}

func TestKeyAssignmentModesControlFutureKeys(t *testing.T) {
	app := newIntegrationApp(t)
	allUser := seedSubscriptionUser(t, app, model.UserStatusActive)
	selectedResult, err := app.db.Exec(`
		INSERT INTO users(name, token, activation_code, subscription_id, status, key_assignment_mode)
		VALUES('Selected', 'selected-token', 'selected-code', 'selected-sub', 'active', 'selected')
	`)
	if err != nil {
		t.Fatalf("seed selected user: %v", err)
	}
	selectedUser, _ := selectedResult.LastInsertId()

	body := `{"label":"new","url":"vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls","status":"active","kind":"real"}`
	recorder := httptest.NewRecorder()
	app.keys().legacy.CreateKey(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/keys", strings.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("create key status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	var allCount, selectedCount int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = ?`, allUser).Scan(&allCount); err != nil {
		t.Fatal(err)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = ?`, selectedUser).Scan(&selectedCount); err != nil {
		t.Fatal(err)
	}
	if allCount != 1 || selectedCount != 0 {
		t.Fatalf("future assignment mismatch: all=%d selected=%d", allCount, selectedCount)
	}
}

func TestUpdateKeyAssignmentAllSelectedAndValidation(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	activeID, activeKey, _ := app.profileKeyring.GetActiveEncryptionKey()
	_, bikKey, _ := app.profileKeyring.GetActiveBlindIndexKey()

	first, err := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, status) VALUES('one', ?, 'active')`, profilestorage.ComputeBlindIndex(bikKey, "vless://assignment-one"))
	if err != nil {
		t.Fatal(err)
	}
	firstID, _ := first.LastInsertId()
	env1, _ := profilestorage.Encrypt([]byte("vless://assignment-one"), activeID, activeKey, firstID)
	_, _ = app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, firstID, env1)

	second, err := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, status) VALUES('two', ?, 'active')`, profilestorage.ComputeBlindIndex(bikKey, "vless://assignment-two"))
	if err != nil {
		t.Fatal(err)
	}
	secondID, _ := second.LastInsertId()
	env2, _ := profilestorage.Encrypt([]byte("vless://assignment-two"), activeID, activeKey, secondID)
	_, _ = app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, secondID, env2)

	if err := app.updateUserKeyAssignment(userID, model.KeyAssignmentModeAll, nil); err != nil {
		t.Fatalf("assign all: %v", err)
	}
	var count int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = ?`, userID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("all assignment count=%d err=%v", count, err)
	}
	if err := app.updateUserKeyAssignment(userID, model.KeyAssignmentModeSelected, []int64{secondID, secondID}); err != nil {
		t.Fatalf("assign selected: %v", err)
	}
	var selectedID int64
	if err := app.db.QueryRow(`SELECT key_id FROM user_keys WHERE user_id = ?`, userID).Scan(&selectedID); err != nil || selectedID != secondID {
		t.Fatalf("selected assignment=%d err=%v", selectedID, err)
	}
	if err := app.updateUserKeyAssignment(userID, "invalid", []int64{firstID}); err == nil {
		t.Fatal("invalid mode was accepted")
	}
	if err := app.updateUserKeyAssignment(userID, model.KeyAssignmentModeSelected, []int64{999999}); !errors.Is(err, errAssignmentKeyNotFound) {
		t.Fatalf("missing key error=%v", err)
	}
}

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
			_, _ = app.db.Exec(`UPDATE users SET status = 'paused' WHERE subscription_id = 'subscription-token'`)
		}, wantCode: http.StatusForbidden, wantStatus: "paused", wantReason: "paused"},
		{name: "blocked reason", subID: "subscription-token", setup: func(t *testing.T, app *App) {
			_, _ = app.db.Exec(`UPDATE users SET status = 'blocked', blocked_reason = 'billing hold' WHERE subscription_id = 'subscription-token'`)
		}, wantCode: http.StatusForbidden, wantStatus: "blocked", wantReason: "billing hold"},
		{name: "future", subID: "subscription-token", setup: func(t *testing.T, app *App) {
			_, _ = app.db.Exec(`UPDATE users SET starts_at = ? WHERE subscription_id = 'subscription-token'`, time.Now().Add(time.Hour).UTC())
		}, wantCode: http.StatusForbidden, wantStatus: "future", wantReason: "not active"},
		{name: "expired", subID: "subscription-token", setup: func(t *testing.T, app *App) {
			_, _ = app.db.Exec(`UPDATE users SET expires_at = ? WHERE subscription_id = 'subscription-token'`, time.Now().Add(-time.Hour).UTC())
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
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			if denial.Code != test.wantCode || denial.Status != test.wantStatus || !strings.Contains(denial.Reason, test.wantReason) {
				t.Fatalf("denial=%+v", denial)
			}
		})
	}
}

func TestSubscriptionDevicePolicyMandatoryOptionalAndLimit(t *testing.T) {
	app := newIntegrationApp(t)
	seedSubscriptionUser(t, app, model.UserStatusActive)
	optional := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
	if _, denial, err := app.prepareSubscriptionDelivery(optional, "subscription-token", false); err != nil || denial.denied() {
		t.Fatalf("optional request denied: %+v err=%v", denial, err)
	}

	if _, err := app.db.Exec(`UPDATE subscription_settings SET provider_id = 'ABCDEFGH', happ_mandatory_hwid = 1 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if _, denial, err := app.prepareSubscriptionDelivery(optional, "subscription-token", false); err != nil || denial.Code != http.StatusForbidden || denial.Status != "limited" || !strings.Contains(denial.Reason, "required") {
		t.Fatalf("mandatory HWID policy: %+v err=%v", denial, err)
	}

	firstDevice := httptest.NewRequest(http.MethodGet, "/sub/subscription-token?hwid=device-one", nil)
	if _, denial, err := app.prepareSubscriptionDelivery(firstDevice, "subscription-token", false); err != nil || denial.denied() {
		t.Fatalf("first device denied: %+v err=%v", denial, err)
	}
	secondDevice := httptest.NewRequest(http.MethodGet, "/sub/subscription-token?hwid=device-two", nil)
	if _, denial, err := app.prepareSubscriptionDelivery(secondDevice, "subscription-token", false); err != nil || denial.Code != http.StatusForbidden || denial.Status != "limited" {
		t.Fatalf("device limit not enforced: %+v err=%v", denial, err)
	}
	invalidDevice := httptest.NewRequest(http.MethodGet, "/sub/subscription-token?hwid="+strings.Repeat("x", 129), nil)
	if _, denial, err := app.prepareSubscriptionDelivery(invalidDevice, "subscription-token", false); err != nil || denial.Code != http.StatusForbidden || denial.Reason != "invalid HWID" {
		t.Fatalf("invalid HWID policy: %+v err=%v", denial, err)
	}
}

func TestEffectiveSettingsInheritanceAndDenialHeaders(t *testing.T) {
	app := newIntegrationApp(t)
	seedSubscriptionUser(t, app, model.UserStatusActive)
	ctx, err := app.loadSubscriptionContext("subscription-token")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Settings.Title != "Personal title" || ctx.Settings.RefreshHours != 24 || ctx.Settings.InfoURL != "https://user.example/info" {
		t.Fatalf("user overrides ignored: %#v", ctx.Settings)
	}
	if _, err := app.db.Exec(`
		UPDATE users SET subscription_name = NULL, subscription_refresh_hours = 0, subscription_info_url = NULL
		WHERE subscription_id = 'subscription-token'
	`); err != nil {
		t.Fatal(err)
	}
	ctx, err = app.loadSubscriptionContext("subscription-token")
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Settings.Title != "AllKeys" || ctx.Settings.RefreshHours != 12 {
		t.Fatalf("global inheritance failed: %#v", ctx.Settings)
	}

	recorder := httptest.NewRecorder()
	writeSubscriptionDenial(recorder, deny(http.StatusGone, "expired", "expired"))
	if recorder.Code != http.StatusGone || recorder.Header().Get("Subscription-Status") != "expired" {
		t.Fatalf("denial headers missing: code=%d headers=%v", recorder.Code, recorder.Header())
	}
}

func TestResponseRuleBlockPrecedesBrowserFallbackAcrossSubBody(t *testing.T) {
	app := newIntegrationApp(t)
	seedSubscriptionUser(t, app, model.UserStatusActive)
	if _, err := app.db.Exec(`
		INSERT INTO response_rules(
			name, enabled, priority, operator, conditions_json, response_type, headers_json
		) VALUES('block browser', 1, 1, 'AND',
		         '[{"headerName":"user-agent","operator":"CONTAINS","value":"mozilla","caseSensitive":false}]',
		         'block', '[]')
	`); err != nil {
		t.Fatalf("insert rule: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token/subbody", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	recorder := httptest.NewRecorder()
	app.handleSubscriptionSubBody(recorder, request)
	if recorder.Code != http.StatusForbidden || recorder.Header().Get("Subscription-Status") != "blocked" {
		t.Fatalf("block rule bypassed: status=%d subscription-status=%q body=%s",
			recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())
	}
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
			if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Subscription-Status") != "empty" {
				t.Fatalf("status=%d subscription-status=%q body=%s", recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())
			}
		})
	}

	activeID, activeKey, _ := app.profileKeyring.GetActiveEncryptionKey()
	_, bikKey, _ := app.profileKeyring.GetActiveBlindIndexKey()
	edgeURI := "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls"
	keyResult, err := app.db.Exec(`
		INSERT INTO vless_keys(label, url_blind_index, status, key_kind, health_failure_count)
		VALUES('edge', ?, 'active', 'real', 0)
	`, profilestorage.ComputeBlindIndex(bikKey, edgeURI))
	if err != nil {
		t.Fatal(err)
	}
	keyID, _ := keyResult.LastInsertId()
	secEnv, _ := profilestorage.Encrypt([]byte(edgeURI), activeID, activeKey, keyID)
	if _, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, secEnv); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, keyID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token/subbody/plain", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	recorder := httptest.NewRecorder()
	app.handleSubscriptionSubBodyPlain(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Subscription-Status") != "active" || !strings.Contains(recorder.Body.String(), "vless://") {
		t.Fatalf("status=%d subscription-status=%q body=%s", recorder.Code, recorder.Header().Get("Subscription-Status"), recorder.Body.String())
	}
}

func TestPrepareDeliveryRuleNotFoundNoMatchAndInvalidRule(t *testing.T) {
	t.Run("not-found", func(t *testing.T) {
		app := newIntegrationApp(t)
		seedSubscriptionUser(t, app, model.UserStatusActive)
		if _, err := app.db.Exec(`
			INSERT INTO response_rules(name, enabled, priority, operator, conditions_json, response_type, headers_json)
			VALUES('hide', 1, 0, 'AND', '[]', 'not-found', '[]')
		`); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		_, denial, err := app.prepareSubscriptionDelivery(request, "subscription-token", true)
		if err != nil || denial.Code != http.StatusNotFound || denial.Status != "not-found" {
			t.Fatalf("denial=%+v err=%v", denial, err)
		}
	})

	t.Run("no-match", func(t *testing.T) {
		app := newIntegrationApp(t)
		seedSubscriptionUser(t, app, model.UserStatusActive)
		if _, err := app.db.Exec(`UPDATE response_rules SET enabled = 0`); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		prepared, denial, err := app.prepareSubscriptionDelivery(request, "subscription-token", true)
		if err != nil || denial.denied() || prepared.Rule != nil {
			t.Fatalf("denial=%+v rule=%#v err=%v", denial, prepared.Rule, err)
		}
	})

	t.Run("invalid rule is disabled and audited", func(t *testing.T) {
		app := newIntegrationApp(t)
		seedSubscriptionUser(t, app, model.UserStatusActive)
		result, err := app.db.Exec(`
			INSERT INTO response_rules(name, enabled, priority, operator, conditions_json, response_type, headers_json)
			VALUES('broken', 1, 0, 'AND', '{', 'plain', '[]')
		`)
		if err != nil {
			t.Fatal(err)
		}
		ruleID, _ := result.LastInsertId()
		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		if _, _, err := app.prepareSubscriptionDelivery(request, "subscription-token", true); err == nil {
			t.Fatal("invalid persisted rule did not fail closed")
		}
		var enabled, audits int
		if err := app.db.QueryRow(`SELECT enabled FROM response_rules WHERE id = ?`, ruleID).Scan(&enabled); err != nil {
			t.Fatal(err)
		}
		if err := app.db.QueryRow(`
			SELECT COUNT(*) FROM audit_events
			WHERE action = 'response_rule.disabled_invalid' AND target_id = ?
		`, strconv.FormatInt(ruleID, 10)).Scan(&audits); err != nil {
			t.Fatal(err)
		}
		if enabled != 0 || audits != 1 {
			t.Fatalf("enabled=%d audits=%d", enabled, audits)
		}
	})
}

func TestRenderersPreserveVMessTrojanAndXrayOutbounds(t *testing.T) {
	vmessPayload, _ := json.Marshal(map[string]any{
		"v": "2", "ps": "vmess", "add": "vmess.example", "port": "443",
		"id": "22222222-2222-2222-2222-222222222222", "aid": "0",
		"net": "ws", "path": "/socket", "host": "cdn.example", "tls": "tls", "sni": "vmess.example",
	})
	vmess := "vmess://" + base64.StdEncoding.EncodeToString(vmessPayload)
	trojan := "trojan://secret@trojan.example:443?security=tls&sni=trojan.example&type=grpc&serviceName=edge#trojan"
	xray := `{"outbounds":[{"protocol":"vless","tag":"xray-vless","settings":{"vnext":[{"address":"xray.example","port":443,"users":[{"id":"33333333-3333-3333-3333-333333333333","encryption":"none"}]}]},"streamSettings":{"network":"grpc","security":"reality","realitySettings":{"serverName":"xray.example","publicKey":"public","shortId":"abcd"}}}]}`
	raw := strings.Join([]string{vmess, trojan, xray}, "\n")

	for name, render := range map[string]func(string) (string, error){
		"mihomo":   renderMihomoSubscription,
		"sing-box": renderSingBoxSubscription,
	} {
		t.Run(name, func(t *testing.T) {
			rendered, err := render(raw)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			for _, expected := range []string{"vmess", "trojan", "xray-vless"} {
				if !strings.Contains(rendered, expected) {
					t.Fatalf("%s disappeared from output: %s", expected, rendered)
				}
			}
		})
	}
}

func TestStrictJSONRejectsUnknownTrailingAndOversizedBodies(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"unknown", `{"mode":"all","surprise":true}`},
		{"trailing", `{"mode":"all"} {"mode":"selected"}`},
		{"oversized", `{"mode":"` + strings.Repeat("x", 1024) + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var request model.UpdateKeyAssignmentRequest
			err := httpapi.ReadJSONWithLimit(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/", bytes.NewBufferString(test.body)), &request, 128)
			if err == nil {
				t.Fatal("invalid JSON body was accepted")
			}
		})
	}
}

func TestPublicPageEscapesScriptCSSAndUnsafeURLs(t *testing.T) {
	cfg := defaultSubscriptionPageConfig()
	cfg.Theme["pageBackground"] = `red;} </style><script>alert(1)</script>`
	cfg.Blocks[0].Logo.Src = `javascript:alert(1)`
	cfg.Blocks[1].Steps[0].Block.Buttons[0].Href = `javascript:alert(2)`
	htmlDocument := renderSubscriptionPageHTML(
		cfg,
		model.PanelSettings{PanelTitle: `</script><script>alert(3)</script>`},
		`</title><script>alert(4)</script>`,
		`javascript:alert(5)`,
		`javascript:alert(6)`,
		`https://example.test/sub/</script><script>alert(7)</script>`,
		`happ://add/</script><script>alert(8)</script>`,
	)
	if strings.Contains(htmlDocument, "<script>alert(") || strings.Contains(htmlDocument, "javascript:") {
		t.Fatalf("unsafe content reached page: %s", htmlDocument)
	}
	if !strings.Contains(htmlDocument, `\u003c/script\u003e`) {
		t.Fatalf("inline JSON was not safely encoded: %s", htmlDocument)
	}
}
