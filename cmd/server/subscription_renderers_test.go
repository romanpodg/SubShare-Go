package main

import (
	"encoding/json"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"gopkg.in/yaml.v3"
	"reflect"
	"strings"
	"testing"
)

func TestMihomoMappingsAndSafeExclusions(t *testing.T) {
	entries := []delivery.Entry{
		{ID: 1, Raw: deliverySSPlugin, Kind: model.KeyKindReal},
		{ID: 2, Raw: deliveryHY2, Kind: model.KeyKindReal},
		{ID: 3, Raw: deliveryTUICMihomo, Kind: model.KeyKindReal},
		{ID: 4, Raw: "hysteria2://auth@[2001:db8::1]:443?obfs=gecko&obfs-password=gecko-secret#Gecko", Kind: model.KeyKindReal},
		{ID: 5, Raw: externalTestTUICV4, Kind: model.KeyKindReal},
		{ID: 6, Raw: "hy2://auth@hy.example:443?ech=client-version-sensitive", Kind: model.KeyKindReal},
		{ID: 7, Raw: "hy2://auth@hy.example:443?sni=first.example&sni=second.example", Kind: model.KeyKindReal},
		{ID: 8, Raw: "tuic://33333333-3333-4333-8333-333333333333:tuic-password@tuic.example:443?server_name=tls.example&congestion_control=bbr&udp_relay_mode=quic&zero_rtt_handshake=true&heartbeat=10s#Alias", Kind: model.KeyKindReal},
		{ID: 9, Raw: deliverySSXrayAlias, Kind: model.KeyKindReal},
	}
	first, err := delivery.RenderMihomo(entries)
	if err != nil {
		t.Fatal(err)
	}
	second, err := delivery.RenderMihomo(entries)
	if err != nil || first.Body != second.Body {
		t.Fatalf("Mihomo output is not deterministic: %v", err)
	}
	var yamlValue any
	if err := yaml.Unmarshal([]byte(first.Body), &yamlValue); err != nil {
		t.Fatalf("Mihomo YAML invalid: %v", err)
	}
	root := decodeObject(t, first.Body)
	proxies := root["proxies"].([]any)
	if len(proxies) != 5 {
		t.Fatalf("Mihomo proxies=%d exclusions=%#v", len(proxies), first.Exclusions)
	}
	ss := proxies[0].(map[string]any)
	if ss["type"] != "ss" || ss["plugin"] != "obfs" || ss["password"] != "shadow-password" {
		t.Fatal("Mihomo Shadowsocks mapping mismatch")
	}
	hy := proxies[1].(map[string]any)
	if hy["ports"] != "443,5000-5010" || hy["obfs"] != "salamander" || hy["password"] != "hysteria-auth" || hy["fingerprint"] != "abababababababababababababababababababababababababababababababab" {
		t.Fatal("Mihomo Hysteria mapping mismatch")
	}
	tuic := proxies[2].(map[string]any)
	if tuic["uuid"] != "33333333-3333-4333-8333-333333333333" || tuic["request-timeout"] != float64(8000) || tuic["reduce-rtt"] != true {
		t.Fatal("Mihomo TUIC mapping mismatch")
	}
	gecko := proxies[3].(map[string]any)
	if gecko["server"] != "2001:db8::1" || gecko["obfs"] != "gecko" {
		t.Fatal("Mihomo Gecko/IPv6 mapping mismatch")
	}
	aliasTUIC := proxies[4].(map[string]any)
	if aliasTUIC["sni"] != "tls.example" || aliasTUIC["congestion-controller"] != "bbr" || aliasTUIC["reduce-rtt"] != true {
		t.Fatal("Mihomo TUIC aliases were not normalized")
	}
	codes := exclusionCodes(first.Exclusions)
	if !codes[delivery.ReasonClientVersion] || !codes[delivery.ReasonCompatibility] || !codes[delivery.ReasonUnrepresentable] || !codes[delivery.ReasonAmbiguous] {
		t.Fatalf("Mihomo exclusions: %#v", first.Exclusions)
	}
	for _, exclusion := range first.Exclusions {
		if strings.Contains(exclusion.Error(), "gecko-secret") || strings.Contains(exclusion.Error(), "legacy-token-value") {
			t.Fatalf("secret-bearing exclusion: %v", exclusion)
		}
	}
}

func TestSingBoxMappingsProvenanceAndExclusions(t *testing.T) {
	entries := []delivery.Entry{
		{ID: 1, Raw: deliverySSPlugin, Kind: model.KeyKindReal},
		{ID: 2, Raw: "hysteria2://auth@[2001:db8::2]:443,5000-5002?sni=hy.example&insecure=1&obfs=salamander&obfs-password=secret#HY", Kind: model.KeyKindReal},
		{ID: 3, Raw: deliveryTUICSingBox, Kind: model.KeyKindReal},
		{ID: 4, Raw: deliveryTUICMihomo, Kind: model.KeyKindReal},
		{ID: 5, Raw: "hy2://auth@hy.example:443?pinSHA256=abababababababababababababababababababababababababababababababab", Kind: model.KeyKindReal},
		{ID: 6, Raw: externalTestTUICV4, Kind: model.KeyKindReal},
		{ID: 7, Raw: "hysteria2://auth@hy.example:443?obfs=gecko&obfs-password=gecko-secret", Kind: model.KeyKindReal},
	}
	generated, err := delivery.RenderSingBox(entries)
	if err != nil {
		t.Fatal(err)
	}
	root := decodeObject(t, generated.Body)
	outbounds := root["outbounds"].([]any)
	if len(outbounds) != 3 {
		t.Fatalf("sing-box outbounds=%d exclusions=%#v", len(outbounds), generated.Exclusions)
	}
	ss := outbounds[0].(map[string]any)
	if ss["type"] != "shadowsocks" || ss["plugin"] != "obfs-local" {
		t.Fatal("sing-box Shadowsocks mapping mismatch")
	}
	hy := outbounds[1].(map[string]any)
	if hy["server"] != "2001:db8::2" || !reflect.DeepEqual(hy["server_ports"], []any{"443:443", "5000:5002"}) {
		t.Fatal("sing-box Hysteria mapping mismatch")
	}
	tuic := outbounds[2].(map[string]any)
	if tuic["udp_over_stream"] != true || tuic["zero_rtt_handshake"] != true {
		t.Fatal("sing-box TUIC mapping mismatch")
	}
	if _, exists := tuic["request-timeout"]; exists {
		t.Fatal("Mihomo-only field leaked into sing-box")
	}
	codes := exclusionCodes(generated.Exclusions)
	if !codes[delivery.ReasonUnrepresentable] || !codes[delivery.ReasonCompatibility] || !codes[delivery.ReasonClientVersion] {
		t.Fatalf("sing-box exclusions: %#v", generated.Exclusions)
	}
}

