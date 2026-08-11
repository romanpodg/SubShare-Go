package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestValidateSubscriptionDeliverySettings(t *testing.T) {
	t.Parallel()
	input := defaultSubscriptionDeliverySettings()
	input.ResponseHeaders = []responseHeader{{Key: "X-Provider", Value: "SubShare"}}
	input.Remarks["expired"] = []string{"Subscription expired", "Contact support"}
	if _, err := validateSubscriptionDeliverySettings(input); err != nil {
		t.Fatalf("valid delivery settings rejected: %v", err)
	}

	input.ResponseHeaders = []responseHeader{{Key: "Set-Cookie", Value: "unsafe=true"}}
	if _, err := validateSubscriptionDeliverySettings(input); err == nil {
		t.Fatal("unsafe response header was accepted")
	}

	for _, key := range []string{
		"announce", "Announce", "ANNOUNCE", "profile-title", "profile-update-interval",
		"profile-web-page-url", "support-url", "subscription-userinfo", "routing", "Routing", "ROUTING",
	} {
		input.ResponseHeaders = []responseHeader{{Key: key, Value: "override"}}
		if _, err := validateSubscriptionDeliverySettings(input); err == nil {
			t.Errorf("reserved response header %q was accepted", key)
		}
	}
}

func TestSubscriptionAnnouncementUsesCanonicalMetadata(t *testing.T) {
	tests := []struct {
		name             string
		global           string
		userOverride     string
		legacyDelivery   string
		legacyHeaderName string
		want             string
	}{
		{
			name:             "global UTF-8 metadata",
			global:           "Плановые работы сегодня",
			legacyDelivery:   "deprecated delivery value",
			legacyHeaderName: "ANNOUNCE",
			want:             "Плановые работы сегодня",
		},
		{
			name:             "user override wins",
			global:           "Global announcement",
			userOverride:     "Персональное объявление",
			legacyDelivery:   "deprecated delivery value",
			legacyHeaderName: "Announce",
			want:             "Персональное объявление",
		},
		{
			name:           "empty canonical value emits no header",
			legacyDelivery: "deprecated delivery value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			userID := seedSubscriptionUser(t, app, model.UserStatusActive)
			if _, err := app.db.Exec(`UPDATE subscription_settings SET extra_status = ? WHERE id = 1`, nullStringValue(test.global)); err != nil {
				t.Fatal(err)
			}
			if _, err := app.db.Exec(`UPDATE users SET subscription_extra_status = ? WHERE id = ?`, nullStringValue(test.userOverride), userID); err != nil {
				t.Fatal(err)
			}
			headers := []responseHeader{{Key: "X-Provider", Value: "SubShare"}}
			if test.legacyHeaderName != "" {
				headers = append(headers, responseHeader{Key: test.legacyHeaderName, Value: "custom override"})
			}
			headersJSON, err := json.Marshal(headers)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := app.db.Exec(`UPDATE subscription_delivery_settings SET announcement = ?, response_headers_json = ? WHERE id = 1`, test.legacyDelivery, string(headersJSON)); err != nil {
				t.Fatal(err)
			}
			insertAssignedDeliveryKey(t, app, userID, nil, "VLESS", externalTestVLESS, "vless", "full", 1)

			request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
			request.SetPathValue("subscription_id", "subscription-token")
			recorder := httptest.NewRecorder()
			app.handleSubscription(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
			}
			if got := recorder.Header().Get("X-Provider"); got != "SubShare" {
				t.Fatalf("safe custom header=%q", got)
			}
			got := recorder.Header().Get("announce")
			if test.want == "" {
				if got != "" {
					t.Fatalf("deprecated delivery setting emitted announce=%q", got)
				}
				return
			}
			want := "base64:" + base64.StdEncoding.EncodeToString([]byte(test.want))
			if got != want {
				t.Fatalf("announce=%q want=%q", got, want)
			}
			if values := recorder.Header().Values("announce"); len(values) != 1 {
				t.Fatalf("announce values=%#v want exactly one", values)
			}
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "base64:"))
			if err != nil || string(decoded) != test.want {
				t.Fatalf("decoded announce=%q err=%v", decoded, err)
			}
		})
	}
}

