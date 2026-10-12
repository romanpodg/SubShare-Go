package main

import (
	"encoding/json"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"gopkg.in/yaml.v3"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSubscriptionCapabilityMatrixIsCompleteAndVersionPinned(t *testing.T) {
	matrix := delivery.CapabilityMatrix()
	if len(matrix) != 7 {
		t.Fatalf("capability profiles = %d, want 7", len(matrix))
	}
	want := map[string]bool{
		"vless/current": true, "vmess/current": true, "trojan/current": true,
		"shadowsocks/sip002/sip022": true, "hysteria2/2": true,
		"tuic/5": true, "tuic/4": true,
	}
	for _, capability := range matrix {
		delete(want, capability.Protocol+"/"+capability.Generation)
		for _, output := range []string{"plain", "base64", "mihomo", "sing-box", "xray-json", "structured-editing", "connectivity-probe", "compatibility-raw"} {
			claim, ok := capability.Outputs[output]
			if !ok {
				t.Errorf("%s/%s is missing %s", capability.Protocol, capability.Generation, output)
				continue
			}
			assertOfficialDeliveryCapability(t, capability, output, claim)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing capability profiles: %#v", want)
	}
	if got := findDeliveryCapability(t, matrix, "hysteria2", "2", "xray-json"); got.Status != delivery.CapabilityConditional || got.TargetVersion != delivery.TargetXrayCurrentVersion || got.MinimumVersion != delivery.TargetXrayMinimumVersion || got.SyntaxValidation != "official_binary" || got.RuntimeInteroperability != "not_tested" {
		t.Fatalf("Hysteria 2 Xray capability = %#v", got)
	}
	if got := findDeliveryCapability(t, matrix, "tuic", "5", "xray-json"); got.Status != delivery.CapabilityUnsupported || got.ReasonCode != delivery.ReasonUnsupportedProtocol {
		t.Fatalf("TUIC v5 Xray capability = %#v", got)
	}
	if got := findDeliveryCapability(t, matrix, "tuic", "4", "plain"); got.Status != delivery.CapabilityCompatibility || got.ReasonCode != "raw_delivery_only" {
		t.Fatalf("TUIC v4 plain capability = %#v", got)
	}
	encoded, err := json.Marshal(matrix)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, version := range []string{delivery.TargetMihomoVersion, delivery.TargetSingBoxVersion, delivery.TargetXrayMinimumVersion, delivery.TargetXrayCurrentVersion} {
		if !strings.Contains(text, version) {
			t.Errorf("target version %q is absent", version)
		}
	}
	for _, secret := range []string{"shadow-password", "hysteria-auth", "tuic-password"} {
		if strings.Contains(text, secret) {
			t.Fatal("capability JSON leaked synthetic credential material")
		}
	}
}

func TestCapabilityAPIUsesBackendMatrixAndRejectsClientOverride(t *testing.T) {
	app := newIntegrationApp(t)
	recorder := httptest.NewRecorder()
	app.apiV1GetSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/subscription-delivery-settings", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), delivery.TargetMihomoVersion) || !strings.Contains(recorder.Body.String(), delivery.ReasonUnsafeControl) {
		t.Fatalf("capability response status=%d or required metadata absent", recorder.Code)
	}
	var settings subscriptionDeliverySettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode capability response: %v", err)
	}
	if !reflect.DeepEqual(settings.GenerationExclusionReasonCodes, delivery.ExclusionReasonCodes()) {
		t.Fatalf("exclusion code catalog = %#v", settings.GenerationExclusionReasonCodes)
	}

	input := `{"response_headers":[],"announcement":"","remarks":{},"capabilities":[{"protocol":"fake","generation":"1","outputs":{}}],"generation_exclusion_reason_codes":["fake_reason"]}`
	recorder = httptest.NewRecorder()
	app.apiV1UpdateSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/subscription-delivery-settings", strings.NewReader(input)))
	if recorder.Code != http.StatusBadRequest || strings.Contains(recorder.Body.String(), `"protocol":"fake"`) || strings.Contains(recorder.Body.String(), "fake_reason") {
		t.Fatalf("client capability override was not rejected safely: status=%d", recorder.Code)
	}
}

func TestOpenAPIDeliveryContractsRemainParseableAndReadOnly(t *testing.T) {
	raw, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	var document any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}
	text := string(raw)
	for _, required := range []string{
		"SubscriptionDeliverySettingsUpdate:",
		"SubscriptionDeliverySettings:",
		"SubscriptionGenerationFailure:",
		"readOnly: true",
		"additionalProperties: false",
		"runtime_interoperability:",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("OpenAPI delivery contract is missing %s", required)
		}
	}
}
