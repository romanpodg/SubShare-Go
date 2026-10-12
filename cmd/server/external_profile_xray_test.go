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
			if parsed.DetectedFormat != "xray-json" || len(parsed.Keys) != 1 || parsed.Keys[0].Protocol != "xray-json" || parsed.Keys[0].Host != test.wantHost {
				t.Fatalf("Xray compatibility result=%#v", parsed)
			}
		})
	}
	arrayBody := `[
		{"remarks":"Array VLESS","outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"array-vless.example","port":443,"users":[{"id":"77777777-7777-4777-8777-777777777777"}]}]}}]},
		{"remarks":"Array Trojan","outbounds":[{"protocol":"trojan","settings":{"servers":[{"address":"array-trojan.example","port":443,"password":"array-secret"}]}}]}
	]`
	arrayParsed := parseExternalTestBody(t, arrayBody)
	if arrayParsed.DetectedFormat != "xray-json" || len(arrayParsed.Keys) != 2 || arrayParsed.Keys[0].Host != "array-vless.example" || arrayParsed.Keys[1].Host != "array-trojan.example" {
		t.Fatalf("Xray JSON array compatibility result=%#v", arrayParsed)
	}

	for _, raw := range []string{
		`{"outbounds":[]}`,
		`{"valid":"json but not xray"}`,
	} {
		parsed := parseExternalTestBody(t, raw)
		if len(parsed.Keys) != 0 || parsed.Counts.Rejected != 1 || parsed.Items[0].ErrorCode != "invalid_xray_json" {
			t.Fatalf("unsupported Xray-shaped JSON result=%#v", parsed)
		}
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
	if parsed.Counts.Accepted != 1 || len(parsed.Keys) != 1 || parsed.Keys[0].Protocol != "hysteria2" {
		t.Fatalf("Hysteria2 JSON parse=%#v", parsed)
	}
	profile, err := profiles.Parse(parsed.Keys[0].URL)
	if err != nil {
		t.Fatalf("parse imported Hysteria2 URI: %v", err)
	}
	data, ok := profile.Data.(profiles.Hysteria2Data)
	if !ok || profile.Server != "hy-json.example" || profile.Port.Expression != "443,5000-5010" ||
		profile.DisplayName != "HY2 JSON" ||
		data.Authentication.Reveal() != "json-hysteria-auth" || data.SNI != "tls-json.example" ||
		data.CertificateSHA256 != "abababababababababababababababababababababababababababababababab" ||
		data.ObfuscationType != "salamander" || data.ObfuscationPassword.Reveal() != "json-obfs-secret" {
		t.Fatalf("imported Hysteria2 profile=%#v data=%#v", profile, data)
	}
	if parsed.Keys[0].Fingerprint == "" {
		t.Fatal("semantic fingerprint missing")
	}
	encoded, err := json.Marshal(parsed.Items)
	if err != nil {
		t.Fatalf("marshal preview: %v", err)
	}
	for _, secret := range []string{"json-hysteria-auth", "json-obfs-secret", "hysteria2://", `"outbounds"`} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("preview leaked %q: %s", secret, encoded)
		}
	}
}

