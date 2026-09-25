package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

var externalTestFingerprintKey = []byte("0123456789abcdef0123456789abcdef")

const (
	externalTestVLESS  = "vless://11111111-1111-4111-8111-111111111111@vless.example:443?security=tls#VLESS"
	externalTestVMess  = "vmess://eyJ2IjoiMiIsInBzIjoiVk1lc3MiLCJhZGQiOiJ2bWVzcy5leGFtcGxlIiwicG9ydCI6IjQ0MyIsImlkIjoiMjIyMjIyMjItMjIyMi00MjIyLTgyMjItMjIyMjIyMjIyMjIyIn0="
	externalTestTrojan = "trojan://trojan-password@trojan.example:443#Trojan"
	externalTestSS     = "ss://YWVzLTI1Ni1nY206c2hhZG93LXBhc3N3b3Jk@ss.example:8388?plugin=local%3Bpsk%3Dplugin-secret#SS"
	externalTestHY2    = "hysteria2://hysteria-auth@hy.example:443,5000-5010?sni=tls.example&obfs=salamander&obfs-password=obfs-secret#HY2"
	externalTestTUIC   = "tuic://33333333-3333-4333-8333-333333333333:tuic-password@tuic.example:443?sni=tls.example&congestion_control=bbr#TUIC"
	externalTestTUICV4 = "tuic://legacy-token-value@tuic-v4.example:443?sni=tls.example#TUIC-v4"
	externalTestXray   = `{"outbounds":[{"protocol":"vless","tag":"Xray","settings":{"vnext":[{"address":"xray.example","port":443,"users":[{"id":"44444444-4444-4444-8444-444444444444"}]}]}}]}`
)

func parseExternalTestBody(t *testing.T, body string) sources.ParseResult {
	t.Helper()
	parsed, err := sources.ParseBody(body, [][]byte{externalTestFingerprintKey})
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	return parsed
}

func TestExternalProviderAnnouncementMetadataRemainsIndependent(t *testing.T) {
	providerAnnouncement := "Сообщение внешнего провайдера"
	encoded := "base64:" + base64.StdEncoding.EncodeToString([]byte(providerAnnouncement))
	if got := sources.DecodeHeaderValue(encoded); got != providerAnnouncement {
		t.Fatalf("decoded provider announcement=%q", got)
	}

	app := newIntegrationApp(t)
	if _, err := app.db.Exec(`
		INSERT INTO external_subscription_sources(name, source_url, meta_announce)
		VALUES('Provider', 'https://provider.example/subscription', ?)
	`, providerAnnouncement); err != nil {
		t.Fatal(err)
	}
	var sourceID int64
	if err := app.db.QueryRow(`SELECT id FROM external_subscription_sources WHERE source_url = 'https://provider.example/subscription'`).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	source, err := app.getExternalSourceByID(sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if source.MetaAnnounce != providerAnnouncement {
		t.Fatalf("external meta_announce was not preserved: %#v", source)
	}
	settings, err := app.getSubscriptionSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.ExtraStatus != "" {
		t.Fatalf("external announcement was promoted globally: %q", settings.ExtraStatus)
	}
}

func TestExternalProfileInputRecognitionAndMixedPartialSuccess(t *testing.T) {
	body := strings.Join([]string{
		externalTestVLESS,
		externalTestVMess,
		externalTestTrojan,
		externalTestSS,
		strings.Replace(externalTestHY2, "hysteria2://", "hy2://", 1),
		externalTestTUIC,
		externalTestXray,
		"tuic://not-a-uuid:password@example.com:443",
		"hysteria://legacy@example.com:443",
		"hysteria2+realm://secret@example.com/realm",
	}, "\n")
	parsed := parseExternalTestBody(t, body)
	if parsed.DetectedFormat != "links" || len(parsed.Keys) != 7 {
		t.Fatalf("format=%q keys=%d items=%#v", parsed.DetectedFormat, len(parsed.Keys), parsed.Items)
	}
	if parsed.Counts.Accepted != 7 || parsed.Counts.Rejected != 1 || parsed.Counts.Unsupported != 2 {
		t.Fatalf("unexpected counts: %#v", parsed.Counts)
	}
	wantProtocols := []string{"vless", "vmess", "trojan", "shadowsocks", "hysteria2", "tuic", "xray-json"}
	for index, want := range wantProtocols {
		if parsed.Keys[index].Protocol != want || parsed.Keys[index].LineIndex != index+1 {
			t.Fatalf("key %d = %#v, want protocol %q", index, parsed.Keys[index], want)
		}
	}
	if !sources.ContainsWarningCode(parsed.Warnings, "partial_import") {
		t.Fatalf("partial import warning missing: %#v", parsed.Warnings)
	}
}

func TestExternalProfileBase64AliasCompatibilityAndSafePreview(t *testing.T) {
	plain := strings.Join([]string{
		externalTestSS,
		strings.Replace(externalTestHY2, "hysteria2://", "hy2://", 1),
		externalTestTUICV4,
	}, "\n")
	wrapped := base64.StdEncoding.EncodeToString([]byte(plain))
	parsed := parseExternalTestBody(t, wrapped)
	if len(parsed.Keys) != 3 || !sources.ContainsWarningCode(parsed.Warnings, "base64_decoded") {
		t.Fatalf("unexpected base64 parse: keys=%d warnings=%#v", len(parsed.Keys), parsed.Warnings)
	}
	if parsed.Items[2].Status != sources.StatusCompatibilityOnly || parsed.Items[2].Compatibility != "read_only" {
		t.Fatalf("TUIC v4 item = %#v", parsed.Items[2])
	}
	encoded, err := json.Marshal(parsed.Items)
	if err != nil {
		t.Fatalf("marshal safe items: %v", err)
	}
	unsafe := string(encoded)
	for _, secret := range []string{"shadow-password", "plugin-secret", "hysteria-auth", "obfs-secret", "33333333-3333-4333-8333-333333333333", "tuic-password", "legacy-token-value", "ss://", "hy2://", "tuic://"} {
		if strings.Contains(unsafe, secret) {
			t.Fatalf("safe preview exposed %q: %s", secret, unsafe)
		}
	}
	for _, item := range parsed.Items {
		if !strings.HasPrefix(item.ItemRef, "ir1_") || item.LineIndex == 0 || item.Protocol == "" {
			t.Fatalf("unsafe/incomplete item: %#v", item)
		}
	}
}

func TestExternalImporterBase64DetectionPolicy(t *testing.T) {
	plain := parseExternalTestBody(t, externalTestSS)
	if sources.ContainsWarningCode(plain.Warnings, "base64_decoded") || len(plain.Keys) != 1 {
		t.Fatalf("plain URI was treated as Base64: %#v", plain)
	}

	body := externalTestSS + "\n" + externalTestTUIC
	encodings := map[string]string{
		"standard padded":   base64.StdEncoding.EncodeToString([]byte(body)),
		"standard unpadded": base64.RawStdEncoding.EncodeToString([]byte(body)),
		"url padded":        base64.URLEncoding.EncodeToString([]byte(body)),
		"url unpadded":      base64.RawURLEncoding.EncodeToString([]byte(body)),
		"explicit marker":   "BASE64:" + base64.StdEncoding.EncodeToString([]byte(body)),
	}
	for name, encoded := range encodings {
		t.Run(name, func(t *testing.T) {
			parsed := parseExternalTestBody(t, encoded)
			if len(parsed.Keys) != 2 || !sources.ContainsWarningCode(parsed.Warnings, "base64_decoded") {
				t.Fatalf("Base64 policy result=%#v", parsed)
			}
		})
	}

	for name, encoded := range map[string]string{
		"invalid length":             strings.Repeat("A", 25),
		"explicit malformed":         "base64:not-valid-%%%",
		"mixed alphabets":            base64.RawStdEncoding.EncodeToString([]byte(body)) + "-_",
		"unsupported decoded format": base64.RawStdEncoding.EncodeToString([]byte("provider-secret-without-a-supported-format")),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := sources.ParseBody(encoded, [][]byte{externalTestFingerprintKey})
			if err == nil || err.Error() != "invalid_base64_subscription" || strings.Contains(err.Error(), "provider-secret") {
				t.Fatalf("malformed Base64 error=%v", err)
			}
		})
	}
	if _, err := sources.ParseBody(strings.Repeat("A", sources.MaxBodyBytes+1), [][]byte{externalTestFingerprintKey}); err == nil || err.Error() != "subscription_body_too_large" {
		t.Fatalf("oversized encoded body error=%v", err)
	}
}

