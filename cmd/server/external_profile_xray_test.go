package main

import (
	"encoding/json"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"strings"
	"testing"
)

func TestExternalImporterExistingXrayJSONCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantHost string
	}{
		{
			name: "VLESS with unrelated top-level fields and UTF-8",
			raw: ` {
				"log":{"loglevel":"warning"},
				"remarks":"Тестовый профиль",
				"routing":{"domainStrategy":"AsIs"},
				"outbounds":[{"protocol":"vless","tag":"Выход","settings":{"vnext":[{"address":"vless-json.example","port":443,"users":[{"id":"44444444-4444-4444-8444-444444444444"}]}]}}]
			} `,
			wantHost: "vless-json.example",
		},
		{
			name:     "VMess",
			raw:      `{"outbounds":[{"protocol":"vmess","tag":"VMess JSON","settings":{"vnext":[{"address":"vmess-json.example","port":8443,"users":[{"id":"55555555-5555-4555-8555-555555555555","security":"auto"}]}]}}]}`,
			wantHost: "vmess-json.example",
		},
		{
			name:     "Trojan",
			raw:      `{"outbounds":[{"protocol":"trojan","tag":"Trojan JSON","settings":{"servers":[{"address":"trojan-json.example","port":443,"password":"trojan-json-secret"}]}}]}`,
			wantHost: "trojan-json.example",
		},
		{
			name:     "multiple supported and unrelated outbounds",
			raw:      `{"dns":{"servers":["1.1.1.1"]},"outbounds":[{"protocol":"freedom","tag":"direct","settings":{}},{"protocol":"vless","tag":"first","settings":{"vnext":[{"address":"multi-vless.example","port":443,"users":[{"id":"66666666-6666-4666-8666-666666666666"}]}]}},{"protocol":"trojan","tag":"second","settings":{"servers":[{"address":"multi-trojan.example","port":8443,"password":"multi-secret"}]}}]}`,
			wantHost: "multi-vless.example",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := parseExternalTestBody(t, test.raw)
			requireFixturef(t, parsed.DetectedFormat == "xray-json", "Xray compatibility result=%#v", parsed)
			requireFixturef(t, len(parsed.Keys) == 1, "Xray compatibility result=%#v", parsed)
			requireFixturef(t, parsed.Keys[0].Protocol == "xray-json", "Xray compatibility result=%#v", parsed)
			requireFixturef(t, parsed.Keys[0].Host == test.wantHost, "Xray compatibility result=%#v", parsed)

		})
	}
	arrayBody := `[
		{"remarks":"Array VLESS","outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"array-vless.example","port":443,"users":[{"id":"77777777-7777-4777-8777-777777777777"}]}]}}]},
		{"remarks":"Array Trojan","outbounds":[{"protocol":"trojan","settings":{"servers":[{"address":"array-trojan.example","port":443,"password":"array-secret"}]}}]}
	]`
	arrayParsed := parseExternalTestBody(t, arrayBody)
	requireFixturef(t, arrayParsed.DetectedFormat == "xray-json", "Xray JSON array compatibility result=%#v", arrayParsed)
	requireFixturef(t, len(arrayParsed.Keys) == 2, "Xray JSON array compatibility result=%#v", arrayParsed)
	requireFixturef(t, arrayParsed.Keys[0].Host == "array-vless.example", "Xray JSON array compatibility result=%#v", arrayParsed)
	requireFixturef(t, arrayParsed.Keys[1].Host == "array-trojan.example", "Xray JSON array compatibility result=%#v", arrayParsed)

	for _, raw := range []string{
		`{"outbounds":[]}`,
		`{"valid":"json but not xray"}`,
	} {
		parsed := parseExternalTestBody(t, raw)
		requireFixturef(t, len(parsed.Keys) == 0, "unsupported Xray-shaped JSON result=%#v", parsed)
		requireFixturef(t, parsed.Counts.Rejected == 1, "unsupported Xray-shaped JSON result=%#v", parsed)
		requireFixturef(t, parsed.Items[0].ErrorCode == "invalid_xray_json", "unsupported Xray-shaped JSON result=%#v", parsed)

	}
}

