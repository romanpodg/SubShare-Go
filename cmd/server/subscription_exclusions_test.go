package main

import (
	"bytes"
	"encoding/base64"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestUnsafeAndMalformedStoredRowsAreSafelyExcluded(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: "valid", Raw: deliverySSClean, Protocol: "shadowsocks", Compatibility: "full", SortOrder: 0})
	unsafeSecret := "trojan://never-log-this-password@example.com:443\nss://injected"
	malformedSecret := "hysteria2://never-log-this-auth@host:0"
	insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: "unsafe", Raw: unsafeSecret, Protocol: "trojan", Compatibility: "legacy", SortOrder: 1})
	insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: "malformed", Raw: malformedSecret, Protocol: "hysteria2", Compatibility: "full", SortOrder: 2})

	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	generated, _, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || generated.Body != deliverySSClean || len(generated.Exclusions) != 2 {
		t.Fatalf("safe partial output mismatch exclusions=%d err=%v", len(generated.Exclusions), err)
	}
	codes := exclusionCodes(generated.Exclusions)
	if !codes[delivery.ReasonUnsafeControl] || !codes[delivery.ReasonInvalidStored] {
		t.Fatalf("unexpected exclusion codes: %#v", codes)
	}
	formatted := strings.Join([]string{logs.String(), generated.Exclusions[0].Error(), generated.Exclusions[1].Error()}, " ")
	for _, secret := range []string{"never-log-this-password", "never-log-this-auth", unsafeSecret, malformedSecret} {
		if strings.Contains(formatted, secret) {
			t.Fatal("generation diagnostics leaked credential-bearing material")
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token/subbody/plain", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	recorder := httptest.NewRecorder()
	app.handleSubscriptionSubBodyPlain(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != deliverySSClean {
		t.Fatalf("safe partial handler status=%d or body mismatch", recorder.Code)
	}
	if recorder.Header().Get("SubShare-Excluded-Count") != "2" || !strings.Contains(recorder.Header().Get("SubShare-Exclusion-Codes"), delivery.ReasonUnsafeControl) {
		t.Fatalf("safe exclusion headers: %#v", recorder.Header())
	}
	var auditMetadata string
	if err := app.db.QueryRow(`SELECT COALESCE(GROUP_CONCAT(metadata_json, ''), '') FROM audit_events`).Scan(&auditMetadata); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{unsafeSecret, malformedSecret, deliverySSClean, base64.StdEncoding.EncodeToString([]byte(deliverySSClean))} {
		if strings.Contains(logs.String(), forbidden) || strings.Contains(auditMetadata, forbidden) {
			t.Fatalf("logs or audit contain credential-bearing delivery material")
		}
	}
}

func TestSubscriptionControlValidationRejectsCRLFNULAndDEL(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  string
	}{
		{"cr", "ss://safe\rsecond"},
		{"lf", "ss://safe\nsecond"},
		{"nul", "ss://safe\x00second"},
		{"del", "ss://safe\x7fsecond"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !delivery.HasUnsafeControl(test.raw) {
				t.Fatal("unsafe control was accepted")
			}
		})
	}
}

func TestStructuredDeliveryWithOnlyExcludedProfilesReturnsAllExcludedFailure(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: "TUIC v4", Raw: externalTestTUICV4, Protocol: "tuic", Compatibility: "read_only", SortOrder: 0})

	generated, denial, err := app.generateSelectedSubscription("subscription-token", "mihomo")
	if err != nil {
		t.Fatal(err)
	}
	if denial.Code != http.StatusUnprocessableEntity || denial.Reason != delivery.ReasonAllExcluded || generated.Body != "" || generated.GeneratedCount != 0 {
		t.Fatalf("empty structured result code=%d reason=%q or non-empty body", denial.Code, denial.Reason)
	}
	if len(generated.Exclusions) != 1 || generated.Exclusions[0].Reason != delivery.ReasonCompatibility {
		t.Fatalf("empty structured exclusions = %#v", generated.Exclusions)
	}
}

