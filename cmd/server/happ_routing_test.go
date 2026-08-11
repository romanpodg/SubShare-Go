package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

const testHappRoutingConfig = `{"Name":"Маршрутизация SubShare","GlobalProxy":false,"BypassSites":["пример.рф"],"FutureField":{"enabled":true}}`

func decodeHappRoutingPayload(t *testing.T, link string) string {
	t.Helper()
	parts := strings.Split(link, "/")
	if len(parts) == 0 {
		t.Fatalf("invalid routing link %q", link)
	}
	escaped, err := url.PathUnescape(parts[len(parts)-1])
	if err != nil {
		t.Fatalf("unescape routing payload: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(escaped)
	if err != nil {
		t.Fatalf("decode routing payload: %v", err)
	}
	return string(decoded)
}

func TestBuildHappRoutingLink(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{routingDeliveryModeAdd, routingDeliveryModeOnAdd} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			first, err := buildHappRoutingLink(testHappRoutingConfig, mode)
			if err != nil {
				t.Fatalf("build link: %v", err)
			}
			second, err := buildHappRoutingLink(testHappRoutingConfig, mode)
			if err != nil || second != first {
				t.Fatalf("builder is not deterministic: first=%q second=%q err=%v", first, second, err)
			}
			if !strings.HasPrefix(first, "happ://routing/"+mode+"/") {
				t.Fatalf("link=%q", first)
			}
			if got := decodeHappRoutingPayload(t, first); got != testHappRoutingConfig {
				t.Fatalf("decoded payload=%q want=%q", got, testHappRoutingConfig)
			}
		})
	}
	if happRoutingOffURL != "happ://routing/off" {
		t.Fatalf("off URL=%q", happRoutingOffURL)
	}
}

func TestValidateHappRoutingConfig(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		config      string
		requireName bool
		wantError   bool
	}{
		{name: "valid automatic profile", config: testHappRoutingConfig, requireName: true},
		{name: "unknown fields remain forward compatible", config: `{"Name":"Profile","Unknown":[1,2,3]}`, requireName: true},
		{name: "empty disabled profile", config: "", requireName: false},
		{name: "missing automatic name", config: `{"GlobalProxy":false}`, requireName: true, wantError: true},
		{name: "null object", config: `null`, wantError: true},
		{name: "wrong known boolean", config: `{"Name":"Profile","GlobalProxy":"yes"}`, requireName: true, wantError: true},
		{name: "wrong known list", config: `{"Name":"Profile","DirectSites":[1]}`, requireName: true, wantError: true},
		{name: "wrong DNS host value", config: `{"Name":"Profile","DnsHosts":{"example.com":1}}`, requireName: true, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateHappRoutingConfig(test.config, test.requireName)
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v wantError=%v", err, test.wantError)
			}
		})
	}
}