func TestExternalProfileSchemeRecognitionIsCaseInsensitiveAndExplicit(t *testing.T) {
	upper := strings.Replace(externalTestSS, "ss://", "SS://", 1)
	parsed := parseExternalTestBody(t, upper)
	if len(parsed.Keys) != 1 || parsed.Keys[0].Protocol != "shadowsocks" {
		t.Fatalf("uppercase scheme was not recognized: %#v", parsed)
	}
	notScheme := parseExternalTestBody(t, "prefix "+externalTestSS)
	if len(notScheme.Keys) != 0 || notScheme.Counts.Unsupported != 1 {
		t.Fatalf("substring was treated as a scheme: %#v", notScheme)
	}
}

func TestExternalImporterDoesNotGuessProviderJSONOrSIP008(t *testing.T) {
	for _, body := range []string{
		`{"server":"ss.example","server_port":8388,"method":"aes-256-gcm","password":"secret"}`,
		`[{"server":"ss.example","server_port":8388,"method":"aes-256-gcm","password":"secret"}]`,
	} {
		parsed := parseExternalTestBody(t, body)
		if len(parsed.Keys) != 0 || parsed.Counts.Unsupported != 1 || parsed.Items[0].ErrorCode != "unsupported_sip008_format" {
			t.Fatalf("SIP008 was not classified as deferred input: %#v", parsed)
		}
		encoded, err := json.Marshal(parsed.Items)
		if err != nil || strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "ss.example") {
			t.Fatalf("SIP008 rejection exposed input: json=%s err=%v", encoded, err)
		}
		if _, err := sources.ParseRawBody("https://provider.example/sip008", body, sources.Metadata{}, [][]byte{externalTestFingerprintKey}); err == nil || err.Error() != "unsupported_sip008_format" {
			t.Fatalf("SIP008 source-level error=%v", err)
		}
	}
	providerSecret := "provider-json-secret"
	parsed := parseExternalTestBody(t, `{"endpoint":"provider.example","credential":"`+providerSecret+`"}`)
	if len(parsed.Keys) != 0 || parsed.Counts.Rejected != 1 || parsed.Items[0].ErrorCode != "invalid_xray_json" {
		t.Fatalf("arbitrary provider JSON was guessed: %#v", parsed)
	}
	formatted := fmt.Sprintf("%v %#v", parsed.Items, parsed.Items)
	if strings.Contains(formatted, providerSecret) || strings.Contains(formatted, "provider.example") {
		t.Fatalf("provider JSON rejection exposed input: %s", formatted)
	}
}

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

func TestExternalProfileSemanticDeduplication(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		duplicates int
		ambiguous  int
		keys       int
	}{
		{
			name: "tag and query order",
			body: "hy2://auth@example.com?sni=tls.example&x=1#one\n" +
				"hysteria2://auth@example.com:443?x=1&sni=tls.example#two",
			duplicates: 1, keys: 1,
		},
		{
			name:       "omitted versus explicit default port",
			body:       "hy2://auth@example.com#one\nhy2://auth@example.com:443#two",
			duplicates: 1, keys: 1,
		},
		{
			name: "same-key duplicate ordering remains distinct",
			body: "hy2://auth@example.com?sni=first.example&sni=second.example\n" +
				"hy2://auth@example.com?sni=second.example&sni=first.example",
			ambiguous: 2, keys: 2,
		},
		{
			name: "unknown duplicate ordering remains distinct",
			body: "hy2://auth@example.com?extension=first&extension=second\n" +
				"hy2://auth@example.com?extension=second&extension=first",
			ambiguous: 2, keys: 2,
		},
		{
			name: "different-key ordering with duplicate order preserved",
			body: "hy2://auth@example.com?extension=first&extension=second&other=value\n" +
				"hy2://auth@example.com?other=value&extension=first&extension=second",
			duplicates: 1, ambiguous: 2, keys: 1,
		},
		{
			name: "conflicting duplicate does not collapse into unambiguous",
			body: "hy2://auth@example.com?sni=first.example\n" +
				"hy2://auth@example.com?sni=first.example&sni=second.example",
			ambiguous: 1, keys: 2,
		},
		{
			name: "TUIC aliases normalize",
			body: "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?congestion-controller=bbr&udp-relay-mode=quic\n" +
				"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?udp_relay_mode=quic&congestion_control=bbr",
			duplicates: 1, keys: 1,
		},
		{
			name: "different credentials",
			body: "hy2://first@example.com\nhy2://second@example.com",
			keys: 2,
		},
		{
			name: "TUIC v4 and v5",
			body: externalTestTUICV4 + "\n" + externalTestTUIC,
			keys: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := parseExternalTestBody(t, test.body)
			if len(parsed.Keys) != test.keys || parsed.Counts.Duplicate != test.duplicates || parsed.Counts.Ambiguous != test.ambiguous {
				t.Fatalf("keys=%d counts=%#v items=%#v", len(parsed.Keys), parsed.Counts, parsed.Items)
			}
		})
	}
}

func TestExternalProfileConnectivityChangesRemainDistinct(t *testing.T) {
	tests := []string{
		"hy2://auth@example.com?sni=one.example\nhy2://auth@example.com?sni=two.example",
		"hy2://auth@example.com?obfs=salamander&obfs-password=one\nhy2://auth@example.com?obfs=salamander&obfs-password=two",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?skip-cert-verify=0\n" +
			"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?skip-cert-verify=1",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?udp-relay-mode=native\n" +
			"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?udp-relay-mode=quic",
	}
	for _, body := range tests {
		parsed := parseExternalTestBody(t, body)
		if len(parsed.Keys) != 2 || parsed.Counts.Duplicate != 0 || parsed.Keys[0].Fingerprint == parsed.Keys[1].Fingerprint {
			t.Fatalf("connectivity-changing profiles collapsed: %#v", parsed)
		}
	}
	contradictory := parseExternalTestBody(t, "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=first.example&server_name=second.example")
	if contradictory.Counts.Ambiguous != 1 || !sources.ContainsWarningCode(contradictory.Items[0].Warnings, profiles.WarningAmbiguousParameter) {
		t.Fatalf("contradictory TUIC aliases were not marked ambiguous: %#v", contradictory)
	}
}

func TestExternalProfileFingerprintKeyAndSelectionIntegrity(t *testing.T) {
	if _, err := sources.ParseBody(externalTestSS, nil); err == nil || err.Error() != "profile_fingerprint_key_unavailable" {
		t.Fatalf("nil fingerprint key error = %v", err)
	}
	parsed := parseExternalTestBody(t, externalTestSS+"\n"+externalTestHY2)
	selected, err := sources.FilterSelection(parsed, []string{parsed.Items[1].ItemRef})
	if err != nil || len(selected.Keys) != 1 || selected.Keys[0].Protocol != "hysteria2" {
		t.Fatalf("selection result=%#v err=%v", selected, err)
	}
	for _, refs := range [][]string{{"ir1_tampered"}, {parsed.Items[0].ItemRef, parsed.Items[0].ItemRef}} {
		if _, err := sources.FilterSelection(parsed, refs); err == nil {
			t.Fatalf("tampered selection %#v was accepted", refs)
		}
	}

	identical := parseExternalTestBody(t, externalTestSS+"\n"+externalTestSS)
	if len(identical.Keys) != 1 || len(identical.Items) != 2 || identical.Items[1].Status != sources.StatusDuplicate {
		t.Fatalf("identical-line duplicate result = %#v", identical)
	}
	if identical.Items[0].ItemRef == identical.Items[1].ItemRef {
		t.Fatalf("identical source lines share item reference %q", identical.Items[0].ItemRef)
	}
	if _, err := sources.FilterSelection(identical, []string{identical.Items[1].ItemRef}); err == nil {
		t.Fatal("duplicate-only preview occurrence was accepted for persistence")
	}
}

