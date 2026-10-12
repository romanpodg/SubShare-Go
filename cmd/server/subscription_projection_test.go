package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSourceOwnedClientDisplayNameOverridesURIAndJSONProjection(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/delivery-client-overrides")
	xray := `{"outbounds":[{"protocol":"trojan","settings":{"servers":[{"address":"json-source.example","port":443,"password":"json-password"}]}}]}`
	items := []struct {
		raw, protocol, name string
	}{
		{externalTestVLESS, "vless", "Override VLESS"},
		{externalTestVMess, "vmess", "Override VMess"},
		{xray, "xray-json", "Override JSON"},
	}
	for index, item := range items {
		keyID := insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, SourceID: &sourceID, Label: "Source panel", Raw: item.raw, Protocol: item.protocol, Compatibility: "full", SortOrder: index})
		if _, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = ? WHERE id = ?`, item.name, keyID); err != nil {
			t.Fatal(err)
		}
	}

	generated, denial, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denial.Code != 0 {
		t.Fatalf("generate source overrides code=%d reason=%q err=%v", denial.Code, denial.Reason, err)
	}
	if strings.Contains(generated.Body, `"outbounds"`) || strings.Contains(generated.Body, `{"`) {
		t.Fatalf("raw JSON leaked from source projection: %q", generated.Body)
	}
	wantNames := map[string]bool{"Override VLESS": false, "Override VMess": false, "Override JSON": false}
	for _, line := range strings.Split(generated.Body, "\n") {
		name := profileconfig.ClientDisplayNameFromKeyURL(line, "")
		if _, ok := wantNames[name]; ok {
			wantNames[name] = true
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Fatalf("source client name %q missing from %q", name, generated.Body)
		}
	}
}

func TestSourceOwnedXrayProjectionUsesHumanLabelInsteadOfRoutingTag(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	sourceID := seedExternalProfileSource(t, app, "https://provider.example/source-xray-name")
	raw := `{"outbounds":[
		{"tag":"proxy","protocol":"vless","settings":{"vnext":[{"address":"one.example","port":443,"users":[{"id":"61111111-1111-4111-8111-111111111111","encryption":"none"}]}]}},
		{"tag":"proxy","protocol":"trojan","settings":{"servers":[{"address":"two.example","port":443,"password":"secret"}]}}
	]}`
	keyID := insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, SourceID: &sourceID, Label: "🌟 Human source name", Raw: raw, Protocol: "xray-json", Compatibility: "full", SortOrder: 0})
	var envelopeBefore string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeBefore); err != nil {
		t.Fatal(err)
	}

	generated, denial, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denial.Code != 0 {
		t.Fatalf("source XRAY projection code=%d reason=%q err=%v", denial.Code, denial.Reason, err)
	}
	lines := strings.Split(generated.Body, "\n")
	if len(lines) != 2 {
		t.Fatalf("projected links=%d body=%q", len(lines), generated.Body)
	}
	for index, line := range lines {
		want := "🌟 Human source name"
		if index > 0 {
			want = fmt.Sprintf("🌟 Human source name (%d)", index+1)
		}
		if got := profileconfig.ClientDisplayNameFromKeyURL(line, ""); got != want {
			t.Fatalf("projected name %d=%q want=%q", index, got, want)
		}
	}
	var storedName sql.NullString
	var envelopeAfter string
	if err := app.db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = ?`, keyID).Scan(&storedName); err != nil || storedName.Valid {
		t.Fatalf("derived source name was persisted: %#v err=%v", storedName, err)
	}
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeAfter); err != nil || envelopeAfter != envelopeBefore {
		t.Fatalf("projection rewrote encrypted source config=%v err=%v", envelopeAfter != envelopeBefore, err)
	}
}

func TestLinkModeProjectsRawXrayJSONAtResolverBoundaryWithoutMutatingProfiles(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	xrayOnly := `{"remarks":"JSON only","outbounds":[{"tag":"json-only","protocol":"vless","settings":{"vnext":[{"address":"json-only.example","port":443,"users":[{"id":"44444444-4444-4444-8444-444444444444"}]}]}}]}`
	items := []struct {
		raw, protocol string
	}{
		{externalTestVLESS, "vless"},
		{externalTestVMess, "vmess"},
		{externalTestTrojan, "trojan"},
		{deliverySSClean, "shadowsocks"},
		{deliveryHY2, "hysteria2"},
		{deliveryTUICSingBox, "tuic"},
		{xrayOnly, "xray-json"},
	}
	for index, item := range items {
		insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: item.protocol, Raw: item.raw, Protocol: item.protocol, Compatibility: "full", SortOrder: index})
	}
	infoResult, err := app.db.Exec(`INSERT INTO vless_keys(label, status, key_kind, template_text, sort_order) VALUES('Info', 'active', 'informational', 'Hello {user_name}', 100)`)
	if err != nil {
		t.Fatalf("insert informational key: %v", err)
	}
	infoID, _ := infoResult.LastInsertId()
	if _, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, infoID); err != nil {
		t.Fatalf("assign informational key: %v", err)
	}

	generated, denial, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denial.Code != 0 {
		t.Fatalf("link generation code=%d reason=%q err=%v", denial.Code, denial.Reason, err)
	}
	if json.Valid([]byte(generated.Body)) || strings.Contains(generated.Body, `"outbounds"`) || strings.Contains(generated.Body, `{"`) {
		t.Fatalf("raw XRAY-JSON leaked into link body: %q", generated.Body)
	}
	if !strings.Contains(generated.Body, "vless://44444444-4444-4444-8444-444444444444@json-only.example:443") {
		t.Fatalf("projected XRAY-JSON VLESS missing from link body: %q", generated.Body)
	}
	lines := strings.Split(generated.Body, "\n")
	if len(lines) != 8 {
		t.Fatalf("link lines=%d body=%q exclusions=%#v", len(lines), generated.Body, generated.Exclusions)
	}
	for _, line := range lines {
		scheme := profileconfig.SupportedConfigScheme(line)
		if scheme == model.SubscriptionFormatXrayJSON || scheme == "" {
			t.Fatalf("non-link entry reached link output: %q", line)
		}
	}
	if len(generated.Exclusions) != 0 {
		t.Fatalf("XRAY-JSON exclusion=%#v", generated.Exclusions)
	}

	var rowsBefore int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys`).Scan(&rowsBefore); err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.generateSelectedSubscription("subscription-token", "xray-json"); err != nil {
		t.Fatalf("JSON mode generation: %v", err)
	}
	var rowsAfter int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys`).Scan(&rowsAfter); err != nil || rowsAfter != rowsBefore {
		t.Fatalf("format switching mutated profiles: before=%d after=%d err=%v", rowsBefore, rowsAfter, err)
	}
}