func TestRoutingSettingsAPIRoundTripAndLegacyUpdate(t *testing.T) {
	app := newIntegrationApp(t)

	for _, mode := range []string{routingDeliveryModeDisabled, routingDeliveryModeAdd, routingDeliveryModeOnAdd} {
		body, _ := json.Marshal(model.UpdateRoutingSettingsRequest{ConfigJSON: testHappRoutingConfig, DeliveryMode: &mode})
		recorder := httptest.NewRecorder()
		app.apiUpdateRoutingSettings(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/routing-settings", bytes.NewReader(body)))
		if recorder.Code != http.StatusOK {
			t.Fatalf("mode=%s status=%d body=%q", mode, recorder.Code, recorder.Body.String())
		}
		var response model.RoutingSettings
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.ConfigJSON != testHappRoutingConfig || response.DeliveryMode != mode {
			t.Fatalf("mode=%s response=%#v", mode, response)
		}
		if response.Message != "routing settings updated" {
			t.Fatalf("legacy update message missing: %#v", response)
		}
		if response.AddURL == "" || response.OnAddURL == "" || response.OffURL != happRoutingOffURL {
			t.Fatalf("canonical manual links missing: %#v", response)
		}
		getRecorder := httptest.NewRecorder()
		app.apiGetRoutingSettings(getRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/routing-settings", nil))
		if getRecorder.Code != http.StatusOK {
			t.Fatalf("mode=%s GET status=%d body=%q", mode, getRecorder.Code, getRecorder.Body.String())
		}
		var loaded model.RoutingSettings
		if err := json.Unmarshal(getRecorder.Body.Bytes(), &loaded); err != nil || loaded.DeliveryMode != mode || loaded.ConfigJSON != testHappRoutingConfig || loaded.Message != "" {
			t.Fatalf("mode=%s GET response=%#v err=%v", mode, loaded, err)
		}
	}

	legacyConfig := `{"Name":"Legacy client update"}`
	recorder := httptest.NewRecorder()
	app.apiUpdateRoutingSettings(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/routing-settings", strings.NewReader(`{"config_json":`+strconvQuote(legacyConfig)+`}`)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("legacy update status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	settings, err := app.getRoutingSettings()
	if err != nil || settings.ConfigJSON != legacyConfig || settings.DeliveryMode != routingDeliveryModeOnAdd {
		t.Fatalf("legacy update did not preserve mode: settings=%#v err=%v", settings, err)
	}
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestRoutingSettingsAPIRejectsInvalidAutomaticDelivery(t *testing.T) {
	app := newIntegrationApp(t)
	for _, payload := range []string{
		`{"config_json":"{\"GlobalProxy\":false}","delivery_mode":"add"}`,
		`{"config_json":"{\"Name\":\"Profile\",\"GlobalProxy\":\"yes\"}","delivery_mode":"onadd"}`,
		`{"config_json":"{\"Name\":\"Profile\"}","delivery_mode":"other"}`,
	} {
		recorder := httptest.NewRecorder()
		app.apiUpdateRoutingSettings(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/routing-settings", strings.NewReader(payload)))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid payload accepted: status=%d body=%q", recorder.Code, recorder.Body.String())
		}
	}
}

func TestSubscriptionRoutingDeliveryModesAndLegacyHeaderConflicts(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	insertAssignedDeliveryKey(t, app, userID, nil, "VLESS", externalTestVLESS, "vless", "full", 1)

	legacyHeaders, _ := json.Marshal([]responseHeader{
		{Key: "ROUTING", Value: "happ://routing/onadd/legacy-global"},
		{Key: "X-Safe-Global", Value: "global"},
	})
	if _, err := app.db.Exec(`UPDATE subscription_delivery_settings SET response_headers_json = ? WHERE id = 1`, string(legacyHeaders)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`
		INSERT INTO response_rules(name, enabled, priority, operator, conditions_json, response_type, headers_json)
		VALUES('legacy routing override', 1, 0, 'AND', '[]', 'plain', '[{"key":"Routing","value":"happ://routing/add/legacy-rule"},{"key":"X-Safe-Rule","value":"rule"}]')
	`); err != nil {
		t.Fatal(err)
	}

	var baselineBody string
	for _, test := range []struct {
		mode       string
		wantPrefix string
	}{
		{mode: routingDeliveryModeDisabled},
		{mode: routingDeliveryModeAdd, wantPrefix: "happ://routing/add/"},
		{mode: routingDeliveryModeOnAdd, wantPrefix: "happ://routing/onadd/"},
	} {
		if _, err := app.db.Exec(`UPDATE routing_settings SET config_json = ?, delivery_mode = ? WHERE id = 1`, testHappRoutingConfig, test.mode); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		request.SetPathValue("subscription_id", "subscription-token")
		recorder := httptest.NewRecorder()
		app.handleSubscription(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("mode=%s status=%d body=%q", test.mode, recorder.Code, recorder.Body.String())
		}
		got := recorder.Header().Get("routing")
		if test.wantPrefix == "" {
			if got != "" {
				t.Fatalf("disabled mode emitted routing=%q", got)
			}
		} else {
			if !strings.HasPrefix(got, test.wantPrefix) || decodeHappRoutingPayload(t, got) != testHappRoutingConfig {
				t.Fatalf("mode=%s routing=%q", test.mode, got)
			}
		}
		if strings.Contains(got, "/off") {
			t.Fatalf("automatic delivery emitted off link: %q", got)
		}
		if recorder.Header().Get("X-Safe-Global") != "global" || recorder.Header().Get("X-Safe-Rule") != "rule" {
			t.Fatalf("safe headers lost: %#v", recorder.Header())
		}
		if baselineBody == "" {
			baselineBody = recorder.Body.String()
		} else if recorder.Body.String() != baselineBody {
			t.Fatal("routing mode changed the VPN subscription body")
		}
	}
}

func TestCorruptStoredRoutingDoesNotBreakSubscription(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	insertAssignedDeliveryKey(t, app, userID, nil, "VLESS", externalTestVLESS, "vless", "full", 1)
	if _, err := app.db.Exec(`UPDATE routing_settings SET config_json = '{', delivery_mode = 'add' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	recorder := httptest.NewRecorder()
	app.handleSubscription(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Header().Get("routing") != "" || recorder.Body.Len() == 0 {
		t.Fatalf("status=%d routing=%q body=%q", recorder.Code, recorder.Header().Get("routing"), recorder.Body.String())
	}
}

func TestRoutingHeaderIsReservedCaseInsensitively(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"routing", "Routing", "ROUTING"} {
		if safeCustomResponseHeader(key, "override") {
			t.Errorf("reserved routing header %q accepted", key)
		}
		settings := defaultSubscriptionDeliverySettings()
		settings.ResponseHeaders = []responseHeader{{Key: key, Value: "override"}}
		if _, err := validateSubscriptionDeliverySettings(settings); err == nil {
			t.Errorf("global routing header %q accepted", key)
		}
		rule := responseRuleInput{Name: "rule", Operator: "AND", ResponseType: "plain", Headers: []responseHeader{{Key: key, Value: "override"}}}
		if _, err := validateResponseRuleInput(rule); err == nil {
			t.Errorf("rule routing header %q accepted", key)
		}
	}
}
