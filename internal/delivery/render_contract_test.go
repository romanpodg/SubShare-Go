package delivery_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

const contractTrojan = "trojan://fixture-password@edge.example:443#Embedded"

func deliveryEqual(t *testing.T, label string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s differs from the frozen contract", label)
	}
}

func deliverySuccess(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func deliveryFailure(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected delivery rejection")
	}
	return err.Error()
}

func TestLocalDeliveryFormatSelection(t *testing.T) {
	cases := []struct{ requested, configured, encoding, want string }{
		{"mihomo", "xray-json", "base64", "mihomo"},
		{"sing-box", "links", "", "sing-box"},
		{"plain", "xray-json", "base64", "plain"},
		{"", "xray-json", "base64", "xray-json"},
		{"", "links", "base64", "base64"},
		{"", "links", "", "plain"},
	}
	for _, c := range cases {
		t.Run(c.want+"/"+c.requested, func(t *testing.T) {
			settings := model.SubscriptionSettings{SubscriptionFormat: c.configured}
			deliveryEqual(t, "format precedence", delivery.EffectiveFormat(c.requested, settings, c.encoding), c.want)
		})
	}
}

func TestLocalDeliveryStructuredCredentialsAndNames(t *testing.T) {
	wantedNames := map[string][]string{
		"mihomo":   {"Subscriber", "Subscriber (2)"},
		"sing-box": {"Subscriber", "Subscriber (2)"},
		// Legacy Xray conversion retains embedded remarks independently per document.
		"xray-json": {"Embedded", "Embedded"},
	}
	for _, format := range []string{"mihomo", "sing-box", "xray-json"} {
		t.Run(format, func(t *testing.T) {
			entries := []delivery.Entry{
				{ID: 7, Kind: model.KeyKindReal, Raw: contractTrojan, ClientDisplayName: "Subscriber"},
				{ID: 8, Kind: model.KeyKindReal, Raw: contractTrojan, ClientDisplayName: "Subscriber"},
			}
			generated, err := delivery.Render(format, entries, "links", delivery.TemplateData{})
			deliverySuccess(t, err)
			deliveryEqual(t, "generated entries", generated.GeneratedCount, 2)
			deliveryEqual(t, "exclusions", len(generated.Exclusions), 0)
			deliverySuccess(t, delivery.ValidateStructuredBody(format, generated.Body))
			items := structuredContractItems(t, format, generated.Body)
			assertStructuredContract(t, format, items[0], wantedNames[format][0])
			assertStructuredContract(t, format, items[1], wantedNames[format][1])
		})
	}
}

func structuredContractItems(t *testing.T, format, body string) []map[string]any {
	t.Helper()
	if format == "xray-json" {
		var documents []struct {
			Outbounds []map[string]any `json:"outbounds"`
		}
		deliverySuccess(t, json.Unmarshal([]byte(body), &documents))
		deliveryEqual(t, "document count", len(documents), 2)
		return []map[string]any{documents[0].Outbounds[0], documents[1].Outbounds[0]}
	}
	var document map[string][]map[string]any
	deliverySuccess(t, json.Unmarshal([]byte(body), &document))
	if format == "mihomo" {
		return document["proxies"]
	}
	return document["outbounds"]
}

func assertStructuredContract(t *testing.T, format string, item map[string]any, name string) {
	t.Helper()
	if format == "xray-json" {
		deliveryEqual(t, "tag", item["tag"], name)
		deliveryEqual(t, "protocol", item["protocol"], "trojan")
		settings := item["settings"].(map[string]any)
		server := settings["servers"].([]any)[0].(map[string]any)
		deliveryEqual(t, "credentials", server, map[string]any{"address": "edge.example", "port": float64(443), "password": "fixture-password"})
		return
	}
	nameKey, portKey := "tag", "server_port"
	if format == "mihomo" {
		nameKey, portKey = "name", "port"
	}
	deliveryEqual(t, "name", item[nameKey], name)
	deliveryEqual(t, "protocol", item["type"], "trojan")
	deliveryEqual(t, "server", item["server"], "edge.example")
	deliveryEqual(t, "port", item[portKey], float64(443))
	deliveryEqual(t, "password", item["password"], "fixture-password")
}

