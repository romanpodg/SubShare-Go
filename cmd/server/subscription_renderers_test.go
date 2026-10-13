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
	requireFixture(t, err == nil, err)

	second, err := delivery.RenderMihomo(entries)
	requireFixturef(t, err == nil, "Mihomo output is not deterministic: %v", err)
	requireFixturef(t, first.Body == second.Body, "Mihomo output is not deterministic: %v", err)

	var yamlValue any
	{
		err := yaml.Unmarshal([]byte(first.Body), &yamlValue)
		requireFixturef(t, err == nil, "Mihomo YAML invalid: %v", err)
	}

	root := decodeObject(t, first.Body)
	proxies := root["proxies"].([]any)
	requireFixturef(t, len(proxies) == 5, "Mihomo proxies=%d exclusions=%#v", len(proxies), first.Exclusions)

	ss := proxies[0].(map[string]any)
	requireFixture(t, ss["type"] == "ss", "Mihomo Shadowsocks mapping mismatch")
	requireFixture(t, ss["plugin"] == "obfs", "Mihomo Shadowsocks mapping mismatch")
	requireFixture(t, ss["password"] == "shadow-password", "Mihomo Shadowsocks mapping mismatch")

	hy := proxies[1].(map[string]any)
	requireFixture(t, hy["ports"] == "443,5000-5010", "Mihomo Hysteria mapping mismatch")
	requireFixture(t, hy["obfs"] == "salamander", "Mihomo Hysteria mapping mismatch")
	requireFixture(t, hy["password"] == "hysteria-auth", "Mihomo Hysteria mapping mismatch")
	requireFixture(t, hy["fingerprint"] == "abababababababababababababababababababababababababababababababab", "Mihomo Hysteria mapping mismatch")

	tuic := proxies[2].(map[string]any)
	requireFixture(t, tuic["uuid"] == "33333333-3333-4333-8333-333333333333", "Mihomo TUIC mapping mismatch")
	requireFixture(t, tuic["request-timeout"] == float64(8000), "Mihomo TUIC mapping mismatch")
	requireFixture(t, tuic["reduce-rtt"] == true, "Mihomo TUIC mapping mismatch")

	gecko := proxies[3].(map[string]any)
	requireFixture(t, gecko["server"] == "2001:db8::1", "Mihomo Gecko/IPv6 mapping mismatch")
	requireFixture(t, gecko["obfs"] == "gecko", "Mihomo Gecko/IPv6 mapping mismatch")

	aliasTUIC := proxies[4].(map[string]any)
	requireFixture(t, aliasTUIC["sni"] == "tls.example", "Mihomo TUIC aliases were not normalized")
	requireFixture(t, aliasTUIC["congestion-controller"] == "bbr", "Mihomo TUIC aliases were not normalized")
	requireFixture(t, aliasTUIC["reduce-rtt"] == true, "Mihomo TUIC aliases were not normalized")

	codes := exclusionCodes(first.Exclusions)
	requireFixturef(t, codes[delivery.ReasonClientVersion], "Mihomo exclusions: %#v", first.Exclusions)
	requireFixturef(t, codes[delivery.ReasonCompatibility], "Mihomo exclusions: %#v", first.Exclusions)
	requireFixturef(t, codes[delivery.ReasonUnrepresentable], "Mihomo exclusions: %#v", first.Exclusions)
	requireFixturef(t, codes[delivery.ReasonAmbiguous], "Mihomo exclusions: %#v", first.Exclusions)

	for _, exclusion := range first.Exclusions {
		requireFixturef(t, !(strings.Contains(exclusion.Error(), "gecko-secret")), "secret-bearing exclusion: %v", exclusion)
		requireFixturef(t, !(strings.Contains(exclusion.Error(), "legacy-token-value")), "secret-bearing exclusion: %v", exclusion)

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
	requireFixture(t, err == nil, err)

	root := decodeObject(t, generated.Body)
	outbounds := root["outbounds"].([]any)
	requireFixturef(t, len(outbounds) == 3, "sing-box outbounds=%d exclusions=%#v", len(outbounds), generated.Exclusions)

	ss := outbounds[0].(map[string]any)
	requireFixture(t, ss["type"] == "shadowsocks", "sing-box Shadowsocks mapping mismatch")
	requireFixture(t, ss["plugin"] == "obfs-local", "sing-box Shadowsocks mapping mismatch")

	hy := outbounds[1].(map[string]any)
	requireFixture(t, hy["server"] == "2001:db8::2", "sing-box Hysteria mapping mismatch")
	requireFixture(t, reflect.DeepEqual(hy["server_ports"], []any{"443:443", "5000:5002"}), "sing-box Hysteria mapping mismatch")

	tuic := outbounds[2].(map[string]any)
	requireFixture(t, tuic["udp_over_stream"] == true, "sing-box TUIC mapping mismatch")
	requireFixture(t, tuic["zero_rtt_handshake"] == true, "sing-box TUIC mapping mismatch")

	{
		_, exists := tuic["request-timeout"]
		requireFixture(t, !(exists), "Mihomo-only field leaked into sing-box")
	}

	codes := exclusionCodes(generated.Exclusions)
	requireFixturef(t, codes[delivery.ReasonUnrepresentable], "sing-box exclusions: %#v", generated.Exclusions)
	requireFixturef(t, codes[delivery.ReasonCompatibility], "sing-box exclusions: %#v", generated.Exclusions)
	requireFixturef(t, codes[delivery.ReasonClientVersion], "sing-box exclusions: %#v", generated.Exclusions)

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
	requireFixturef(t, err == nil, "Xray output invalid: err=%v", err)
	requireFixturef(t, json.Valid([]byte(generated.Body)), "Xray output invalid: err=%v", err)

	var values []map[string]any
	{
		err := json.Unmarshal([]byte(generated.Body), &values)
		requireFixture(t, err == nil, err)
	}

	requireFixturef(t, len(values) == 6, "Xray supported entries=%d exclusions=%#v", len(values), generated.Exclusions)

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
	requireFixture(t, foundSS, "plugin-free Shadowsocks mapping absent")

	requireFixture(t, foundHysteria, "representable Hysteria 2 mapping absent")

	requireFixture(t, !(inventedHopInterval), "Xray mapping invented a URI-absent UDP hop interval")

	codes := exclusionCodes(generated.Exclusions)
	requireFixturef(t, codes[delivery.ReasonPlugin], "Xray exclusions: %#v", generated.Exclusions)
	requireFixturef(t, codes[delivery.ReasonUnsupportedProtocol], "Xray exclusions: %#v", generated.Exclusions)
	requireFixturef(t, codes[delivery.ReasonCompatibility], "Xray exclusions: %#v", generated.Exclusions)
	requireFixturef(t, codes[delivery.ReasonUnrepresentable], "Xray exclusions: %#v", generated.Exclusions)

}