func seedExternalProfileSource(t *testing.T, app *App, sourceURL string) int64 {
	t.Helper()
	result, err := app.db.Exec(`INSERT INTO external_subscription_sources(name, category, key_category, key_insert_mode, source_url, enabled, import_status) VALUES('profiles', 'general', '', 'bottom', ?, 1, 'idle')`, sourceURL)
	if err != nil {
		t.Fatalf("insert source: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

func seedExternalRepairUser(t *testing.T, app *App, suffix string) int64 {
	t.Helper()
	result, err := app.db.Exec(
		`INSERT INTO users(name, token, status, key_assignment_mode) VALUES(?, ?, 'active', 'selected')`,
		"repair-"+suffix,
		"repair-token-"+suffix,
	)
	if err != nil {
		t.Fatalf("insert repair user: %v", err)
	}
	id, _ := result.LastInsertId()
	return id
}

func insertHistoricalExternalProfile(t *testing.T, app *App, sourceID int64, raw, protocol, fingerprint string, userIDs ...int64) int64 {
	t.Helper()
	result, err := app.db.Exec(`
		INSERT INTO vless_keys(
			label, status, key_kind, external_source_id, external_key_ref, protocol,
			profile_fingerprint, profile_schema_version, profile_compatibility
		) VALUES('historical', 'active', 'real', ?, ?, ?, ?, 0, 'legacy')
	`, sourceID, sources.KeyRef(raw), protocol, nullStringValue(fingerprint))
	if err != nil {
		t.Fatalf("insert historical profile: %v", err)
	}
	keyID, _ := result.LastInsertId()
	activeID, activeKey, err := app.profileKeyring.GetActiveEncryptionKey()
	if err != nil {
		t.Fatalf("load encryption key: %v", err)
	}
	envelope, err := profilestorage.Encrypt([]byte(raw), activeID, activeKey, keyID)
	if err != nil {
		t.Fatalf("encrypt historical profile: %v", err)
	}
	if _, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, envelope); err != nil {
		t.Fatalf("insert historical profile secret: %v", err)
	}
	for _, userID := range userIDs {
		if _, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, keyID); err != nil {
			t.Fatalf("assign historical profile: %v", err)
		}
	}
	return keyID
}

func sourceProfileCount(t *testing.T, app *App, sourceID int64) int {
	t.Helper()
	var count int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&count); err != nil {
		t.Fatalf("count source profiles: %v", err)
	}
	return count
}

func TestExternalProfileSyncRepairsHistoricalDuplicatesAndMergesAssignments(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/historical-repair")
	firstUser := seedExternalRepairUser(t, app, "first")
	secondUser := seedExternalRepairUser(t, app, "second")
	firstRaw := strings.Replace(externalTestVLESS, "#VLESS", "#Historical first", 1)
	secondRaw := strings.Replace(externalTestVLESS, "#VLESS", "#Historical second", 1)
	blankLowerID := insertHistoricalExternalProfile(t, app, sourceID, firstRaw, "vless", "", firstUser)
	currentFingerprint := parseExternalTestBody(t, externalTestVLESS).Keys[0].Fingerprint
	ownerID := insertHistoricalExternalProfile(t, app, sourceID, secondRaw, "vless", currentFingerprint, secondUser)
	if ownerID <= blankLowerID {
		t.Fatalf("test fixture IDs are not ordered: blank=%d owner=%d", blankLowerID, ownerID)
	}
	if _, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = 'Historical override' WHERE id = ?`, blankLowerID); err != nil {
		t.Fatalf("set duplicate override: %v", err)
	}

	result, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, externalTestVLESS))
	if err != nil {
		t.Fatalf("repair sync failed: %v", err)
	}
	if got := sourceProfileCount(t, app, sourceID); got != 1 {
		t.Fatalf("source rows=%d want=1 result=%#v", got, result)
	}
	var survivorID int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&survivorID); err != nil {
		t.Fatal(err)
	}
	if survivorID != ownerID {
		t.Fatalf("survivor=%d want current-fingerprint owner=%d", survivorID, ownerID)
	}
	var clientDisplayName string
	if err := app.db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = ?`, survivorID).Scan(&clientDisplayName); err != nil || clientDisplayName != "Historical override" {
		t.Fatalf("merged client override=%q err=%v", clientDisplayName, err)
	}
	var assignments int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE key_id = ? AND user_id IN (?, ?)`, survivorID, firstUser, secondUser).Scan(&assignments); err != nil || assignments != 2 {
		t.Fatalf("merged assignments=%d want=2 err=%v", assignments, err)
	}
}

func TestExternalProfileSyncPreservesLocalStatusAndClientDisplayName(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/client-name-override")
	initialRaw := strings.Replace(externalTestVLESS, "#VLESS", "#Source A", 1)
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, initialRaw)); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	var keyID int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&keyID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.Exec(`UPDATE vless_keys SET status = 'non-active', client_display_name = 'CUSTOM' WHERE id = ?`, keyID); err != nil {
		t.Fatal(err)
	}

	renamedRaw := strings.Replace(externalTestVLESS, "#VLESS", "#Source B", 1)
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, renamedRaw)); err != nil {
		t.Fatalf("renamed sync: %v", err)
	}
	var label, status string
	var override sql.NullString
	if err := app.db.QueryRow(`SELECT label, status, client_display_name FROM vless_keys WHERE id = ?`, keyID).Scan(&label, &status, &override); err != nil {
		t.Fatal(err)
	}
	if label != "Source B" || status != "non-active" || !override.Valid || override.String != "CUSTOM" {
		t.Fatalf("after source update label=%q status=%q override=%#v", label, status, override)
	}
	connectionUpdatedRaw := strings.Replace(renamedRaw, "vless.example", "rotated-vless.example", 1)
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, connectionUpdatedRaw)); err != nil {
		t.Fatalf("connection update sync: %v", err)
	}
	var updatedEnvelope string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&updatedEnvelope); err != nil {
		t.Fatal(err)
	}
	updatedSecret, err := profilestorage.Decrypt(updatedEnvelope, app.profileKeyring, keyID)
	if err != nil || updatedSecret.Reveal() != connectionUpdatedRaw {
		t.Fatalf("source-controlled configuration was not updated: err=%v raw=%q", err, updatedSecret.Reveal())
	}
	if err := app.db.QueryRow(`SELECT status, client_display_name FROM vless_keys WHERE id = ?`, keyID).Scan(&status, &override); err != nil {
		t.Fatal(err)
	}
	if status != "non-active" || !override.Valid || override.String != "CUSTOM" {
		t.Fatalf("connection update lost local metadata: status=%q override=%#v", status, override)
	}

	if _, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = NULL WHERE id = ?`, keyID); err != nil {
		t.Fatal(err)
	}
	latestRaw := strings.Replace(connectionUpdatedRaw, "#Source B", "#Source C", 1)
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, latestRaw)); err != nil {
		t.Fatalf("post-reset sync: %v", err)
	}
	if err := app.db.QueryRow(`SELECT label, status, client_display_name FROM vless_keys WHERE id = ?`, keyID).Scan(&label, &status, &override); err != nil {
		t.Fatal(err)
	}
	if label != "Source C" || status != "non-active" || override.Valid {
		t.Fatalf("after reset sync label=%q status=%q override=%#v", label, status, override)
	}
	var envelope string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelope); err != nil {
		t.Fatal(err)
	}
	secret, err := profilestorage.Decrypt(envelope, app.profileKeyring, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if effective := profileconfig.EffectiveClientDisplayName("", secret.Reveal(), label, true); effective != "Source C" {
		t.Fatalf("post-reset effective client name=%q", effective)
	}

	userID := seedSubscriptionUser(t, app, "active")
	if _, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, keyID); err != nil {
		t.Fatal(err)
	}
	generated, _, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(generated.Body, "vless.example") {
		t.Fatalf("inactive source profile was delivered: %q", generated.Body)
	}
}