func TestLocalDeliveryMixedExclusionsKeepGoodProfiles(t *testing.T) {
	for _, format := range []string{"mihomo", "sing-box", "xray-json"} {
		t.Run(format, func(t *testing.T) {
			generated, err := delivery.Render(format, []delivery.Entry{
				{ID: 1, Kind: model.KeyKindReal, Raw: contractTrojan},
				{ID: 2, Kind: model.KeyKindReal, Raw: "invalid-fixture", StoredProtocol: "trojan"},
			}, "links", delivery.TemplateData{})
			deliverySuccess(t, err)
			deliveryEqual(t, "generated count", generated.GeneratedCount, 1)
			deliveryEqual(t, "safe rejection", generated.Exclusions, []delivery.Exclusion{{RecordRef: "key:2", Protocol: "trojan", Format: format, Reason: delivery.ReasonInvalidStored}})
			deliverySuccess(t, delivery.ValidateStructuredBody(format, generated.Body))
			deliveryEqual(t, "invalid input absent", strings.Contains(generated.Body, "invalid-fixture"), false)
		})
	}
}

func TestLocalDeliveryPlainAndInformationalSemantics(t *testing.T) {
	entries := []delivery.Entry{
		{ID: 1, Kind: model.KeyKindReal, Raw: contractTrojan, ClientDisplayName: "Subscriber"},
		{ID: 2, Kind: model.KeyKindInformational, TemplateText: "Hello {user_name}: {real_keys_count}"},
		{ID: 3, Kind: model.KeyKindInformational},
	}
	generated, err := delivery.Render("plain", entries, "links", delivery.TemplateData{UserName: " Alice ", RealKeysCount: 1})
	deliverySuccess(t, err)
	deliveryEqual(t, "plain body", generated.Body, "trojan://fixture-password@edge.example:443#Subscriber\n"+delivery.InformationalVLESSURL("Hello Alice: 1"))
	deliveryEqual(t, "blank info omitted", generated.GeneratedCount, 2)
	deliveryEqual(t, "base64 transport", delivery.EncodeBase64("hello\n"), "aGVsbG8K")
}

func TestLocalDeliveryExclusionHeadersAndFailureAreSafe(t *testing.T) {
	exclusions := []delivery.Exclusion{
		{RecordRef: "key:1", Reason: delivery.ReasonPlugin},
		{RecordRef: "key:2", Reason: delivery.ReasonInvalidStored},
		{RecordRef: "key:3", Reason: delivery.ReasonPlugin},
		{RecordRef: "key:4", Reason: "unregistered-secret-like-value"},
	}
	headers := http.Header{}
	delivery.ApplyExclusionHeaders(headers, exclusions)
	deliveryEqual(t, "excluded count", headers.Get("SubShare-Excluded-Count"), "4")
	deliveryEqual(t, "ordered safe codes", headers.Get("SubShare-Exclusion-Codes"), "invalid_stored_raw_uri,required_plugin_unsupported")
	deliveryEqual(t, "counts", headers.Get("SubShare-Exclusion-Counts"), "invalid_stored_raw_uri=1,required_plugin_unsupported=2")
	failure := delivery.FailurePayload(delivery.Generated{OutputFormat: "mihomo", EligibleCount: 4, Exclusions: exclusions})
	deliveryEqual(t, "failure", failure, delivery.Failure{ErrorCode: delivery.ReasonAllExcluded, OutputFormat: "mihomo", EligibleCount: 4, ExcludedCount: 4, ExclusionCounts: map[string]int{delivery.ReasonInvalidStored: 1, delivery.ReasonPlugin: 2}})
}

func TestLocalDeliveryStructuredValidationRejectsBrokenOutput(t *testing.T) {
	cases := []struct{ format, body string }{
		{"mihomo", `{"proxies":[]}`},
		{"sing-box", `{"outbounds":[{}]}`},
		{"xray-json", `[{"outbounds":[]}]`},
		{"sing-box", "{\"outbounds\":[]}\x00"},
	}
	for _, c := range cases {
		t.Run(c.format+"/"+c.body, func(t *testing.T) {
			err := delivery.ValidateStructuredBody(c.format, c.body)
			deliveryEqual(t, "invalid generated output", deliveryFailure(t, err), delivery.ReasonSerialization)
		})
	}
}
