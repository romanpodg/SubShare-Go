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
	requireFixturef(t, len(matrix) == 7, "capability profiles = %d, want 7", len(matrix))

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
	requireFixturef(t, len(want) == 0, "missing capability profiles: %#v", want)

	{
		got := findDeliveryCapability(t, matrix, deliveryCapabilitySelector{Protocol: "hysteria2", Generation: "2", Output: "xray-json"})
		requireFixturef(t, got.Status == delivery.CapabilityConditional, "Hysteria 2 Xray capability = %#v", got)
		requireFixturef(t, got.TargetVersion == delivery.TargetXrayCurrentVersion, "Hysteria 2 Xray capability = %#v", got)
		requireFixturef(t, got.MinimumVersion == delivery.TargetXrayMinimumVersion, "Hysteria 2 Xray capability = %#v", got)
		requireFixturef(t, got.SyntaxValidation == "official_binary", "Hysteria 2 Xray capability = %#v", got)
		requireFixturef(t, got.RuntimeInteroperability == "not_tested", "Hysteria 2 Xray capability = %#v", got)
	}

	{
		got := findDeliveryCapability(t, matrix, deliveryCapabilitySelector{Protocol: "tuic", Generation: "5", Output: "xray-json"})
		requireFixturef(t, got.Status == delivery.CapabilityUnsupported, "TUIC v5 Xray capability = %#v", got)
		requireFixturef(t, got.ReasonCode == delivery.ReasonUnsupportedProtocol, "TUIC v5 Xray capability = %#v", got)
	}

	{
		got := findDeliveryCapability(t, matrix, deliveryCapabilitySelector{Protocol: "tuic", Generation: "4", Output: "plain"})
		requireFixturef(t, got.Status == delivery.CapabilityCompatibility, "TUIC v4 plain capability = %#v", got)
		requireFixturef(t, got.ReasonCode == "raw_delivery_only", "TUIC v4 plain capability = %#v", got)
	}

	encoded, err := json.Marshal(matrix)
	requireFixture(t, err == nil, err)

	text := string(encoded)
	for _, version := range []string{delivery.TargetMihomoVersion, delivery.TargetSingBoxVersion, delivery.TargetXrayMinimumVersion, delivery.TargetXrayCurrentVersion} {
		checkFixturef(t, strings.Contains(text, version), "target version %q is absent", version)

	}
	for _, secret := range []string{"shadow-password", "hysteria-auth", "tuic-password"} {
		requireFixture(t, !(strings.Contains(text, secret)), "capability JSON leaked synthetic credential material")

	}
}

func TestCapabilityAPIUsesBackendMatrixAndRejectsClientOverride(t *testing.T) {
	app := newIntegrationApp(t)
	recorder := httptest.NewRecorder()
	app.apiV1GetSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/subscription-delivery-settings", nil))
	requireFixturef(t, recorder.Code == http.StatusOK, "capability response status=%d or required metadata absent", recorder.Code)
	requireFixturef(t, strings.Contains(recorder.Body.String(), delivery.TargetMihomoVersion), "capability response status=%d or required metadata absent", recorder.Code)
	requireFixturef(t, strings.Contains(recorder.Body.String(), delivery.ReasonUnsafeControl), "capability response status=%d or required metadata absent", recorder.Code)

	var settings subscriptionDeliverySettingsResponse
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &settings)
		requireFixturef(t, err == nil, "decode capability response: %v", err)
	}

	requireFixturef(t, reflect.DeepEqual(settings.GenerationExclusionReasonCodes, delivery.ExclusionReasonCodes()), "exclusion code catalog = %#v", settings.GenerationExclusionReasonCodes)

	input := `{"response_headers":[],"announcement":"","remarks":{},"capabilities":[{"protocol":"fake","generation":"1","outputs":{}}],"generation_exclusion_reason_codes":["fake_reason"]}`
	recorder = httptest.NewRecorder()
	app.apiV1UpdateSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/subscription-delivery-settings", strings.NewReader(input)))
	requireFixturef(t, recorder.Code == http.StatusBadRequest, "client capability override was not rejected safely: status=%d", recorder.Code)
	requireFixturef(t, !(strings.Contains(recorder.Body.String(), `"protocol":"fake"`)), "client capability override was not rejected safely: status=%d", recorder.Code)
	requireFixturef(t, !(strings.Contains(recorder.Body.String(), "fake_reason")), "client capability override was not rejected safely: status=%d", recorder.Code)

}

func TestOpenAPIDeliveryContractsRemainParseableAndReadOnly(t *testing.T) {
	raw, err := os.ReadFile("openapi.yaml")
	requireFixturef(t, err == nil, "read OpenAPI contract: %v", err)

	var document any
	{
		err := yaml.Unmarshal(raw, &document)
		requireFixturef(t, err == nil, "parse OpenAPI contract: %v", err)
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
		requireFixturef(t, strings.Contains(text, required), "OpenAPI delivery contract is missing %s", required)

	}
}