func TestExternalProfileSyncRepairsBlankDuplicatesDuringPartialImport(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/partial-repair")
	firstRaw := strings.Replace(externalTestHY2, "#HY2", "#first", 1)
	secondRaw := strings.Replace(externalTestHY2, "#HY2", "#second", 1)
	secondRaw = strings.Replace(secondRaw, "hysteria2://", "hy2://", 1)
	firstID := insertHistoricalExternalProfile(t, app, sourceID, firstRaw, "hysteria2", "")
	insertHistoricalExternalProfile(t, app, sourceID, secondRaw, "hysteria2", "")
	missingID := insertHistoricalExternalProfile(t, app, sourceID, externalTestVMess, "vmess", "")

	parsed := parseExternalTestBody(t, externalTestHY2+"\n"+"hysteria://unsupported.example:443")
	result, err := app.syncExternalSource(sourceID, parsed)
	if err != nil {
		t.Fatalf("partial repair sync failed: %v", err)
	}
	if result.Counts.Removed != 1 || sourceProfileCount(t, app, sourceID) != 2 {
		t.Fatalf("partial repair result=%#v rows=%d", result, sourceProfileCount(t, app, sourceID))
	}
	for _, keyID := range []int64{firstID, missingID} {
		var exists int
		if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, keyID).Scan(&exists); err != nil || exists != 1 {
			t.Fatalf("preserved key %d exists=%d err=%v", keyID, exists, err)
		}
	}
}

func TestExternalProfileSyncLeavesRawXrayJSONOnExactRawIdentity(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/xray-exact-repair")
	secondRaw := strings.Replace(externalTestXray, `"tag":"Xray"`, `"tag":"Xray renamed"`, 1)
	firstID := insertHistoricalExternalProfile(t, app, sourceID, externalTestXray, "xray-json", "")
	secondID := insertHistoricalExternalProfile(t, app, sourceID, secondRaw, "xray-json", "")

	parsed := parseExternalTestBody(t, externalTestXray+"\n"+"hysteria://unsupported.example:443")
	result, err := app.syncExternalSource(sourceID, parsed)
	if err != nil {
		t.Fatalf("XRAY-JSON partial sync failed: %v", err)
	}
	if result.Counts.Removed != 0 {
		t.Fatalf("XRAY-JSON exact-raw rows were reported removed: result=%#v", result)
	}
	for _, keyID := range []int64{firstID, secondID} {
		var exists int
		if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, keyID).Scan(&exists); err != nil || exists != 1 {
			t.Fatalf("XRAY-JSON exact-raw key %d exists=%d err=%v", keyID, exists, err)
		}
	}
}

func TestExternalProfileSyncRepairsEightyRowsToFortyAndRemainsStable(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/eighty-to-forty")
	otherSourceID := seedExternalProfileSource(t, app, "https://provider.example/eighty-to-forty-other")
	current := make([]string, 0, 40)
	for index := 1; index <= 40; index++ {
		raw := fmt.Sprintf("vless://%08x-1111-4111-8111-%012x@node-%02d.example:443?security=tls&type=tcp#Current-%02d", index, index, index, index)
		variant := strings.Replace(raw, "?security=tls&type=tcp", "?type=tcp&security=tls", 1)
		variant = strings.Replace(variant, fmt.Sprintf("#Current-%02d", index), fmt.Sprintf("#Historical-%02d", index), 1)
		insertHistoricalExternalProfile(t, app, sourceID, raw, "vless", "")
		insertHistoricalExternalProfile(t, app, sourceID, variant, "vless", "")
		current = append(current, raw)
	}
	insertHistoricalExternalProfile(t, app, otherSourceID, current[0], "vless", "")
	if got := sourceProfileCount(t, app, sourceID); got != 80 {
		t.Fatalf("corrupted fixture rows=%d want=80", got)
	}

	for attempt := 1; attempt <= 3; attempt++ {
		result, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, strings.Join(current, "\n")))
		if err != nil {
			t.Fatalf("sync %d failed: %v", attempt, err)
		}
		if got := sourceProfileCount(t, app, sourceID); got != 40 {
			t.Fatalf("sync %d rows=%d want=40 result=%#v", attempt, got, result)
		}
		if attempt == 1 {
			if result.Counts.Removed != 40 || result.Counts.Updated != 40 || result.Counts.Added != 0 {
				t.Fatalf("repair classifications=%#v", result.Counts)
			}
		} else if result.Counts.Removed != 0 || result.Counts.Updated != 0 || result.Counts.Unchanged != 40 {
			t.Fatalf("stable sync %d classifications=%#v", attempt, result.Counts)
		}
	}
	if got := sourceProfileCount(t, app, otherSourceID); got != 1 {
		t.Fatalf("cross-source row count=%d want=1", got)
	}
}

func TestExternalProfileDuplicateWithinSourcePersistsOnce(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/source-duplicate")
	body := externalTestHY2 + "\n" + strings.Replace(externalTestHY2, "#HY2", "#duplicate-tag", 1)
	parsed := parseExternalTestBody(t, body)
	if parsed.Counts.Duplicate != 1 || len(parsed.Keys) != 1 {
		t.Fatalf("parser duplicate result=%#v", parsed)
	}
	result, err := app.syncExternalSource(sourceID, parsed)
	if err != nil || result.Imported != 1 || result.Counts.Duplicate != 1 {
		t.Fatalf("source duplicate sync result=%#v err=%v", result, err)
	}
	var rows int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("source duplicate rows=%d err=%v", rows, err)
	}
}