func TestExternalImporterRecognizesXrayHysteria2AndPreservesSupportedFields(t *testing.T) {
	raw := `{
		"remarks":"HY2 JSON",
		"outbounds":[{
			"tag":"HY2 outbound",
			"protocol":"hysteria",
			"settings":{"version":2,"address":"hy-json.example","port":443},
			"streamSettings":{
				"method":"hysteria",
				"security":"tls",
				"hysteriaSettings":{"version":2,"auth":"json-hysteria-auth"},
				"tlsSettings":{"serverName":"tls-json.example","pinnedPeerCertSha256":"abababababababababababababababababababababababababababababababab"},
				"finalmask":{
					"quicParams":{"udpHop":{"ports":"443,5000-5010"}},
					"udp":[{"type":"salamander","settings":{"password":"json-obfs-secret"}}]
				}
			}
		}]
	}`
	parsed := parseExternalTestBody(t, raw)
	requireFixturef(t, parsed.Counts.Accepted == 1, "Hysteria2 JSON parse=%#v", parsed)
	requireFixturef(t, len(parsed.Keys) == 1, "Hysteria2 JSON parse=%#v", parsed)
	requireFixturef(t, parsed.Keys[0].Protocol == "hysteria2", "Hysteria2 JSON parse=%#v", parsed)

	profile, err := profiles.Parse(parsed.Keys[0].URL)
	requireFixturef(t, err == nil, "parse imported Hysteria2 URI: %v", err)

	data, ok := profile.Data.(profiles.Hysteria2Data)
	requireFixturef(t, ok, "imported Hysteria2 profile=%#v data=%#v", profile, data)
	requireFixturef(t, profile.Server == "hy-json.example", "imported Hysteria2 profile=%#v data=%#v", profile, data)
	requireFixturef(t, profile.Port.Expression == "443,5000-5010", "imported Hysteria2 profile=%#v data=%#v", profile, data)
	requireFixturef(t, profile.DisplayName == "HY2 JSON", "imported Hysteria2 profile=%#v data=%#v", profile, data)
	requireFixturef(t, data.Authentication.Reveal() == "json-hysteria-auth", "imported Hysteria2 profile=%#v data=%#v", profile, data)
	requireFixturef(t, data.SNI == "tls-json.example", "imported Hysteria2 profile=%#v data=%#v", profile, data)
	requireFixturef(t, data.CertificateSHA256 == "abababababababababababababababababababababababababababababababab", "imported Hysteria2 profile=%#v data=%#v", profile, data)
	requireFixturef(t, data.ObfuscationType == "salamander", "imported Hysteria2 profile=%#v data=%#v", profile, data)
	requireFixturef(t, data.ObfuscationPassword.Reveal() == "json-obfs-secret", "imported Hysteria2 profile=%#v data=%#v", profile, data)

	requireFixture(t, parsed.Keys[0].Fingerprint != "", "semantic fingerprint missing")

	encoded, err := json.Marshal(parsed.Items)
	requireFixturef(t, err == nil, "marshal preview: %v", err)

	for _, secret := range []string{"json-hysteria-auth", "json-obfs-secret", "hysteria2://", `"outbounds"`} {
		requireFixturef(t, !(strings.Contains(string(encoded), secret)), "preview leaked %q: %s", secret, encoded)

	}
}