func TestStructuredDeliveryAllExcludedHandlerPolicy(t *testing.T) {
	tests := []struct {
		name          string
		userAgent     string
		raw           string
		protocol      string
		compatibility string
		reason        string
		forbidden     []string
	}{
		{
			name: "TUIC v5 requested as Xray", userAgent: "xray-core",
			raw: validationTUICBasic, protocol: "tuic", compatibility: "full",
			reason:    delivery.ReasonUnsupportedProtocol,
			forbidden: []string{"synthetic-password", "tuic-basic.example", "TUIC-basic"},
		},
		{
			name: "TUIC v4 requested as Mihomo", userAgent: "mihomo",
			raw: externalTestTUICV4, protocol: "tuic", compatibility: "read_only",
			reason:    delivery.ReasonCompatibility,
			forbidden: []string{"legacy-token-value", "legacy.example", "Legacy"},
		},
		{
			name: "Shadowsocks plugin requested as Xray", userAgent: "xray-core",
			raw: deliverySSPlugin, protocol: "shadowsocks", compatibility: "full",
			reason:    delivery.ReasonPlugin,
			forbidden: []string{"shadow-password", "ss.example", "SS-plugin"},
		},
		{
			name: "Hysteria insecure requested as Xray", userAgent: "xray-core",
			raw: "hy2://synthetic-auth@unsafe-tls.example:443?insecure=1#Unsafe-TLS", protocol: "hysteria2", compatibility: "full",
			reason:    delivery.ReasonUnrepresentable,
			forbidden: []string{"synthetic-auth", "unsafe-tls.example", "Unsafe-TLS"},
		},
		{
			name: "malformed stored profile", userAgent: "xray-core",
			raw: "hysteria2://synthetic-auth@malformed.example:0#Malformed", protocol: "hysteria2", compatibility: "full",
			reason:    delivery.ReasonInvalidStored,
			forbidden: []string{"synthetic-auth", "malformed.example", "Malformed"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			userID := seedSubscriptionUser(t, app, model.UserStatusActive)
			insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: "sensitive display", Raw: test.raw, Protocol: test.protocol, Compatibility: test.compatibility, SortOrder: 0})
			request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
			request.SetPathValue("subscription_id", "subscription-token")
			request.Header.Set("User-Agent", test.userAgent)
			recorder := httptest.NewRecorder()
			app.handleSubscription(recorder, request)

			assertAllExcludedDeliveryResponse(t, recorder, test.reason, append(test.forbidden, test.raw, "sensitive display", "key:"))
		})
	}
}

func TestStructuredDeliveryMixedAndNoEligiblePolicies(t *testing.T) {
	t.Run("mixed supported and excluded", func(t *testing.T) {
		app := newIntegrationApp(t)
		userID := seedSubscriptionUser(t, app, model.UserStatusActive)
		insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: "supported", Raw: deliverySSClean, Protocol: "shadowsocks", Compatibility: "full", SortOrder: 0})
		insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: "excluded", Raw: validationTUICBasic, Protocol: "tuic", Compatibility: "full", SortOrder: 1})

		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		request.SetPathValue("subscription_id", "subscription-token")
		request.Header.Set("User-Agent", "xray-core")
		recorder := httptest.NewRecorder()
		app.handleSubscription(recorder, request)
		if recorder.Code != http.StatusOK || recorder.Header().Get("SubShare-Excluded-Count") != "1" || recorder.Header().Get("SubShare-Exclusion-Codes") != delivery.ReasonUnsupportedProtocol {
			t.Fatalf("mixed response status=%d or exclusion summary mismatch", recorder.Code)
		}
		if err := delivery.ValidateStructuredBody("xray-json", recorder.Body.String()); err != nil {
			t.Fatal("mixed response did not retain its supported Xray profile")
		}
	})

	t.Run("no assigned profiles", func(t *testing.T) {
		app := newIntegrationApp(t)
		seedSubscriptionUser(t, app, model.UserStatusActive)
		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		request.SetPathValue("subscription_id", "subscription-token")
		request.Header.Set("User-Agent", "xray-core")
		recorder := httptest.NewRecorder()
		app.handleSubscription(recorder, request)
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("no-assignment response status=%d, want 503", recorder.Code)
		}
	})
}

func TestExclusionHeadersAreAggregatedDeterministicallyAndBounded(t *testing.T) {
	exclusions := make([]delivery.Exclusion, 0, 1500)
	for index := 0; index < 1000; index++ {
		exclusions = append(exclusions, delivery.Exclusion{Reason: delivery.ReasonUnsupportedProtocol})
	}
	for index := 0; index < 500; index++ {
		exclusions = append(exclusions, delivery.Exclusion{Reason: delivery.ReasonCompatibility})
	}
	first := http.Header{}
	second := http.Header{}
	delivery.ApplyExclusionHeaders(first, exclusions)
	delivery.ApplyExclusionHeaders(second, append([]delivery.Exclusion(nil), exclusions...))
	if !reflect.DeepEqual(first, second) {
		t.Fatal("exclusion headers were not deterministic")
	}
	if first.Get("SubShare-Excluded-Count") != "1500" || first.Get("SubShare-Exclusion-Codes") != delivery.ReasonCompatibility+","+delivery.ReasonUnsupportedProtocol {
		t.Fatal("exclusion headers were not aggregated in stable order")
	}
	if first.Get("SubShare-Exclusion-Counts") != delivery.ReasonCompatibility+"=500,"+delivery.ReasonUnsupportedProtocol+"=1000" {
		t.Fatal("exclusion count summary mismatch")
	}
	if len(first.Get("SubShare-Exclusion-Counts")) > 512 {
		t.Fatal("aggregated exclusion header exceeded its fixed safe bound")
	}
}