func TestExternalProfilePersistenceAndSynchronization(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/profiles")
	initialRaw := externalTestSS + "\n" + externalTestHY2
	parsed := parseExternalTestBody(t, initialRaw)
	result, err := app.syncExternalSource(sourceID, parsed)
	if err != nil || result.Imported != 2 {
		t.Fatalf("initial sync result=%#v err=%v", result, err)
	}
	var id int64
	var encURL, protocol, fingerprint, compatibility, warningsJSON, stableRef string
	var schemaVersion int
	if err := app.db.QueryRow(`SELECT k.id, s.encrypted_url, k.protocol, k.profile_fingerprint, k.profile_schema_version, k.profile_compatibility, k.profile_warnings_json, k.external_key_ref FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.external_source_id = ? AND k.protocol = 'shadowsocks'`, sourceID).Scan(&id, &encURL, &protocol, &fingerprint, &schemaVersion, &compatibility, &warningsJSON, &stableRef); err != nil {
		t.Fatalf("read profile row: %v", err)
	}
	sec, err := profilestorage.Decrypt(encURL, app.profileKeyring, id)
	if err != nil {
		t.Fatalf("decrypt profile secret: %v", err)
	}
	raw := sec.Reveal()
	if raw != externalTestSS || protocol != "shadowsocks" || !strings.HasPrefix(fingerprint, "pf1_") || schemaVersion != sources.ExternalProfileSchemaVersion || compatibility != "full" || warningsJSON != "[]" {
		t.Fatalf("unexpected persisted profile: raw_equal=%v protocol=%q fingerprint=%q version=%d compatibility=%q warnings=%q", raw == externalTestSS, protocol, fingerprint, schemaVersion, compatibility, warningsJSON)
	}

	tagChanged := strings.Replace(externalTestSS, "#SS", "#Renamed", 1)
	updated, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, tagChanged+"\n"+externalTestHY2))
	if err != nil || updated.Counts.Updated != 1 || updated.Counts.Unchanged != 1 {
		t.Fatalf("tag refresh result=%#v err=%v", updated, err)
	}
	var updatedID int64
	var updatedEncURL, updatedRef string
	if err := app.db.QueryRow(`SELECT k.id, s.encrypted_url, k.external_key_ref FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.external_source_id = ? AND k.protocol = 'shadowsocks'`, sourceID).Scan(&updatedID, &updatedEncURL, &updatedRef); err != nil {
		t.Fatalf("read updated profile: %v", err)
	}
	secUpdated, _ := profilestorage.Decrypt(updatedEncURL, app.profileKeyring, updatedID)
	updatedRaw := secUpdated.Reveal()
	if updatedID != id || updatedRaw != tagChanged || updatedRef != stableRef {
		t.Fatalf("semantic update changed identity or raw output: id=%d/%d raw_equal=%v ref=%q/%q", id, updatedID, updatedRaw == tagChanged, stableRef, updatedRef)
	}

	partial := tagChanged + "\n" + "tuic://not-a-uuid:password@example.com:443"
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, partial)); err != nil {
		t.Fatalf("partial refresh: %v", err)
	}
	var retained int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&retained); err != nil || retained != 2 {
		t.Fatalf("partial refresh removed valid stored profile: count=%d err=%v", retained, err)
	}
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, tagChanged)); err != nil {
		t.Fatalf("complete missing-key refresh: %v", err)
	}
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&retained); err != nil || retained != 1 {
		t.Fatalf("complete refresh did not remove missing profile: count=%d err=%v", retained, err)
	}

	changedSecret := strings.Replace(tagChanged, "c2hhZG93LXBhc3N3b3Jk", "bmV3LXBhc3N3b3Jk", 1)
	changed, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, changedSecret))
	if err != nil || changed.Imported != 1 {
		t.Fatalf("changed secret refresh result=%#v err=%v", changed, err)
	}
	var changedFingerprint string
	if err := app.db.QueryRow(`SELECT profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&changedFingerprint); err != nil {
		t.Fatalf("read changed fingerprint: %v", err)
	}
	if changedFingerprint == fingerprint {
		t.Fatal("changed credential retained the old semantic fingerprint")
	}
}

func TestExternalProfileSynchronizationIsIdempotentAndSourceScoped(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/idempotent")
	initialBody := strings.Join([]string{externalTestVLESS, externalTestVMess, externalTestTrojan}, "\n")
	initial := parseExternalTestBody(t, initialBody)

	first, err := app.syncExternalSource(sourceID, initial)
	if err != nil || first.Counts.Added != 3 || first.Counts.Updated != 0 || first.Counts.Unchanged != 0 {
		t.Fatalf("first sync result=%#v err=%v", first, err)
	}
	assertCount := func(want int) {
		t.Helper()
		var got int
		if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&got); err != nil || got != want {
			t.Fatalf("source rows=%d want=%d err=%v", got, want, err)
		}
	}
	assertCount(3)

	for attempt := 2; attempt <= 3; attempt++ {
		result, syncErr := app.syncExternalSource(sourceID, parseExternalTestBody(t, initialBody))
		if syncErr != nil || result.Counts.Added != 0 || result.Counts.Updated != 0 || result.Counts.Unchanged != 3 {
			t.Fatalf("sync %d result=%#v err=%v", attempt, result, syncErr)
		}
		assertCount(3)
	}

	reorderedBody := strings.Join([]string{externalTestTrojan, externalTestVLESS, externalTestVMess}, "\n")
	reordered, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, reorderedBody))
	if err != nil || reordered.Counts.Added != 0 || reordered.Counts.Unchanged != 3 {
		t.Fatalf("reordered sync result=%#v err=%v", reordered, err)
	}
	assertCount(3)

	var stableVLESSID int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ? AND protocol = 'vless'`, sourceID).Scan(&stableVLESSID); err != nil {
		t.Fatalf("read stable VLESS id: %v", err)
	}
	renamedVLESS := strings.Replace(externalTestVLESS, "#VLESS", "#Renamed VLESS", 1)
	renamedBody := strings.Join([]string{renamedVLESS, externalTestVMess, externalTestTrojan}, "\n")
	renamed, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, renamedBody))
	if err != nil || renamed.Counts.Added != 0 || renamed.Counts.Updated != 1 || renamed.Counts.Unchanged != 2 {
		t.Fatalf("renamed sync result=%#v err=%v", renamed, err)
	}
	var renamedVLESSID int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ? AND protocol = 'vless'`, sourceID).Scan(&renamedVLESSID); err != nil || renamedVLESSID != stableVLESSID {
		t.Fatalf("renamed VLESS id=%d want=%d err=%v", renamedVLESSID, stableVLESSID, err)
	}

	withNew := renamedBody + "\n" + externalTestSS
	added, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, withNew))
	if err != nil || added.Counts.Added != 1 || added.Counts.Unchanged != 3 {
		t.Fatalf("new-profile sync result=%#v err=%v", added, err)
	}
	assertCount(4)

	partial := parseExternalTestBody(t, renamedVLESS+"\n"+"hysteria://unsupported@example.com:443")
	retained, err := app.syncExternalSource(sourceID, partial)
	if err != nil || retained.Counts.Removed != 0 {
		t.Fatalf("partial missing-profile sync result=%#v err=%v", retained, err)
	}
	assertCount(4)

	removed, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, renamedVLESS))
	if err != nil || removed.Counts.Removed != 3 || removed.Counts.Unchanged != 1 {
		t.Fatalf("complete missing-profile sync result=%#v err=%v", removed, err)
	}
	assertCount(1)

	otherSourceID := seedExternalProfileSource(t, app, "https://provider.example/idempotent-other")
	other, err := app.syncExternalSource(otherSourceID, parseExternalTestBody(t, renamedVLESS))
	if err != nil || other.Counts.Added != 1 {
		t.Fatalf("second-source sync result=%#v err=%v", other, err)
	}
	var crossSourceRows int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE protocol = 'vless' AND external_source_id IN (?, ?)`, sourceID, otherSourceID).Scan(&crossSourceRows); err != nil || crossSourceRows != 2 {
		t.Fatalf("cross-source rows=%d err=%v", crossSourceRows, err)
	}
}

func TestExternalProfileConcurrentRetryDoesNotDuplicateAndHistoryCountsAreAccurate(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/concurrent")
	parsed := parseExternalTestBody(t, strings.Join([]string{externalTestVLESS, externalTestVMess, externalTestTrojan}, "\n"))

	results := make(chan sources.SyncResult, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := app.syncExternalSource(sourceID, parsed)
			results <- result
			errors <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent sync failed: %v", err)
		}
	}
	addedTotal := 0
	unchangedTotal := 0
	var historyResult sources.SyncResult
	for result := range results {
		addedTotal += result.Counts.Added
		unchangedTotal += result.Counts.Unchanged
		if result.Counts.Unchanged == 3 {
			historyResult = result
		}
	}
	if addedTotal != 3 || unchangedTotal != 3 {
		t.Fatalf("concurrent classifications added=%d unchanged=%d", addedTotal, unchangedTotal)
	}
	var rows int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&rows); err != nil || rows != 3 {
		t.Fatalf("concurrent source rows=%d err=%v", rows, err)
	}

	runID := app.startSourceSyncRun(sourceID)
	app.finishSourceSyncRun(runID, historyResult, nil)
	var countsJSON string
	if err := app.db.QueryRow(`SELECT result_counts_json FROM source_sync_runs WHERE id = ?`, runID).Scan(&countsJSON); err != nil {
		t.Fatalf("read sync history: %v", err)
	}
	var counts sources.Counts
	if err := json.Unmarshal([]byte(countsJSON), &counts); err != nil || counts.Added != 0 || counts.Updated != 0 || counts.Unchanged != 3 {
		t.Fatalf("history counts=%#v json=%q err=%v", counts, countsJSON, err)
	}
}