func TestExternalImporterClassifiesHysteriaV1AndIdentifiableXrayFailures(t *testing.T) {
	v1 := `{"remarks":"Legacy Hysteria","outbounds":[{"protocol":"hysteria","settings":{"version":1,"address":"legacy-hy.example","port":8443},"streamSettings":{"hysteriaSettings":{"version":1,"auth":"never-preview-this"}}}]}`
	parsed := parseExternalTestBody(t, v1)
	if len(parsed.Keys) != 0 || parsed.Counts.Unsupported != 1 {
		t.Fatalf("Hysteria v1 parse=%#v", parsed)
	}
	item := parsed.Items[0]
	if item.Protocol != "hysteria" || item.ErrorCode != "unsupported_hysteria_v1" || item.DisplayName != "Legacy Hysteria" || item.Host != "legacy-hy.example" || item.Port != "8443" {
		t.Fatalf("Hysteria v1 diagnostic=%#v", item)
	}
	ambiguous := parseExternalTestBody(t, `{"remarks":"Unknown Hysteria","outbounds":[{"protocol":"hysteria","settings":{"address":"unknown-hy.example","port":443},"streamSettings":{"network":"hysteria","hysteriaSettings":{"auth":"never-preview-this-either"}}},{"protocol":"freedom","settings":{}},{"protocol":"blackhole","settings":{}}]}`)
	if ambiguous.Counts.Unsupported != 1 || ambiguous.Items[0].Protocol != "hysteria-unknown" || ambiguous.Items[0].ErrorCode != "ambiguous_hysteria_version" {
		t.Fatalf("markerless Hysteria must remain ambiguous: %#v", ambiguous)
	}

	unsupported := parseExternalTestBody(t, `{"outbounds":[{"tag":"Direct","protocol":"freedom","settings":{}}]}`)
	if unsupported.Counts.Unsupported != 1 || unsupported.Items[0].Protocol != "freedom" || unsupported.Items[0].ErrorCode != "unsupported_xray_protocol" {
		t.Fatalf("unsupported Xray diagnostic=%#v", unsupported)
	}

	malformed := parseExternalTestBody(t, `{"outbounds":[{"tag":"Broken VLESS","protocol":"vless","settings":{"vnext":[]}}]}`)
	if malformed.Counts.Rejected != 1 || malformed.Items[0].Protocol != "vless" || malformed.Items[0].ErrorCode != "invalid_vless_json" {
		t.Fatalf("malformed VLESS diagnostic=%#v", malformed)
	}

	withAuxiliary := parseExternalTestBody(t, `{"outbounds":[{"protocol":"hysteria","settings":{"version":2,"address":"hy.example","port":443},"streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2,"auth":"not-in-preview"}}},{"protocol":"freedom","settings":{}},{"protocol":"blackhole","settings":{}}]}`)
	if withAuxiliary.Counts.Accepted != 1 || len(withAuxiliary.Keys) != 1 || withAuxiliary.Keys[0].Protocol != "hysteria2" {
		t.Fatalf("Hysteria2 with routing helpers must import: %#v", withAuxiliary)
	}

	malformedHysteria2 := parseExternalTestBody(t, `{"remarks":"Broken HY2","outbounds":[{"protocol":"hysteria","settings":{"version":2,"address":"broken-hy2.example","port":443},"streamSettings":{"network":"hysteria","security":"tls","hysteriaSettings":{"version":2}}},{"protocol":"freedom","settings":{}},{"protocol":"blackhole","settings":{}}]}`)
	if malformedHysteria2.Counts.Rejected != 1 || malformedHysteria2.Items[0].Protocol != "hysteria2" || malformedHysteria2.Items[0].ErrorCode != "invalid_hysteria2_json" {
		t.Fatalf("malformed Hysteria2 diagnostic=%#v", malformedHysteria2)
	}

	diagnosticItems := append(append([]sources.ImportItem{}, parsed.Items...), ambiguous.Items...)
	encoded, err := json.Marshal(diagnosticItems)
	if err != nil || strings.Contains(string(encoded), "never-preview-this") || strings.Contains(string(encoded), "never-preview-this-either") {
		t.Fatalf("Hysteria v1 preview leaked credentials: json=%s err=%v", encoded, err)
	}
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
		if got := sources.ExtractJSONLabel(root, "Fallback"); got != "Human source name" {
			t.Fatalf("generic tag %q replaced human metadata: %q", tag, got)
		}
	}
	root := map[string]any{"outbounds": []any{map[string]any{"tag": "Meaningful edge name"}}}
	if got := sources.ExtractJSONLabel(root, "Fallback"); got != "Meaningful edge name" {
		t.Fatalf("meaningful outbound tag was discarded: %q", got)
	}
}

func TestExternalImporterRuntimeXrayHysteria2ShapeAndCandidateAccounting(t *testing.T) {
	standalone := runtimeShapeHysteria2Fixture("EE fixture", "ee-hy2.fixture.example", "fixture-auth-ee")
	parsedStandalone := parseExternalTestBody(t, standalone)
	if parsedStandalone.Counts.Accepted != 1 || len(parsedStandalone.Keys) != 1 || len(parsedStandalone.Items) != 1 || parsedStandalone.Keys[0].Protocol != "hysteria2" {
		t.Fatalf("runtime-shape Hysteria2 parse=%#v", parsedStandalone)
	}
	profile, err := profiles.Parse(parsedStandalone.Keys[0].URL)
	if err != nil {
		t.Fatalf("parse runtime-shape Hysteria2 URI: %v", err)
	}
	data := profile.Data.(profiles.Hysteria2Data)
	if profile.Server != "ee-hy2.fixture.example" || data.Authentication.Reveal() != "fixture-auth-ee" || data.SNI != "sni.fixture.example" {
		t.Fatalf("runtime-shape Hysteria2 data=%#v profile=%#v", data, profile)
	}
	extensions := map[string]bool{}
	for _, parameter := range profile.UnknownQueryParameters {
		extensions[parameter.Key] = true
	}
	if !extensions["alpn"] || !extensions["fp"] {
		t.Fatalf("Xray TLS extensions were not preserved: %#v", profile.UnknownQueryParameters)
	}

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
	if len(parsed.Keys) != len(objects) || len(parsed.Items) != len(objects) || parsed.Counts.Accepted != len(objects) || parsed.Counts.Rejected != 0 || parsed.Counts.Unsupported != 0 {
		t.Fatalf("candidate accounting lost an object: keys=%d items=%d counts=%#v", len(parsed.Keys), len(parsed.Items), parsed.Counts)
	}
	if parsed.Keys[0].Protocol != "xray-json" {
		t.Fatalf("multi-profile aggregate was reduced instead of retained as XRAY-JSON: %#v", parsed.Keys[0])
	}
	preview, err := json.Marshal(parsed.Items)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"fixture-auth-ee", "aggregate-fixture-auth", "fixture-auth-1", `"outbounds"`, "hysteria2://"} {
		if strings.Contains(string(preview), secret) {
			t.Fatalf("runtime-shape preview leaked %q: %s", secret, preview)
		}
	}
}
