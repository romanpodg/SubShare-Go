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
		{
			_, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = ? WHERE id = ?`, item.name, keyID)
			requireFixture(t, err == nil, err)
		}

	}

	generated, denial, err := app.generateSelectedSubscription("subscription-token", "plain")
	requireFixturef(t, err == nil, "generate source overrides code=%d reason=%q err=%v", denial.Code, denial.Reason, err)
	requireFixturef(t, denial.Code == 0, "generate source overrides code=%d reason=%q err=%v", denial.Code, denial.Reason, err)

	requireFixturef(t, !(strings.Contains(generated.Body, `"outbounds"`)), "raw JSON leaked from source projection: %q", generated.Body)
	requireFixturef(t, !(strings.Contains(generated.Body, `{"`)), "raw JSON leaked from source projection: %q", generated.Body)

	wantNames := map[string]bool{"Override VLESS": false, "Override VMess": false, "Override JSON": false}
	for _, line := range strings.Split(generated.Body, "\n") {
		name := profileconfig.ClientDisplayNameFromKeyURL(line, "")
		if _, ok := wantNames[name]; ok {
			wantNames[name] = true
		}
	}
	for name, found := range wantNames {
		requireFixturef(t, found, "source client name %q missing from %q", name, generated.Body)

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
	{
		err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeBefore)
		requireFixture(t, err == nil, err)
	}

	generated, denial, err := app.generateSelectedSubscription("subscription-token", "plain")
	requireFixturef(t, err == nil, "source XRAY projection code=%d reason=%q err=%v", denial.Code, denial.Reason, err)
	requireFixturef(t, denial.Code == 0, "source XRAY projection code=%d reason=%q err=%v", denial.Code, denial.Reason, err)

	lines := strings.Split(generated.Body, "\n")
	requireFixturef(t, len(lines) == 2, "projected links=%d body=%q", len(lines), generated.Body)

	for index, line := range lines {
		want := "🌟 Human source name"
		if index > 0 {
			want = fmt.Sprintf("🌟 Human source name (%d)", index+1)
		}
		{
			got := profileconfig.ClientDisplayNameFromKeyURL(line, "")
			requireFixturef(t, got == want, "projected name %d=%q want=%q", index, got, want)
		}

	}
	var storedName sql.NullString
	var envelopeAfter string
	{
		err := app.db.QueryRow(`SELECT client_display_name FROM vless_keys WHERE id = ?`, keyID).Scan(&storedName)
		requireFixturef(t, err == nil, "derived source name was persisted: %#v err=%v", storedName, err)
		requireFixturef(t, !(storedName.Valid), "derived source name was persisted: %#v err=%v", storedName, err)
	}

	{
		err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeAfter)
		requireFixturef(t, err == nil, "projection rewrote encrypted source config=%v err=%v", envelopeAfter != envelopeBefore, err)
		requireFixturef(t, envelopeAfter == envelopeBefore, "projection rewrote encrypted source config=%v err=%v", envelopeAfter != envelopeBefore, err)
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
	requireFixturef(t, err == nil, "insert informational key: %v", err)

	infoID, _ := infoResult.LastInsertId()
	{
		_, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, infoID)
		requireFixturef(t, err == nil, "assign informational key: %v", err)
	}

	generated, denial, err := app.generateSelectedSubscription("subscription-token", "plain")
	requireFixturef(t, err == nil, "link generation code=%d reason=%q err=%v", denial.Code, denial.Reason, err)
	requireFixturef(t, denial.Code == 0, "link generation code=%d reason=%q err=%v", denial.Code, denial.Reason, err)

	requireFixturef(t, !(json.Valid([]byte(generated.Body))), "raw XRAY-JSON leaked into link body: %q", generated.Body)
	requireFixturef(t, !(strings.Contains(generated.Body, `"outbounds"`)), "raw XRAY-JSON leaked into link body: %q", generated.Body)
	requireFixturef(t, !(strings.Contains(generated.Body, `{"`)), "raw XRAY-JSON leaked into link body: %q", generated.Body)

	requireFixturef(t, strings.Contains(generated.Body, "vless://44444444-4444-4444-8444-444444444444@json-only.example:443"), "projected XRAY-JSON VLESS missing from link body: %q", generated.Body)

	lines := strings.Split(generated.Body, "\n")
	requireFixturef(t, len(lines) == 8, "link lines=%d body=%q exclusions=%#v", len(lines), generated.Body, generated.Exclusions)

	for _, line := range lines {
		scheme := profileconfig.SupportedConfigScheme(line)
		requireFixturef(t, scheme != model.SubscriptionFormatXrayJSON, "non-link entry reached link output: %q", line)
		requireFixturef(t, scheme != "", "non-link entry reached link output: %q", line)

	}
	requireFixturef(t, len(generated.Exclusions) == 0, "XRAY-JSON exclusion=%#v", generated.Exclusions)

	var rowsBefore int
	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys`).Scan(&rowsBefore)
		requireFixture(t, err == nil, err)
	}

	{
		_, _, err := app.generateSelectedSubscription("subscription-token", "xray-json")
		requireFixturef(t, err == nil, "JSON mode generation: %v", err)
	}

	var rowsAfter int
	{
		err := app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys`).Scan(&rowsAfter)
		requireFixturef(t, err == nil, "format switching mutated profiles: before=%d after=%d err=%v", rowsBefore, rowsAfter, err)
		requireFixturef(t, rowsAfter == rowsBefore, "format switching mutated profiles: before=%d after=%d err=%v", rowsBefore, rowsAfter, err)
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
	{
		_, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = 'Subscriber node' WHERE id = ?`, keyID)
		requireFixture(t, err == nil, err)
	}

	var envelopeBefore string
	{
		err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeBefore)
		requireFixture(t, err == nil, err)
	}

	generated, denial, err := app.generateSelectedSubscription("subscription-token", "plain")
	requireFixturef(t, err == nil, "link projection code=%d reason=%q err=%v", denial.Code, denial.Reason, err)
	requireFixturef(t, denial.Code == 0, "link projection code=%d reason=%q err=%v", denial.Code, denial.Reason, err)

	requireFixturef(t, !(strings.Contains(generated.Body, `"dns"`)), "non-link JSON content leaked: %q", generated.Body)
	requireFixturef(t, !(strings.Contains(generated.Body, `"routing"`)), "non-link JSON content leaked: %q", generated.Body)
	requireFixturef(t, !(strings.Contains(generated.Body, `"outbounds"`)), "non-link JSON content leaked: %q", generated.Body)
	requireFixturef(t, !(strings.Contains(generated.Body, "freedom")), "non-link JSON content leaked: %q", generated.Body)
	requireFixturef(t, !(strings.Contains(generated.Body, "blackhole")), "non-link JSON content leaked: %q", generated.Body)

	lines := strings.Split(generated.Body, "\n")
	requireFixturef(t, len(lines) == 6, "lines=%d generated=%d exclusions=%#v body=%q", len(lines), generated.GeneratedCount, generated.Exclusions, generated.Body)
	requireFixturef(t, generated.GeneratedCount == 6, "lines=%d generated=%d exclusions=%#v body=%q", len(lines), generated.GeneratedCount, generated.Exclusions, generated.Body)
	requireFixturef(t, len(generated.Exclusions) == 1, "lines=%d generated=%d exclusions=%#v body=%q", len(lines), generated.GeneratedCount, generated.Exclusions, generated.Body)

	assertProjectedClientNames(t, lines, "Subscriber node")

	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token/subbody", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	recorder := httptest.NewRecorder()
	app.handleSubscriptionSubBody(recorder, request)
	decoded, decodeErr := base64.StdEncoding.DecodeString(recorder.Body.String())
	requireFixturef(t, recorder.Code == http.StatusOK, "base64 link projection status=%d decode=%v body=%q", recorder.Code, decodeErr, string(decoded))
	requireFixturef(t, decodeErr == nil, "base64 link projection status=%d decode=%v body=%q", recorder.Code, decodeErr, string(decoded))
	requireFixturef(t, string(decoded) == generated.Body, "base64 link projection status=%d decode=%v body=%q", recorder.Code, decodeErr, string(decoded))

	jsonGenerated, denial, err := app.generateSelectedSubscription("subscription-token", "xray-json")
	requireFixturef(t, err == nil, "JSON mode lost original document: code=%d err=%v body=%q", denial.Code, err, jsonGenerated.Body)
	requireFixturef(t, denial.Code == 0, "JSON mode lost original document: code=%d err=%v body=%q", denial.Code, err, jsonGenerated.Body)
	requireFixturef(t, strings.Contains(jsonGenerated.Body, `"dns"`), "JSON mode lost original document: code=%d err=%v body=%q", denial.Code, err, jsonGenerated.Body)
	requireFixturef(t, strings.Contains(jsonGenerated.Body, `"routing"`), "JSON mode lost original document: code=%d err=%v body=%q", denial.Code, err, jsonGenerated.Body)

	var envelopeAfter string
	{
		err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeAfter)
		requireFixture(t, err == nil, err)
	}

	requireFixture(t, envelopeAfter == envelopeBefore, "switching output formats rewrote encrypted profile state")

}

func assertProjectedClientNames(t *testing.T, lines []string, baseName string) {
	t.Helper()
	for index, line := range lines {
		wantName := baseName
		if index > 0 {
			wantName = fmt.Sprintf("%s (%d)", baseName, index+1)
		}
		{
			got := profileconfig.ClientDisplayNameFromKeyURL(line, "")
			requireFixturef(t, got == wantName, "line %d client name=%q want=%q link=%q", index, got, wantName, line)
		}

	}
}