func TestExternalProfileLegacyRowWithoutFingerprintKeepsIDOnRename(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/legacy-fingerprint-backfill")
	userID := seedSubscriptionUser(t, app, "active")
	legacyID := insertAssignedDeliveryKey(t, app, userID, sourceID, "Old name", externalTestVLESS, "vless", "legacy", 1)
	if _, err := app.db.Exec(`UPDATE vless_keys SET external_key_ref = ?, profile_fingerprint = NULL WHERE id = ?`, sources.KeyRef(externalTestVLESS), legacyID); err != nil {
		t.Fatalf("prepare legacy row: %v", err)
	}

	renamed := strings.Replace(externalTestVLESS, "#VLESS", "#New name", 1)
	result, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, renamed))
	if err != nil || result.Counts.Added != 0 || result.Counts.Updated != 1 {
		t.Fatalf("legacy rename result=%#v err=%v", result, err)
	}
	var currentID int64
	var fingerprint string
	if err := app.db.QueryRow(`SELECT id, profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&currentID, &fingerprint); err != nil {
		t.Fatal(err)
	}
	if currentID != legacyID || !strings.HasPrefix(fingerprint, "pf1_") {
		t.Fatalf("legacy identity changed: id=%d want=%d fingerprint=%q", currentID, legacyID, fingerprint)
	}
}

func TestExternalProfileKeyRotationMatchesPreviousFingerprint(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/rotation")
	parsed := parseExternalTestBody(t, externalTestTUIC)
	if _, err := app.syncExternalSource(sourceID, parsed); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	var id int64
	var oldFingerprint string
	if err := app.db.QueryRow(`SELECT id, profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&id, &oldFingerprint); err != nil {
		t.Fatalf("read old fingerprint: %v", err)
	}
	newKey := []byte("abcdef0123456789abcdef0123456789")
	app.profileFingerprintOldKeys = [][]byte{append([]byte(nil), app.profileFingerprintKey...)}
	app.profileFingerprintKey = newKey
	rotated, err := sources.ParseBody(externalTestTUIC, app.externalProfileFingerprintKeys())
	if err != nil {
		t.Fatalf("parse rotated keyring: %v", err)
	}
	result, err := app.syncExternalSource(sourceID, rotated)
	if err != nil || result.Counts.Updated != 1 {
		t.Fatalf("rotation sync result=%#v err=%v", result, err)
	}
	var currentID int64
	var newFingerprint string
	if err := app.db.QueryRow(`SELECT id, profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&currentID, &newFingerprint); err != nil {
		t.Fatalf("read rotated fingerprint: %v", err)
	}
	if currentID != id || newFingerprint == oldFingerprint {
		t.Fatalf("rotation failed to preserve row and replace digest: id=%d/%d old=%q new=%q", id, currentID, oldFingerprint, newFingerprint)
	}
}

func TestExternalProfileCrossSourceAndTUICV4Compatibility(t *testing.T) {
	app := newIntegrationApp(t)
	firstSource := seedExternalProfileSource(t, app, "https://provider.example/first")
	secondSource := seedExternalProfileSource(t, app, "https://provider.example/second")
	parsed := parseExternalTestBody(t, externalTestTUICV4)
	first, err := app.syncExternalSource(firstSource, parsed)
	if err != nil || first.Counts.CompatibilityOnly != 1 {
		t.Fatalf("TUIC v4 first sync result=%#v err=%v", first, err)
	}
	var tuicID int64
	var encURL, fingerprint, compatibility string
	if err := app.db.QueryRow(`SELECT k.id, s.encrypted_url, COALESCE(k.profile_fingerprint, ''), k.profile_compatibility FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.external_source_id = ?`, firstSource).Scan(&tuicID, &encURL, &fingerprint, &compatibility); err != nil {
		t.Fatalf("read TUIC v4: %v", err)
	}
	secTUIC, _ := profilestorage.Decrypt(encURL, app.profileKeyring, tuicID)
	raw := secTUIC.Reveal()
	if raw != externalTestTUICV4 || !strings.HasPrefix(fingerprint, "pf1_") || compatibility != "read_only" {
		t.Fatalf("TUIC v4 persistence raw_equal=%v fingerprint=%q compatibility=%q", raw == externalTestTUICV4, fingerprint, compatibility)
	}
	second, err := app.syncExternalSource(secondSource, parsed)
	if err != nil || second.Counts.CompatibilityOnly != 1 || second.Imported != 1 {
		t.Fatalf("second source did not retain its own exact profile: result=%#v err=%v", second, err)
	}
	rowsQ, err := app.db.Query(`SELECT k.id, s.encrypted_url FROM vless_keys k JOIN vless_key_secrets s ON k.id = s.vless_key_id`)
	if err != nil {
		t.Fatalf("query vless_keys: %v", err)
	}
	defer rowsQ.Close()
	rows := 0
	for rowsQ.Next() {
		var rowID int64
		var rowEnc string
		_ = rowsQ.Scan(&rowID, &rowEnc)
		if dec, err := profilestorage.Decrypt(rowEnc, app.profileKeyring, rowID); err == nil && dec.Reveal() == externalTestTUICV4 {
			rows++
		}
	}
	if rows != 2 {
		t.Fatalf("source-owned exact profiles were merged: rows=%d", rows)
	}
	var firstID, secondID int64
	var firstRef, secondRef string
	if err := app.db.QueryRow(`SELECT id, external_key_ref FROM vless_keys WHERE external_source_id = ?`, firstSource).Scan(&firstID, &firstRef); err != nil {
		t.Fatalf("read first source identity: %v", err)
	}
	if err := app.db.QueryRow(`SELECT id, external_key_ref FROM vless_keys WHERE external_source_id = ?`, secondSource).Scan(&secondID, &secondRef); err != nil {
		t.Fatalf("read second source identity: %v", err)
	}
	if firstID == secondID || firstRef != secondRef {
		t.Fatalf("identity roles collapsed: first=(%d,%q) second=(%d,%q)", firstID, firstRef, secondID, secondRef)
	}
	userID := seedSubscriptionUser(t, app, "active")
	if _, err := app.db.Exec(`UPDATE users SET key_assignment_mode = 'selected' WHERE id = ?`, userID); err != nil {
		t.Fatalf("set selected assignment mode: %v", err)
	}
	if _, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?), (?, ?)`, userID, firstID, userID, secondID); err != nil {
		t.Fatalf("assign both source-owned records: %v", err)
	}
	for sourceID, label := range map[int64]string{firstSource: "first", secondSource: "second"} {
		refreshed, err := app.syncExternalSource(sourceID, parsed)
		if err != nil || refreshed.Counts.Unchanged != 1 {
			t.Fatalf("independent %s refresh result=%#v err=%v", label, refreshed, err)
		}
	}

	// Removing the profile from Source A must delete only Source A's row and
	// assignment. Source B retains its row, metadata, and lifecycle.
	if _, err := app.syncExternalSource(firstSource, parseExternalTestBody(t, externalTestSS)); err != nil {
		t.Fatalf("replace first source independently: %v", err)
	}
	var firstOldRows, secondRows, remainingAssignments int
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ?`, firstID).Scan(&firstOldRows)
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id = ? AND external_source_id = ? AND profile_compatibility = 'read_only'`, secondID, secondSource).Scan(&secondRows)
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = ? AND key_id = ?`, userID, secondID).Scan(&remainingAssignments)
	if firstOldRows != 0 || secondRows != 1 || remainingAssignments != 1 {
		t.Fatalf("source-specific deletion failed: first=%d second=%d assignment=%d", firstOldRows, secondRows, remainingAssignments)
	}

	// A byte-different tag with the same semantic fingerprint also remains
	// independently source-owned.
	tagged := strings.Replace(externalTestTUIC, "#TUIC", "#Other-source", 1)
	if _, err := app.syncExternalSource(firstSource, parseExternalTestBody(t, externalTestTUIC)); err != nil {
		t.Fatalf("replace first source with v5: %v", err)
	}
	thirdSource := seedExternalProfileSource(t, app, "https://provider.example/third")
	third, err := app.syncExternalSource(thirdSource, parseExternalTestBody(t, tagged))
	if err != nil || third.Imported != 1 {
		t.Fatalf("source-scoped semantic identity result=%#v err=%v", third, err)
	}
}

func TestExternalProfileWarningOnlyRefreshPreservesRow(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/warnings")
	alias := "hy2://auth@example.com:443#Alias"
	first, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, alias))
	if err != nil || first.Imported != 1 {
		t.Fatalf("initial alias sync: result=%#v err=%v", first, err)
	}
	var id int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&id); err != nil {
		t.Fatalf("read initial row: %v", err)
	}
	canonicalScheme := "hysteria2://auth@example.com:443#Alias"
	refreshed, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, canonicalScheme))
	if err != nil || refreshed.Counts.Updated != 1 {
		t.Fatalf("warning-only refresh result=%#v err=%v", refreshed, err)
	}
	var currentID int64
	var warningsJSON string
	if err := app.db.QueryRow(`SELECT id, profile_warnings_json FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&currentID, &warningsJSON); err != nil {
		t.Fatalf("read refreshed row: %v", err)
	}
	if currentID != id || warningsJSON != "[]" {
		t.Fatalf("warning-only refresh replaced row or retained warning: id=%d/%d warnings=%q", id, currentID, warningsJSON)
	}
}