func TestXrayPartialOutputPreservesLegacyAndSupportsPluginFreeShadowsocks(t *testing.T) {
	entries := []delivery.Entry{
		{ID: 1, Raw: externalTestVLESS, Kind: model.KeyKindReal},
		{ID: 2, Raw: externalTestVMess, Kind: model.KeyKindReal},
		{ID: 3, Raw: externalTestTrojan, Kind: model.KeyKindReal},
		{ID: 4, Raw: deliverySSClean, Kind: model.KeyKindReal},
		{ID: 5, Raw: deliverySSPlugin, Kind: model.KeyKindReal},
		{ID: 6, Raw: deliveryHY2, Kind: model.KeyKindReal},
		{ID: 7, Raw: deliveryTUICMihomo, Kind: model.KeyKindReal},
		{ID: 8, Raw: externalTestTUICV4, Kind: model.KeyKindReal},
		{ID: 9, Raw: deliverySSXrayAlias, Kind: model.KeyKindReal},
		{ID: 10, Raw: "hysteria2://auth@hy.example:443?obfs=gecko&obfs-password=gecko-secret", Kind: model.KeyKindReal},
	}
	generated, err := delivery.RenderXray(entries)
	if err != nil || !json.Valid([]byte(generated.Body)) {
		t.Fatalf("Xray output invalid: err=%v", err)
	}
	var values []map[string]any
	if err := json.Unmarshal([]byte(generated.Body), &values); err != nil {
		t.Fatal(err)
	}
	if len(values) != 6 {
		t.Fatalf("Xray supported entries=%d exclusions=%#v", len(values), generated.Exclusions)
	}
	foundSS := false
	foundHysteria := false
	inventedHopInterval := false
	for _, value := range values {
		encoded, _ := json.Marshal(value)
		if strings.Contains(string(encoded), `"protocol":"shadowsocks"`) {
			foundSS = foundSS || strings.Contains(string(encoded), `"method":"aes-256-gcm"`) && strings.Contains(string(encoded), `"address":"ss.example"`)
		}
		if strings.Contains(string(encoded), `"protocol":"hysteria"`) {
			foundHysteria = strings.Contains(string(encoded), `"ports":"443,5000-5010"`) &&
				strings.Contains(string(encoded), `"pinnedPeerCertSha256":"abababab`) &&
				strings.Contains(string(encoded), `"type":"salamander"`)
			inventedHopInterval = strings.Contains(string(encoded), `"interval"`)
		}
	}
	if !foundSS {
		t.Fatal("plugin-free Shadowsocks mapping absent")
	}
	if !foundHysteria {
		t.Fatal("representable Hysteria 2 mapping absent")
	}
	if inventedHopInterval {
		t.Fatal("Xray mapping invented a URI-absent UDP hop interval")
	}
	codes := exclusionCodes(generated.Exclusions)
	if !codes[delivery.ReasonPlugin] || !codes[delivery.ReasonUnsupportedProtocol] || !codes[delivery.ReasonCompatibility] || !codes[delivery.ReasonUnrepresentable] {
		t.Fatalf("Xray exclusions: %#v", generated.Exclusions)
	}
}

func TestXrayRejectsRemovedInsecureTLSField(t *testing.T) {
	generated, err := delivery.RenderXray([]delivery.Entry{{
		ID: 1, Raw: "hy2://synthetic-auth@xray-insecure.example:443?insecure=1", Kind: model.KeyKindReal,
	}})
	if err != nil {
		t.Fatalf("Xray insecure exclusion failed: %v", err)
	}
	if generated.GeneratedCount != 0 || generated.Body != "[]" || len(generated.Exclusions) != 1 || generated.Exclusions[0].Reason != delivery.ReasonUnrepresentable {
		t.Fatal("removed Xray allowInsecure field was not safely excluded")
	}
	if strings.Contains(generated.Exclusions[0].Error(), "synthetic-auth") {
		t.Fatal("Xray insecure exclusion leaked authentication material")
	}
}

func TestStructuredNamesAreSafeUniqueAndSerializationValidationRejectsInjection(t *testing.T) {
	names := &delivery.UniqueNames{}
	if first, second := names.Next("node\nsecret", ""), names.Next("node\nsecret", ""); first != "nodesecret" || second != "nodesecret (2)" {
		t.Fatalf("safe names = %q, %q", first, second)
	}
	if err := delivery.ValidateStructuredBody("mihomo", "proxies:\n  - name: ["); err == nil {
		t.Fatal("invalid Mihomo YAML was accepted")
	}
	if err := delivery.ValidateStructuredBody("sing-box", `{"outbounds":[`); err == nil {
		t.Fatal("invalid sing-box JSON was accepted")
	}
}