func TestDeliverySettingsLegacyAnnouncementCompatibility(t *testing.T) {
	app := newIntegrationApp(t)
	currentPayload := `{"response_headers":[],"remarks":{"expired":[],"paused":[],"blocked":[],"limited":[],"empty":[]}}`
	recorder := httptest.NewRecorder()
	app.apiV1UpdateSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/subscription-delivery-settings", strings.NewReader(currentPayload)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("current update without announcement status=%d body=%q", recorder.Code, recorder.Body.String())
	}

	payload := `{"response_headers":[{"key":"X-Provider","value":"SubShare"}],"announcement":"Старое объявление","remarks":{"expired":["Expired"],"paused":[],"blocked":[],"limited":[],"empty":[]}}`
	recorder = httptest.NewRecorder()
	app.apiV1UpdateSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/subscription-delivery-settings", strings.NewReader(payload)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("legacy update status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	var canonical, retired string
	if err := app.db.QueryRow(`SELECT COALESCE(extra_status, '') FROM subscription_settings WHERE id = 1`).Scan(&canonical); err != nil {
		t.Fatal(err)
	}
	if err := app.db.QueryRow(`SELECT announcement FROM subscription_delivery_settings WHERE id = 1`).Scan(&retired); err != nil {
		t.Fatal(err)
	}
	if canonical != "Старое объявление" || retired != "" {
		t.Fatalf("canonical=%q retired=%q", canonical, retired)
	}
	if got := app.subscriptionRemarkForStatus("expired", "fallback"); got != "Expired" {
		t.Fatalf("remark=%q", got)
	}

	if _, err := app.db.Exec(`UPDATE subscription_settings SET extra_status = 'Canonical' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	app.apiV1UpdateSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/subscription-delivery-settings", strings.NewReader(strings.Replace(payload, "Старое объявление", "Other", 1))))
	if recorder.Code != http.StatusOK {
		t.Fatalf("second legacy update status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if err := app.db.QueryRow(`SELECT COALESCE(extra_status, '') FROM subscription_settings WHERE id = 1`).Scan(&canonical); err != nil {
		t.Fatal(err)
	}
	if canonical != "Canonical" {
		t.Fatalf("legacy input overwrote canonical value: %q", canonical)
	}

	recorder = httptest.NewRecorder()
	app.apiV1GetSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/subscription-delivery-settings", nil))
	if recorder.Code != http.StatusOK || strings.Contains(recorder.Body.String(), `"announcement"`) {
		t.Fatalf("delivery response still exposes announcement: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestRemarkStatusFromReason(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"subscription expired":               "expired",
		"subscription paused":                "paused",
		"subscription is not active yet":     "future",
		"subscription blocked":               "blocked",
		"maximum number of devices reached":  "limited",
		"subscription has no available keys": "empty",
	}
	for reason, want := range tests {
		if got := remarkStatusFromReason(reason); got != want {
			t.Fatalf("remarkStatusFromReason(%q) = %q, want %q", reason, got, want)
		}
	}
}

func TestSubscriptionUserinfoExpireMergePreservesExistingMetadata(t *testing.T) {
	expire := int64(1787616000)
	got := subscriptionUserinfoWithExpire("upload=10; download=20; total=30; expire=1", &expire)
	if got != "upload=10; download=20; total=30; expire=1787616000" {
		t.Fatalf("merged userinfo=%q", got)
	}
	if got := subscriptionUserinfoWithExpire(got, nil); got != "upload=10; download=20; total=30" {
		t.Fatalf("disabled userinfo=%q", got)
	}
}

func TestGeneralExpirationSettingDefaultsAndOmittedAdminFieldIsPreserved(t *testing.T) {
	app := newIntegrationApp(t)
	settings, err := app.getSubscriptionSettings()
	if err != nil {
		t.Fatalf("read clean settings: %v", err)
	}
	if settings.ShowSubscriptionExpiration {
		t.Fatal("clean database enabled general expiration metadata")
	}
	if _, err := app.db.Exec(`UPDATE subscription_settings SET show_subscription_expiration = 1, happ_notify_expiration = 1 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}

	legacyPayload := `{
		"title":"AllKeys","refresh_hours":12,"info_url":"","extra_url":"","extra_status":"",
		"subscription_format":"links","time_zone":"UTC","language":"ru","provider_id":"",
		"happ_no_limit_mode":false,"happ_no_limit_mode_xhttp_only":false,"happ_mandatory_hwid":false,
		"happ_notify_expiration":true,"happ_hide_server_settings":false,"happ_subscription_body":""
	}`
	request := httptest.NewRequest(http.MethodPut, "/api/v1/subscription-settings", strings.NewReader(legacyPayload))
	recorder := httptest.NewRecorder()
	app.apiUpdateSubscriptionSettings(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("legacy settings update status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	settings, err = app.getSubscriptionSettings()
	if err != nil {
		t.Fatalf("reload settings after legacy update: %v", err)
	}
	if !settings.ShowSubscriptionExpiration || !settings.HappNotifyExpiration {
		t.Fatalf("omitted/new and legacy values were not preserved independently: %#v", settings)
	}

	showExpiration := false
	update := model.UpdateSubscriptionSettingsRequest{
		Title:                      "AllKeys",
		RefreshHours:               12,
		SubscriptionFormat:         model.SubscriptionFormatLinks,
		ShowSubscriptionExpiration: &showExpiration,
		TimeZone:                   "UTC",
		Language:                   "ru",
		HappNotifyExpiration:       true,
	}
	payload, err := json.Marshal(update)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPut, "/api/v1/subscription-settings", bytes.NewReader(payload))
	recorder = httptest.NewRecorder()
	app.apiUpdateSubscriptionSettings(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("explicit settings update status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	settings, err = app.getSubscriptionSettings()
	if err != nil {
		t.Fatalf("reload explicit settings: %v", err)
	}
	if settings.ShowSubscriptionExpiration || !settings.HappNotifyExpiration {
		t.Fatalf("general and Happ values did not persist independently: %#v", settings)
	}
}

func TestSubscriptionExpirationMetadataDelivery(t *testing.T) {
	tests := []struct {
		name           string
		format         string
		enabled        bool
		happEnabled    bool
		expiresAt      any
		wantStatus     int
		wantBaseFields bool
	}{
		{name: "finite link enabled independently of Happ", format: model.SubscriptionFormatLinks, enabled: true, happEnabled: false, expiresAt: time.Date(2026, 8, 25, 12, 30, 0, 0, time.FixedZone("sample", 5*60*60+30*60)).UTC(), wantStatus: http.StatusOK, wantBaseFields: true},
		{name: "finite JSON enabled", format: model.SubscriptionFormatXrayJSON, enabled: true, expiresAt: time.Date(2026, 8, 25, 12, 30, 0, 0, time.FixedZone("sample", 5*60*60+30*60)).UTC(), wantStatus: http.StatusOK, wantBaseFields: true},
		{name: "legacy Happ enabled does not expose expiration", format: model.SubscriptionFormatLinks, enabled: false, happEnabled: true, expiresAt: time.Now().Add(24 * time.Hour).UTC(), wantStatus: http.StatusOK, wantBaseFields: true},
		{name: "unlimited", format: model.SubscriptionFormatLinks, enabled: true, expiresAt: nil, wantStatus: http.StatusOK, wantBaseFields: true},
		{name: "expired authorization remains enforced", format: model.SubscriptionFormatLinks, enabled: true, expiresAt: time.Now().Add(-time.Hour).UTC(), wantStatus: http.StatusGone, wantBaseFields: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			userID := seedSubscriptionUser(t, app, model.UserStatusActive)
			if _, err := app.db.Exec(`UPDATE users SET expires_at = ? WHERE id = ?`, test.expiresAt, userID); err != nil {
				t.Fatal(err)
			}
			if _, err := app.db.Exec(`UPDATE subscription_settings SET subscription_format = ?, show_subscription_expiration = ?, happ_notify_expiration = ? WHERE id = 1`, test.format, boolToInt(test.enabled), boolToInt(test.happEnabled)); err != nil {
				t.Fatal(err)
			}
			insertAssignedDeliveryKey(t, app, userID, nil, "VLESS", externalTestVLESS, "vless", "full", 1)

			request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
			request.SetPathValue("subscription_id", "subscription-token")
			recorder := httptest.NewRecorder()
			app.handleSubscription(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status=%d want=%d body=%q", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			userinfo := recorder.Header().Get("Subscription-Userinfo")
			if !test.wantBaseFields {
				if userinfo != "" {
					t.Fatalf("denied response exposed userinfo=%q", userinfo)
				}
				return
			}
			if test.enabled && test.expiresAt != nil {
				expected := test.expiresAt.(time.Time).Unix()
				if !strings.Contains(userinfo, "expire="+strconv.FormatInt(expected, 10)) {
					t.Fatalf("userinfo=%q missing UTC Unix expire=%d", userinfo, expected)
				}
				if strings.Count(strings.ToLower(userinfo), "expire=") != 1 {
					t.Fatalf("userinfo=%q contains duplicate expire metadata", userinfo)
				}
			} else if strings.Contains(strings.ToLower(userinfo), "expire=") {
				t.Fatalf("userinfo=%q unexpectedly exposes expiration", userinfo)
			}
		})
	}
}