func TestExternalProfileSourceSpecificWarningsAndLifecycle(t *testing.T) {
	app := newIntegrationApp(t)
	firstSource := seedExternalProfileSource(t, app, "https://provider.example/warning-source-a")
	secondSource := seedExternalProfileSource(t, app, "https://provider.example/warning-source-b")
	aliasRaw := "hy2://source-auth@example.com:443#Alias"
	canonicalRaw := "hysteria2://source-auth@example.com:443#Canonical"
	if _, err := app.syncExternalSource(firstSource, parseExternalTestBody(t, aliasRaw)); err != nil {
		t.Fatalf("sync alias source: %v", err)
	}
	if _, err := app.syncExternalSource(secondSource, parseExternalTestBody(t, canonicalRaw)); err != nil {
		t.Fatalf("sync canonical source: %v", err)
	}
	var firstID, secondID int64
	var firstWarnings, secondWarnings, secondCreated, secondFingerprint string
	if err := app.db.QueryRow(`SELECT id, profile_warnings_json FROM vless_keys WHERE external_source_id = ?`, firstSource).Scan(&firstID, &firstWarnings); err != nil {
		t.Fatalf("read alias source: %v", err)
	}
	if err := app.db.QueryRow(`SELECT id, profile_warnings_json, created_at, profile_fingerprint FROM vless_keys WHERE external_source_id = ?`, secondSource).Scan(&secondID, &secondWarnings, &secondCreated, &secondFingerprint); err != nil {
		t.Fatalf("read canonical source: %v", err)
	}
	if firstID == secondID || !strings.Contains(firstWarnings, profiles.WarningNonCanonicalScheme) || secondWarnings != "[]" {
		t.Fatalf("source warning state collapsed: first=%d %q second=%d %q", firstID, firstWarnings, secondID, secondWarnings)
	}

	// Source A becomes byte-identical to Source B. It updates only its own row.
	if _, err := app.syncExternalSource(firstSource, parseExternalTestBody(t, canonicalRaw)); err != nil {
		t.Fatalf("canonicalize source A: %v", err)
	}
	var encURL, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter string
	if err := app.db.QueryRow(`SELECT s.encrypted_url, k.profile_warnings_json, k.created_at, k.profile_fingerprint FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id = ? AND k.external_source_id = ?`, secondID, secondSource).Scan(&encURL, &secondWarningsAfter, &secondCreatedAfter, &secondFingerprintAfter); err != nil {
		t.Fatalf("read unaffected source B: %v", err)
	}
	secB, _ := profilestorage.Decrypt(encURL, app.profileKeyring, secondID)
	secondRawAfter := secB.Reveal()
	if secondRawAfter != canonicalRaw || secondWarningsAfter != secondWarnings || secondCreatedAfter != secondCreated || secondFingerprintAfter != secondFingerprint {
		t.Fatalf("Source A refresh mutated Source B: raw=%q warnings=%q created=%q fingerprint=%q", secondRawAfter, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter)
	}
	var identicalRows int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE profile_fingerprint = ?`, secondFingerprint).Scan(&identicalRows); err != nil || identicalRows != 2 {
		t.Fatalf("byte-identical cross-source rows=%d err=%v", identicalRows, err)
	}
}

func TestExternalProfilePreviewApplyIsAuthoritativeAndSecretSafe(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")
	requestBody := fmt.Sprintf(`{"name":"Profiles","category":"general","key_category":"","key_insert_mode":"bottom","source_url":"https://provider.example/manual","enabled":true,"raw_body":%q,"selected_item_refs":["ir1_tampered"]}`, externalTestTUIC)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	recorder := httptest.NewRecorder()
	app.apiV1CreateSource(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "source_selection_invalid") {
		t.Fatalf("tampered apply response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var sources int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources WHERE source_url = 'https://provider.example/manual'`).Scan(&sources); err != nil || sources != 0 {
		t.Fatalf("tampered apply persisted a source: count=%d err=%v", sources, err)
	}
	for _, secret := range []string{"33333333-3333-4333-8333-333333333333", "tuic-password", "tuic://"} {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Fatalf("error response exposed %q: %s", secret, recorder.Body.String())
		}
	}
}

