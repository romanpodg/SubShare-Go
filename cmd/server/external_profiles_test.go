package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/profiles"
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

func parseExternalTestBody(t *testing.T, body string) externalSubscriptionParseResult {
	t.Helper()
	parsed, err := parseExternalSubscriptionBody(body, [][]byte{externalTestFingerprintKey})
	if err != nil {
		t.Fatalf("parseExternalSubscriptionBody: %v", err)
	}
	return parsed
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
	if !containsWarningCode(parsed.Warnings, "partial_import") {
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
	if len(parsed.Keys) != 3 || !containsWarningCode(parsed.Warnings, "base64_decoded") {
		t.Fatalf("unexpected base64 parse: keys=%d warnings=%#v", len(parsed.Keys), parsed.Warnings)
	}
	if parsed.Items[2].Status != externalStatusCompatibilityOnly || parsed.Items[2].Compatibility != "read_only" {
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
	if containsWarningCode(plain.Warnings, "base64_decoded") || len(plain.Keys) != 1 {
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
			if len(parsed.Keys) != 2 || !containsWarningCode(parsed.Warnings, "base64_decoded") {
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
			_, err := parseExternalSubscriptionBody(encoded, [][]byte{externalTestFingerprintKey})
			if err == nil || err.Error() != "invalid_base64_subscription" || strings.Contains(err.Error(), "provider-secret") {
				t.Fatalf("malformed Base64 error=%v", err)
			}
		})
	}
	if _, err := parseExternalSubscriptionBody(strings.Repeat("A", maxExternalSubscriptionBodyBytes+1), [][]byte{externalTestFingerprintKey}); err == nil || err.Error() != "subscription_body_too_large" {
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
		if _, err := parseExternalSubscriptionFromRawBody("https://provider.example/sip008", body, externalSubscriptionMetadata{}, [][]byte{externalTestFingerprintKey}); err == nil || err.Error() != "unsupported_sip008_format" {
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
		`{"outbounds":[{"protocol":"freedom","settings":{}}]}`,
		`{"outbounds":[]}`,
		`{"valid":"json but not xray"}`,
	} {
		parsed := parseExternalTestBody(t, raw)
		if len(parsed.Keys) != 0 || parsed.Counts.Rejected != 1 || parsed.Items[0].ErrorCode != "invalid_xray_json" {
			t.Fatalf("unsupported Xray-shaped JSON result=%#v", parsed)
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
	if contradictory.Counts.Ambiguous != 1 || !containsWarningCode(contradictory.Items[0].Warnings, profiles.WarningAmbiguousParameter) {
		t.Fatalf("contradictory TUIC aliases were not marked ambiguous: %#v", contradictory)
	}
}

func TestExternalProfileFingerprintKeyAndSelectionIntegrity(t *testing.T) {
	if _, err := parseExternalSubscriptionBody(externalTestSS, nil); err == nil || err.Error() != "profile_fingerprint_key_unavailable" {
		t.Fatalf("nil fingerprint key error = %v", err)
	}
	parsed := parseExternalTestBody(t, externalTestSS+"\n"+externalTestHY2)
	selected, err := filterExternalSelection(parsed, []string{parsed.Items[1].ItemRef})
	if err != nil || len(selected.Keys) != 1 || selected.Keys[0].Protocol != "hysteria2" {
		t.Fatalf("selection result=%#v err=%v", selected, err)
	}
	for _, refs := range [][]string{{"ir1_tampered"}, {parsed.Items[0].ItemRef, parsed.Items[0].ItemRef}} {
		if _, err := filterExternalSelection(parsed, refs); err == nil {
			t.Fatalf("tampered selection %#v was accepted", refs)
		}
	}

	identical := parseExternalTestBody(t, externalTestSS+"\n"+externalTestSS)
	if len(identical.Keys) != 1 || len(identical.Items) != 2 || identical.Items[1].Status != externalStatusDuplicate {
		t.Fatalf("identical-line duplicate result = %#v", identical)
	}
	if identical.Items[0].ItemRef == identical.Items[1].ItemRef {
		t.Fatalf("identical source lines share item reference %q", identical.Items[0].ItemRef)
	}
	if _, err := filterExternalSelection(identical, []string{identical.Items[1].ItemRef}); err == nil {
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
	var raw, protocol, fingerprint, compatibility, warningsJSON, stableRef string
	var schemaVersion int
	if err := app.db.QueryRow(`SELECT id, url, protocol, profile_fingerprint, profile_schema_version, profile_compatibility, profile_warnings_json, external_key_ref FROM vless_keys WHERE external_source_id = ? AND protocol = 'shadowsocks'`, sourceID).Scan(&id, &raw, &protocol, &fingerprint, &schemaVersion, &compatibility, &warningsJSON, &stableRef); err != nil {
		t.Fatalf("read profile row: %v", err)
	}
	if raw != externalTestSS || protocol != "shadowsocks" || !strings.HasPrefix(fingerprint, "pf1_") || schemaVersion != externalProfileSchemaVersion || compatibility != "full" || warningsJSON != "[]" {
		t.Fatalf("unexpected persisted profile: raw_equal=%v protocol=%q fingerprint=%q version=%d compatibility=%q warnings=%q", raw == externalTestSS, protocol, fingerprint, schemaVersion, compatibility, warningsJSON)
	}

	tagChanged := strings.Replace(externalTestSS, "#SS", "#Renamed", 1)
	updated, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, tagChanged+"\n"+externalTestHY2))
	if err != nil || updated.Counts.Updated != 1 || updated.Counts.Unchanged != 1 {
		t.Fatalf("tag refresh result=%#v err=%v", updated, err)
	}
	var updatedID int64
	var updatedRaw, updatedRef string
	if err := app.db.QueryRow(`SELECT id, url, external_key_ref FROM vless_keys WHERE external_source_id = ? AND protocol = 'shadowsocks'`, sourceID).Scan(&updatedID, &updatedRaw, &updatedRef); err != nil {
		t.Fatalf("read updated profile: %v", err)
	}
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
	rotated, err := parseExternalSubscriptionBody(externalTestTUIC, app.externalProfileFingerprintKeys())
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
	var raw, fingerprint, compatibility string
	if err := app.db.QueryRow(`SELECT url, COALESCE(profile_fingerprint, ''), profile_compatibility FROM vless_keys WHERE external_source_id = ?`, firstSource).Scan(&raw, &fingerprint, &compatibility); err != nil {
		t.Fatalf("read TUIC v4: %v", err)
	}
	if raw != externalTestTUICV4 || !strings.HasPrefix(fingerprint, "pf1_") || compatibility != "read_only" {
		t.Fatalf("TUIC v4 persistence raw_equal=%v fingerprint=%q compatibility=%q", raw == externalTestTUICV4, fingerprint, compatibility)
	}
	second, err := app.syncExternalSource(secondSource, parsed)
	if err != nil || second.Counts.CompatibilityOnly != 1 || second.Imported != 1 {
		t.Fatalf("second source did not retain its own exact profile: result=%#v err=%v", second, err)
	}
	var rows int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE url = ?`, externalTestTUICV4).Scan(&rows); err != nil || rows != 2 {
		t.Fatalf("source-owned exact profiles were merged: rows=%d err=%v", rows, err)
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
	var secondRawAfter, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter string
	if err := app.db.QueryRow(`SELECT url, profile_warnings_json, created_at, profile_fingerprint FROM vless_keys WHERE id = ? AND external_source_id = ?`, secondID, secondSource).Scan(&secondRawAfter, &secondWarningsAfter, &secondCreatedAfter, &secondFingerprintAfter); err != nil {
		t.Fatalf("read unaffected source B: %v", err)
	}
	if secondRawAfter != canonicalRaw || secondWarningsAfter != secondWarnings || secondCreatedAfter != secondCreated || secondFingerprintAfter != secondFingerprint {
		t.Fatalf("Source A refresh mutated Source B: raw=%q warnings=%q created=%q fingerprint=%q", secondRawAfter, secondWarningsAfter, secondCreatedAfter, secondFingerprintAfter)
	}
	var identicalRows int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE url = ? AND profile_fingerprint = ?`, canonicalRaw, secondFingerprint).Scan(&identicalRows); err != nil || identicalRows != 2 {
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
	app.apiV1ListKeys(listRecorder, listRequest)
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
	if _, err := app.db.Exec(`CREATE TRIGGER reject_refresh_insert BEFORE INSERT ON vless_keys BEGIN SELECT RAISE(ABORT, 'synthetic refresh failure'); END`); err != nil {
		t.Fatalf("create refresh trigger: %v", err)
	}
	changed := strings.Replace(externalTestSS, "c2hhZG93LXBhc3N3b3Jk", "bmV3LXBhc3N3b3Jk", 1)
	if _, err := app.syncExternalSource(sourceID, parseExternalTestBody(t, changed)); err == nil {
		t.Fatal("refresh persistence failure was ignored")
	}
	var currentID int64
	var currentRaw string
	if err := app.db.QueryRow(`SELECT id, url FROM vless_keys WHERE external_source_id = ?`, sourceID).Scan(&currentID, &currentRaw); err != nil {
		t.Fatalf("read retained row: %v", err)
	}
	if currentID != originalID || currentRaw != externalTestSS {
		t.Fatalf("failed refresh partially mutated source: id=%d/%d raw_equal=%v", originalID, currentID, currentRaw == externalTestSS)
	}
}

func TestExternalProfileProbePolicyDoesNotClaimQUICAuthentication(t *testing.T) {
	for _, raw := range []string{
		"hy2://auth@192.0.2.10:443",
		"tuic://33333333-3333-4333-8333-333333333333:password@192.0.2.11:443",
	} {
		status, detail, _ := checkConfigurationAvailability(raw)
		if status != "unknown" || detail != "dns_resolved_udp_quic_probe_unsupported" {
			t.Fatalf("probe(%q) = %q %q", raw, status, detail)
		}
	}
}

func TestExternalProfileUnsupportedProbeDoesNotAccumulateHealthFailures(t *testing.T) {
	app := newIntegrationApp(t)
	raw := "hy2://auth@192.0.2.10:443"
	result, err := app.db.Exec(`INSERT INTO vless_keys(label, url, protocol, health_failure_count) VALUES('hy2', ?, 'hysteria2', 2)`, raw)
	if err != nil {
		t.Fatalf("insert Hysteria profile: %v", err)
	}
	id, _ := result.LastInsertId()
	if err := app.checkAndPersistKey(id, raw); err != nil {
		t.Fatalf("persist unsupported probe: %v", err)
	}
	var status, detail string
	var failures int
	if err := app.db.QueryRow(`SELECT check_status, COALESCE(check_error, ''), health_failure_count FROM vless_keys WHERE id = ?`, id).Scan(&status, &detail, &failures); err != nil {
		t.Fatalf("read probe result: %v", err)
	}
	if status != "unknown" || detail != "dns_resolved_udp_quic_probe_unsupported" || failures != 2 {
		t.Fatalf("probe persistence = status=%q detail=%q failures=%d", status, detail, failures)
	}
}

func TestExternalProfileErrorsDoNotContainSecrets(t *testing.T) {
	secret := "do-not-log-profile-secret"
	parsed, err := parseExternalSubscriptionBody("tuic://not-a-uuid:"+secret+"@example.com:70000", [][]byte{externalTestFingerprintKey})
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