func TestExternalImporterClassifiesHysteriaV1AndIdentifiableXrayFailures(t *testing.T) {
	v1 := `{"remarks":"Legacy Hysteria","outbounds":[{"protocol":"hysteria","settings":{"version":1,"address":"legacy-hy.example","port":8443},"streamSettings":{"hysteriaSettings":{"version":1,"auth":"never-preview-this"}}}]}`
	parsed := parseExternalTestBody(t, v1)
	requireFixturef(t, len(parsed.Keys) == 0, "Hysteria v1 parse=%#v", parsed)
	requireFixturef(t, parsed.Counts.Unsupported == 1, "Hysteria v1 parse=%#v", parsed)

	item := parsed.Items[0]
	requireFixturef(t, item.Protocol == "hysteria", "Hysteria v1 diagnostic=%#v", item)
	requireFixturef(t, item.ErrorCode == "unsupported_hysteria_v1", "Hysteria v1 diagnostic=%#v", item)
	requireFixturef(t, item.DisplayName == "Legacy Hysteria", "Hysteria v1 diagnostic=%#v", item)
	requireFixturef(t, item.Host == "legacy-hy.example", "Hysteria v1 diagnostic=%#v", item)
	requireFixturef(t, item.Port == "8443", "Hysteria v1 diagnostic=%#v", item)

	ambiguous := parseExternalTestBody(t, `{"remarks":"Unknown Hysteria","outbounds":[{"protocol":"hysteria","settings":{"address":"unknown-hy.example","port":443},"streamSettings":{"network":"hysteria","hysteriaSettings":{"auth":"never-preview-this-either"}}},{"protocol":"freedom","settings":{}},{"protocol":"blackhole","settings":{}}]}`)
	requireFixturef(t, ambiguous.Counts.Unsupported == 1, "markerless Hysteria must remain ambiguous: %#v", ambiguous)
	requireFixturef(t, ambiguous.Items[0].Protocol == "hysteria-unknown", "markerless Hysteria must remain ambiguous: %#v", ambiguous)
	requireFixturef(t, ambiguous.Items[0].ErrorCode == "ambiguous_hysteria_version", "markerless Hysteria must remain ambiguous: %#v", ambiguous)

	unsupported := parseExternalTestBody(t, `{"outbounds":[{"tag":"Direct","protocol":"freedom","settings":{}}]}`)
	requireFixturef(t, unsupported.Counts.Unsupported == 1, "unsupported Xray diagnostic=%#v", unsupported)
	requireFixturef(t, unsupported.Items[0].Protocol == "freedom", "unsupported Xray diagnostic=%#v", unsupported)
	requireFixturef(t, unsupported.Items[0].ErrorCode == "unsupported_xray_protocol", "unsupported Xray diagnostic=%#v", unsupported)

	malformed := parseExternalTestBody(t, `{"outbounds":[{"tag":"Broken VLESS","protocol":"vless","settings":{"vnext":[]}}]}`)
	requireFixturef(t, malformed.Counts.Rejected == 1, "malformed VLESS diagnostic=%#v", malformed)
	requireFixturef(t, malformed.Items[0].Protocol == "vless", "malformed VLESS diagnostic=%#v", malformed)
	requireFixturef(t, malformed.Items[0].ErrorCode == "invalid_vless_json", "malformed VLESS diagnostic=%#v", malformed)

	withAuxiliary := parseExternalTestBody(t, `{"outbounds":[{"protocol":"hysteria","settings":{"version":2,"address":"hy.example","port":443},"streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2,"auth":"not-in-preview"}}},{"protocol":"freedom","settings":{}},{"protocol":"blackhole","settings":{}}]}`)
	requireFixturef(t, withAuxiliary.Counts.Accepted == 1, "Hysteria2 with routing helpers must import: %#v", withAuxiliary)
	requireFixturef(t, len(withAuxiliary.Keys) == 1, "Hysteria2 with routing helpers must import: %#v", withAuxiliary)
	requireFixturef(t, withAuxiliary.Keys[0].Protocol == "hysteria2", "Hysteria2 with routing helpers must import: %#v", withAuxiliary)

	malformedHysteria2 := parseExternalTestBody(t, `{"remarks":"Broken HY2","outbounds":[{"protocol":"hysteria","settings":{"version":2,"address":"broken-hy2.example","port":443},"streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2}}},{"protocol":"freedom","settings":{}},{"protocol":"blackhole","settings":{}}]}`)
	requireFixturef(t, malformedHysteria2.Counts.Rejected == 1, "malformed Hysteria2 diagnostic=%#v", malformedHysteria2)
	requireFixturef(t, malformedHysteria2.Items[0].Protocol == "hysteria2", "malformed Hysteria2 diagnostic=%#v", malformedHysteria2)
	requireFixturef(t, malformedHysteria2.Items[0].ErrorCode == "invalid_hysteria2_json", "malformed Hysteria2 diagnostic=%#v", malformedHysteria2)

	diagnosticItems := append(append([]sources.ImportItem{}, parsed.Items...), ambiguous.Items...)
	encoded, err := json.Marshal(diagnosticItems)
	requireFixturef(t, err == nil, "Hysteria v1 preview leaked credentials: json=%s err=%v", encoded, err)
	requireFixturef(t, !(strings.Contains(string(encoded), "never-preview-this")), "Hysteria v1 preview leaked credentials: json=%s err=%v", encoded, err)
	requireFixturef(t, !(strings.Contains(string(encoded), "never-preview-this-either")), "Hysteria v1 preview leaked credentials: json=%s err=%v", encoded, err)

}