func TestExternalProfilePreviewAndAuditNeverExposeSecrets(t *testing.T) {
	app := newIntegrationApp(t)
	sessionID, _, _ := seedIntegrationSession(t, app, "owner")
	previewPayload, _ := json.Marshal(map[string]any{"source_url": "https://provider.example/preview", "raw_body": externalTestSS + "\n" + externalTestTUIC})
	previewRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sources/preview", strings.NewReader(string(previewPayload)))
	previewRequest.Header.Set("Content-Type", "application/json")
	previewRequest.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	previewRecorder := httptest.NewRecorder()
	app.apiV1PreviewSource(previewRecorder, previewRequest)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview map[string]any
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	items, ok := preview["keys"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("safe preview items = %#v", preview["keys"])
	}

	createPayload, _ := json.Marshal(map[string]any{
		"name": "Safe audit", "category": "general", "key_category": "", "key_insert_mode": "bottom",
		"source_url": "https://provider.example/audit", "enabled": true, "raw_body": externalTestSS,
	})
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(string(createPayload)))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
	createRecorder := httptest.NewRecorder()
	app.apiV1CreateSource(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var auditMetadata string
	if err := app.db.QueryRow(`SELECT metadata_json FROM audit_events WHERE action = 'external_source.create' ORDER BY id DESC LIMIT 1`).Scan(&auditMetadata); err != nil {
		t.Fatalf("read audit event: %v", err)
	}
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
	listRecorder := httptest.NewRecorder()
	app.keys().queries.ListKeys(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK || !strings.Contains(listRecorder.Body.String(), `"protocol":"shadowsocks"`) {
		t.Fatalf("safe key listing status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	combined := previewRecorder.Body.String() + createRecorder.Body.String() + listRecorder.Body.String() + auditMetadata
	for _, secret := range []string{"shadow-password", "plugin-secret", "33333333-3333-4333-8333-333333333333", "tuic-password", "ss://", "tuic://"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("preview/apply/audit exposed %q: %s", secret, combined)
		}
	}
	var unsafeMetadataRows int
	if err := app.db.QueryRow(`
		SELECT
		  (SELECT COUNT(*) FROM vless_keys WHERE profile_warnings_json LIKE '%shadow-password%' OR profile_warnings_json LIKE '%plugin-secret%') +
		  (SELECT COUNT(*) FROM audit_events WHERE metadata_json LIKE '%shadow-password%' OR metadata_json LIKE '%plugin-secret%') +
		  (SELECT COUNT(*) FROM source_sync_runs WHERE result_counts_json LIKE '%shadow-password%' OR error_message LIKE '%shadow-password%')
	`).Scan(&unsafeMetadataRows); err != nil || unsafeMetadataRows != 0 {
		t.Fatalf("raw URI was duplicated into metadata/audit/sync storage: rows=%d err=%v", unsafeMetadataRows, err)
	}
}

func TestExternalProfileCreateRollsBackOnPersistenceFailure(t *testing.T) {
	app := newIntegrationApp(t)
	if _, err := app.db.Exec(`CREATE TRIGGER reject_profile_insert BEFORE INSERT ON vless_keys BEGIN SELECT RAISE(ABORT, 'synthetic profile failure'); END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	payload, _ := json.Marshal(map[string]any{
		"name": "Rollback", "category": "general", "key_category": "", "key_insert_mode": "bottom",
		"source_url": "https://provider.example/rollback-profile", "enabled": true, "raw_body": externalTestHY2,
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sources", strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	app.apiV1CreateSource(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("rollback response status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var sources, keys int
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources WHERE source_url = 'https://provider.example/rollback-profile'`).Scan(&sources)
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE protocol = 'hysteria2'`).Scan(&keys)
	if sources != 0 || keys != 0 {
		t.Fatalf("transactional rollback failed: sources=%d keys=%d", sources, keys)
	}
}

func TestExternalProfileSynchronizationRollsBackOnPersistenceFailure(t *testing.T) {
	app := newIntegrationApp(t)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/sync-rollback")
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, externalTestSS)); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	var originalID int64
	if err := app.db.QueryRow(`SELECT id FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&originalID); err != nil {
		t.Fatalf("read original row: %v", err)
	}
	if _, err := app.db.Exec(`CREATE TRIGGER reject_refresh_update BEFORE UPDATE ON vless_keys BEGIN SELECT RAISE(ABORT, 'synthetic refresh failure'); END`); err != nil {
		t.Fatalf("create refresh trigger: %v", err)
	}
	changed := strings.Replace(externalTestSS, "c2hhZG93LXBhc3N3b3Jk", "bmV3LXBhc3N3b3Jk", 1)
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, changed)); err == nil {
		t.Fatal("refresh persistence failure was ignored")
	}
	var currentID int64
	var encURL string
	if err := app.db.QueryRow(`SELECT k.id, s.encrypted_url FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.external_source_id = ?`, sourceID).Scan(&currentID, &encURL); err != nil {
		t.Fatalf("read retained row: %v", err)
	}
	secRetained, _ := profilestorage.Decrypt(encURL, app.profileKeyring, currentID)
	currentRaw := secRetained.Reveal()
	if currentID != originalID || currentRaw != externalTestSS {
		t.Fatalf("failed refresh partially mutated source: id=%d/%d raw_equal=%v", originalID, currentID, currentRaw == externalTestSS)
	}
}

func TestExternalProfileProbePolicyMarksOnlyUnimplementedProtocolsUnsupported(t *testing.T) {
	for _, raw := range []string{
		"tuic://33333333-3333-4333-8333-333333333333:password@192.0.2.11:443",
		"hysteria://legacy-auth@192.0.2.12:443",
	} {
		status, detail, _ := checkConfigurationAvailability(raw)
		if status != "unsupported_check" || detail != "protocol_health_check_unsupported" {
			t.Fatalf("probe(%q) = %q %q", raw, status, detail)
		}
	}
}

func TestExternalProfileUnsupportedProbeDoesNotAccumulateHealthFailures(t *testing.T) {
	app := newIntegrationApp(t)
	raw := "hysteria://legacy-auth@192.0.2.10:443"
	activeID, activeKey, _ := app.profileKeyring.GetActiveEncryptionKey()
	_, bikKey, _ := app.profileKeyring.GetActiveBlindIndexKey()
	result, err := app.db.Exec(`INSERT INTO vless_keys(label, url_blind_index, protocol, health_failure_count) VALUES('legacy-hysteria', ?, 'hysteria', 2)`, profilestorage.ComputeBlindIndex(bikKey, raw))
	if err != nil {
		t.Fatalf("insert Hysteria profile: %v", err)
	}
	id, _ := result.LastInsertId()
	env, _ := profilestorage.Encrypt([]byte(raw), activeID, activeKey, id)
	if _, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, id, env); err != nil {
		t.Fatalf("insert Hysteria secret: %v", err)
	}
	if err := app.checkAndPersistKey(t.Context(), id, raw); err != nil {
		t.Fatalf("persist unsupported probe: %v", err)
	}
	var status, detail string
	var failures int
	if err := app.db.QueryRow(`SELECT check_status, COALESCE(check_error, ''), health_failure_count FROM vless_keys WHERE id = ?`, id).Scan(&status, &detail, &failures); err != nil {
		t.Fatalf("read probe result: %v", err)
	}
	if status != "unsupported_check" || detail != "protocol_health_check_unsupported" || failures != 2 {
		t.Fatalf("probe persistence = status=%q detail=%q failures=%d", status, detail, failures)
	}
}

func TestExternalProfileErrorsDoNotContainSecrets(t *testing.T) {
	secret := "do-not-log-profile-secret"
	parsed, err := sources.ParseBody("tuic://not-a-uuid:"+secret+"@example.com:70000", [][]byte{externalTestFingerprintKey})
	if err != nil {
		t.Fatalf("partial parser should return an item: %v", err)
	}
	formatted := fmt.Sprintf("%v %#v", parsed.Items, parsed.Items)
	if strings.Contains(formatted, secret) || strings.Contains(parsed.Items[0].ErrorCode, secret) {
		t.Fatalf("secret escaped through error/formatting: %s", formatted)
	}
	if profiles.ErrorCodeOf(fmt.Errorf("wrapped: %w", &profiles.Error{Code: profiles.ErrorInvalidProfile, Protocol: profiles.ProtocolTUIC, Field: "password"})) == "" {
		t.Fatal("profile error wrapping regression")
	}
	internalKey := parseExternalTestBody(t, externalTestTUIC).Keys[0]
	if _, err := json.Marshal(internalKey); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("internal persistence DTO JSON boundary error = %v", err)
	}
}

func TestExternalProfileLoggingStyleFormattingIsSecretSafe(t *testing.T) {
	profile, err := profiles.Parse(externalTestTUIC)
	if err != nil {
		t.Fatalf("parse TUIC: %v", err)
	}
	parsed := parseExternalTestBody(t, externalTestTUIC)
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	log.Printf("profile=%s value=%v detail=%+v go=%#v", profile, profile, profile, profile)
	log.Printf("import_items=%v", parsed.Items)
	log.Printf("internal_parse=%s value=%v detail=%+v go=%#v key=%#v", parsed, parsed, parsed, parsed, parsed.Keys[0])
	logged := output.String()
	for _, secret := range []string{"33333333-3333-4333-8333-333333333333", "tuic-password", "tuic://"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("logging exposed %q: %s", secret, logged)
		}
	}
}