func TestXrayRejectsRemovedInsecureTLSField(t *testing.T) {
	generated, err := delivery.RenderXray([]delivery.Entry{{
		ID: 1, Raw: "hy2://synthetic-auth@xray-insecure.example:443?insecure=1", Kind: model.KeyKindReal,
	}})
	requireFixturef(t, err == nil, "Xray insecure exclusion failed: %v", err)

	requireFixture(t, generated.GeneratedCount == 0, "removed Xray allowInsecure field was not safely excluded")
	requireFixture(t, generated.Body == "[]", "removed Xray allowInsecure field was not safely excluded")
	requireFixture(t, len(generated.Exclusions) == 1, "removed Xray allowInsecure field was not safely excluded")
	requireFixture(t, generated.Exclusions[0].Reason == delivery.ReasonUnrepresentable, "removed Xray allowInsecure field was not safely excluded")

	requireFixture(t, !(strings.Contains(generated.Exclusions[0].Error(), "synthetic-auth")), "Xray insecure exclusion leaked authentication material")

}

func TestStructuredNamesAreSafeUniqueAndSerializationValidationRejectsInjection(t *testing.T) {
	names := &delivery.UniqueNames{}
	{
		first, second := names.Next("node\nsecret", ""), names.Next("node\nsecret", "")
		requireFixturef(t, first == "nodesecret", "safe names = %q, %q", first, second)
		requireFixturef(t, second == "nodesecret (2)", "safe names = %q, %q", first, second)
	}

	{
		err := delivery.ValidateStructuredBody("mihomo", "proxies:\n  - name: [")
		requireFixture(t, err != nil, "invalid Mihomo YAML was accepted")
	}

	{
		err := delivery.ValidateStructuredBody("sing-box", `{"outbounds":[`)
		requireFixture(t, err != nil, "invalid sing-box JSON was accepted")
	}

}