func runtimeShapeHysteria2Fixture(label, host, auth string) string {
	return fmt.Sprintf(`{
		"remarks":%q,
		"dns":{},"inbounds":[],"log":{},"routing":{},
		"outbounds":[
			{"tag":"proxy","protocol":"hysteria","settings":{"version":2,"address":%q,"port":443},"streamSettings":{
				"network":"hysteria","security":"tls",
				"hysteriaSettings":{"version":2,"auth":%q},
				"tlsSettings":{"serverName":"sni.fixture.example","alpn":["h3"],"fingerprint":"chrome"},
				"finalmask":{"udp":[]}
			}},
			{"tag":"direct","protocol":"freedom","settings":{}},
			{"tag":"block","protocol":"blackhole","settings":{}}
		]
	}`, label, host, auth)
}

func TestExtractJSONSubscriptionLabelRejectsGenericRoutingTags(t *testing.T) {
	for _, tag := range []string{"proxy", "direct", "block", "dns", "freedom", "blackhole", "outbound"} {
		root := map[string]any{
			"tag":       tag,
			"meta":      map[string]any{"serverDescription": "Human source name"},
			"outbounds": []any{map[string]any{"tag": tag}},
		}
		{
			got := sources.ExtractJSONLabel(root, "Fallback")
			requireFixturef(t, got == "Human source name", "generic tag %q replaced human metadata: %q", tag, got)
		}

	}
	root := map[string]any{"outbounds": []any{map[string]any{"tag": "Meaningful edge name"}}}
	{
		got := sources.ExtractJSONLabel(root, "Fallback")
		requireFixturef(t, got == "Meaningful edge name", "meaningful outbound tag was discarded: %q", got)
	}

}

