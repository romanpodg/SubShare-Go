package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"strings"
	"testing"
)

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