func TestLinkModeProjectsMultipleXrayOutboundsWithClientNameAndKeepsStoredBytes(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	raw := `{
		"dns":{"servers":["1.1.1.1"]},
		"routing":{"rules":[{"outboundTag":"direct"}]},
		"outbounds":[
			{"tag":"raw-vless","protocol":"vless","settings":{"vnext":[{"address":"project-vless.example","port":443,"users":[{"id":"51111111-1111-4111-8111-111111111111","encryption":"none"}]}]},"streamSettings":{"network":"grpc","security":"reality","grpcSettings":{"serviceName":"edge"},"realitySettings":{"serverName":"sni.example","publicKey":"public-key","shortId":"abcd","fingerprint":"chrome"}}},
			{"tag":"raw-vmess","protocol":"vmess","settings":{"vnext":[{"address":"project-vmess.example","port":8443,"users":[{"id":"52222222-2222-4222-8222-222222222222","security":"auto"}]}]}},
			{"tag":"raw-trojan","protocol":"trojan","settings":{"servers":[{"address":"project-trojan.example","port":443,"password":"trojan-password"}]}},
			{"tag":"raw-ss","protocol":"shadowsocks","settings":{"address":"project-ss.example","port":8388,"method":"aes-256-gcm","password":"ss-password"}},
			{"tag":"raw-hy2","protocol":"hysteria","settings":{"version":2,"address":"project-hy.example","port":443},"streamSettings":{"security":"tls","hysteriaSettings":{"version":2,"auth":"hy-auth"},"tlsSettings":{"serverName":"hy-sni.example"}}},
			{"tag":"raw-tuic","protocol":"tuic","settings":{"address":"project-tuic.example","port":443,"uuid":"53333333-3333-4333-8333-333333333333","password":"tuic-password","sni":"tuic-sni.example","alpn":["h3"]}},
			{"tag":"direct","protocol":"freedom","settings":{}},
			{"tag":"block","protocol":"blackhole","settings":{}},
			{"tag":"broken","protocol":"trojan","settings":{"servers":[]}}
		]
	}`
	keyID := insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: "Panel only", Raw: raw, Protocol: "xray-json", Compatibility: "full", SortOrder: 0})
	if _, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = 'Subscriber node' WHERE id = ?`, keyID); err != nil {
		t.Fatal(err)
	}
	var envelopeBefore string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeBefore); err != nil {
		t.Fatal(err)
	}

	generated, denial, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denial.Code != 0 {
		t.Fatalf("link projection code=%d reason=%q err=%v", denial.Code, denial.Reason, err)
	}
	if strings.Contains(generated.Body, `"dns"`) || strings.Contains(generated.Body, `"routing"`) || strings.Contains(generated.Body, `"outbounds"`) || strings.Contains(generated.Body, "freedom") || strings.Contains(generated.Body, "blackhole") {
		t.Fatalf("non-link JSON content leaked: %q", generated.Body)
	}
	lines := strings.Split(generated.Body, "\n")
	if len(lines) != 6 || generated.GeneratedCount != 6 || len(generated.Exclusions) != 1 {
		t.Fatalf("lines=%d generated=%d exclusions=%#v body=%q", len(lines), generated.GeneratedCount, generated.Exclusions, generated.Body)
	}
	for index, line := range lines {
		wantName := "Subscriber node"
		if index > 0 {
			wantName = fmt.Sprintf("Subscriber node (%d)", index+1)
		}
		if got := profileconfig.ClientDisplayNameFromKeyURL(line, ""); got != wantName {
			t.Fatalf("line %d client name=%q want=%q link=%q", index, got, wantName, line)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token/subbody", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	recorder := httptest.NewRecorder()
	app.handleSubscriptionSubBody(recorder, request)
	decoded, decodeErr := base64.StdEncoding.DecodeString(recorder.Body.String())
	if recorder.Code != http.StatusOK || decodeErr != nil || string(decoded) != generated.Body {
		t.Fatalf("base64 link projection status=%d decode=%v body=%q", recorder.Code, decodeErr, string(decoded))
	}

	jsonGenerated, denial, err := app.generateSelectedSubscription("subscription-token", "xray-json")
	if err != nil || denial.Code != 0 || !strings.Contains(jsonGenerated.Body, `"dns"`) || !strings.Contains(jsonGenerated.Body, `"routing"`) {
		t.Fatalf("JSON mode lost original document: code=%d err=%v body=%q", denial.Code, err, jsonGenerated.Body)
	}
	var envelopeAfter string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeAfter); err != nil {
		t.Fatal(err)
	}
	if envelopeAfter != envelopeBefore {
		t.Fatal("switching output formats rewrote encrypted profile state")
	}
}