func TestExternalImporterRuntimeXrayHysteria2ShapeAndCandidateAccounting(t *testing.T) {
	standalone := runtimeShapeHysteria2Fixture("EE fixture", "ee-hy2.fixture.example", "fixture-auth-ee")
	parsedStandalone := parseExternalTestBody(t, standalone)
	requireFixturef(t, parsedStandalone.Counts.Accepted == 1, "runtime-shape Hysteria2 parse=%#v", parsedStandalone)
	requireFixturef(t, len(parsedStandalone.Keys) == 1, "runtime-shape Hysteria2 parse=%#v", parsedStandalone)
	requireFixturef(t, len(parsedStandalone.Items) == 1, "runtime-shape Hysteria2 parse=%#v", parsedStandalone)
	requireFixturef(t, parsedStandalone.Keys[0].Protocol == "hysteria2", "runtime-shape Hysteria2 parse=%#v", parsedStandalone)

	profile, err := profiles.Parse(parsedStandalone.Keys[0].URL)
	requireFixturef(t, err == nil, "parse runtime-shape Hysteria2 URI: %v", err)

	data := profile.Data.(profiles.Hysteria2Data)
	requireFixturef(t, profile.Server == "ee-hy2.fixture.example", "runtime-shape Hysteria2 data=%#v profile=%#v", data, profile)
	requireFixturef(t, data.Authentication.Reveal() == "fixture-auth-ee", "runtime-shape Hysteria2 data=%#v profile=%#v", data, profile)
	requireFixturef(t, data.SNI == "sni.fixture.example", "runtime-shape Hysteria2 data=%#v profile=%#v", data, profile)

	extensions := map[string]bool{}
	for _, parameter := range profile.UnknownQueryParameters {
		extensions[parameter.Key] = true
	}
	requireFixturef(t, extensions["alpn"], "Xray TLS extensions were not preserved: %#v", profile.UnknownQueryParameters)
	requireFixturef(t, extensions["fp"], "Xray TLS extensions were not preserved: %#v", profile.UnknownQueryParameters)

	aggregate := `{
		"remarks":"Best server fixture",
		"outbounds":[
			{"protocol":"vless","settings":{"vnext":[{"address":"vless.fixture.example","port":443,"users":[{"id":"11111111-1111-4111-8111-111111111111"}]}]}},
			{"protocol":"hysteria","settings":{"version":2,"address":"aggregate-hy.fixture.example","port":443},"streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2,"auth":"aggregate-fixture-auth"}}},
			{"protocol":"freedom","settings":{}},{"protocol":"blackhole","settings":{}}
		]
	}`
	objects := []string{aggregate}
	for index := 0; index < 4; index++ {
		objects = append(objects, runtimeShapeHysteria2Fixture(fmt.Sprintf("HY2 fixture %d", index+1), fmt.Sprintf("hy2-%d.fixture.example", index+1), fmt.Sprintf("fixture-auth-%d", index+1)))
	}
	parsed := parseExternalTestBody(t, "["+strings.Join(objects, ",")+"]")
	requireFixturef(t, len(parsed.Keys) == len(objects), "candidate accounting lost an object: keys=%d items=%d counts=%#v", len(parsed.Keys), len(parsed.Items), parsed.Counts)
	requireFixturef(t, len(parsed.Items) == len(objects), "candidate accounting lost an object: keys=%d items=%d counts=%#v", len(parsed.Keys), len(parsed.Items), parsed.Counts)
	requireFixturef(t, parsed.Counts.Accepted == len(objects), "candidate accounting lost an object: keys=%d items=%d counts=%#v", len(parsed.Keys), len(parsed.Items), parsed.Counts)
	requireFixturef(t, parsed.Counts.Rejected == 0, "candidate accounting lost an object: keys=%d items=%d counts=%#v", len(parsed.Keys), len(parsed.Items), parsed.Counts)
	requireFixturef(t, parsed.Counts.Unsupported == 0, "candidate accounting lost an object: keys=%d items=%d counts=%#v", len(parsed.Keys), len(parsed.Items), parsed.Counts)

	requireFixturef(t, parsed.Keys[0].Protocol == "xray-json", "multi-profile aggregate was reduced instead of retained as XRAY-JSON: %#v", parsed.Keys[0])

	preview, err := json.Marshal(parsed.Items)
	requireFixture(t, err == nil, err)

	for _, secret := range []string{"fixture-auth-ee", "aggregate-fixture-auth", "fixture-auth-1", `"outbounds"`, "hysteria2://"} {
		requireFixturef(t, !(strings.Contains(string(preview), secret)), "runtime-shape preview leaked %q: %s", secret, preview)

	}
}
