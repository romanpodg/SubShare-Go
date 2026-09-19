package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"gopkg.in/yaml.v3"
)

const (
	deliverySSClean     = "ss://YWVzLTI1Ni1nY206c2hhZG93LXBhc3N3b3Jk@ss.example:8388#SS"
	deliverySSPlugin    = "ss://YWVzLTI1Ni1nY206c2hhZG93LXBhc3N3b3Jk@ss.example:8388?plugin=obfs-local%3Bobfs%3Dtls%3Bobfs-host%3Dcdn.example#SS-plugin"
	deliverySSXrayAlias = "ss://Y2hhY2hhMjAtcG9seTEzMDU6cGFzc3dvcmQ=@ss-alias.example:8388#SS-alias"
	deliveryHY2         = "hysteria2://hysteria-auth@hy.example:443,5000-5010?sni=tls.example&pinSHA256=abababababababababababababababababababababababababababababababab&obfs=salamander&obfs-password=obfs-secret#HY2"
	deliveryTUICMihomo  = "tuic://33333333-3333-4333-8333-333333333333:tuic-password@tuic.example:443?sni=tls.example&alpn=h3%2Chq-29&skip-cert-verify=true&congestion-controller=bbr&udp-relay-mode=quic&reduce-rtt=true&heartbeat-interval=10s&request-timeout=8s&fast-open=true&max-open-streams=20&max-udp-relay-packet-size=1500#TUIC"
	deliveryTUICSingBox = "tuic://33333333-3333-4333-8333-333333333333:tuic-password@tuic.example:443?server_name=tls.example&alpn=h3%2Chq-29&allow_insecure=true&congestion_control=bbr&udp_over_stream=true&zero_rtt_handshake=true&heartbeat=10s#TUIC"
)

func TestShareURIWithDisplayNameCoversAllSupportedProtocols(t *testing.T) {
	items := []string{externalTestVLESS, externalTestVMess, externalTestTrojan, deliverySSClean, deliveryHY2, deliveryTUICSingBox}
	for _, raw := range items {
		link, err := shareURIWithDisplayName(raw, "Subscriber name")
		if err != nil {
			t.Fatalf("rename %s: %v", profileconfig.SupportedConfigScheme(raw), err)
		}
		if got := profileconfig.ClientDisplayNameFromKeyURL(link, ""); got != "Subscriber name" {
			t.Fatalf("%s client name=%q link=%q", profileconfig.SupportedConfigScheme(raw), got, link)
		}
	}
}

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
		keyID := insertAssignedDeliveryKey(t, app, userID, sourceID, "Source panel", item.raw, item.protocol, "full", index)
		if _, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = ? WHERE id = ?`, item.name, keyID); err != nil {
			t.Fatal(err)
		}
	}

	generated, _, denyCode, denyReason, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denyCode != 0 {
		t.Fatalf("generate source overrides code=%d reason=%q err=%v", denyCode, denyReason, err)
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
	keyID := insertAssignedDeliveryKey(t, app, userID, sourceID, "🌟 Human source name", raw, "xray-json", "full", 0)
	var envelopeBefore string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeBefore); err != nil {
		t.Fatal(err)
	}

	generated, _, denyCode, denyReason, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denyCode != 0 {
		t.Fatalf("source XRAY projection code=%d reason=%q err=%v", denyCode, denyReason, err)
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

func insertAssignedDeliveryKey(t *testing.T, app *App, userID int64, sourceID any, label, raw, protocol, compatibility string, sortOrder int) int64 {
	t.Helper()
	activeID, activeKey, _ := app.profileKeyring.GetActiveEncryptionKey()
	_, bikKey, _ := app.profileKeyring.GetActiveBlindIndexKey()
	blindIndex := profilestorage.ComputeBlindIndex(bikKey, raw)

	result, err := app.db.Exec(`
		INSERT INTO vless_keys(
			label, url_blind_index, status, key_kind, health_failure_count, external_source_id,
			protocol, profile_compatibility, sort_order
		) VALUES(?, ?, 'active', 'real', 0, ?, ?, ?, ?)
	`, label, blindIndex, sourceID, protocol, compatibility, sortOrder)
	if err != nil {
		t.Fatalf("insert delivery key: %v", err)
	}
	keyID, _ := result.LastInsertId()
	env, _ := profilestorage.Encrypt([]byte(raw), activeID, activeKey, keyID)
	if _, err := app.db.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, env); err != nil {
		t.Fatalf("insert delivery key secret: %v", err)
	}
	if _, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, keyID); err != nil {
		t.Fatalf("assign delivery key: %v", err)
	}
	return keyID
}

func decodeObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode generated JSON: %v", err)
	}
	return result
}

func exclusionCodes(exclusions []generationExclusion) map[string]bool {
	result := make(map[string]bool)
	for _, exclusion := range exclusions {
		result[exclusion.Reason] = true
	}
	return result
}

func TestSubscriptionCapabilityMatrixIsCompleteAndVersionPinned(t *testing.T) {
	matrix := subscriptionCapabilityMatrix()
	if len(matrix) != 7 {
		t.Fatalf("capability profiles = %d, want 7", len(matrix))
	}
	want := map[string]bool{
		"vless/current": true, "vmess/current": true, "trojan/current": true,
		"shadowsocks/sip002/sip022": true, "hysteria2/2": true,
		"tuic/5": true, "tuic/4": true,
	}
	for _, capability := range matrix {
		delete(want, capability.Protocol+"/"+capability.Generation)
		for _, output := range []string{"plain", "base64", "mihomo", "sing-box", "xray-json", "structured-editing", "connectivity-probe", "compatibility-raw"} {
			claim, ok := capability.Outputs[output]
			if !ok {
				t.Errorf("%s/%s is missing %s", capability.Protocol, capability.Generation, output)
				continue
			}
			if claim.SyntaxValidation == "official_binary" {
				if claim.RuntimeInteroperability != "not_tested" {
					t.Fatalf("official syntax validation overstated runtime interoperability for %s/%s/%s", capability.Protocol, capability.Generation, output)
				}
				switch output {
				case "mihomo":
					if claim.TargetVersion != targetMihomoVersion || claim.MinimumVersion != targetMihomoVersion {
						t.Fatal("Mihomo capability is not pinned to the validated exact release")
					}
				case "sing-box":
					if claim.TargetVersion != targetSingBoxVersion || claim.MinimumVersion != targetSingBoxVersion {
						t.Fatal("sing-box capability is not pinned to the validated exact release")
					}
				case "xray-json":
					if claim.TargetVersion != targetXrayCurrentVersion || claim.MinimumVersion != targetXrayMinimumVersion {
						t.Fatal("Xray capability does not expose its validated minimum/current policy")
					}
				}
			}
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing capability profiles: %#v", want)
	}
	find := func(protocol, generation, output string) outputCapability {
		t.Helper()
		for _, capability := range matrix {
			if capability.Protocol == protocol && capability.Generation == generation {
				result, ok := capability.Outputs[output]
				if !ok {
					t.Fatalf("%s/%s is missing %s", protocol, generation, output)
				}
				return result
			}
		}
		t.Fatalf("missing capability %s/%s", protocol, generation)
		return outputCapability{}
	}
	if got := find("hysteria2", "2", "xray-json"); got.Status != capabilityConditional || got.TargetVersion != targetXrayCurrentVersion || got.MinimumVersion != targetXrayMinimumVersion || got.SyntaxValidation != "official_binary" || got.RuntimeInteroperability != "not_tested" {
		t.Fatalf("Hysteria 2 Xray capability = %#v", got)
	}
	if got := find("tuic", "5", "xray-json"); got.Status != capabilityUnsupported || got.ReasonCode != generationReasonUnsupportedProtocol {
		t.Fatalf("TUIC v5 Xray capability = %#v", got)
	}
	if got := find("tuic", "4", "plain"); got.Status != capabilityCompatibility || got.ReasonCode != "raw_delivery_only" {
		t.Fatalf("TUIC v4 plain capability = %#v", got)
	}
	encoded, err := json.Marshal(matrix)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, version := range []string{targetMihomoVersion, targetSingBoxVersion, targetXrayMinimumVersion, targetXrayCurrentVersion} {
		if !strings.Contains(text, version) {
			t.Errorf("target version %q is absent", version)
		}
	}
	for _, secret := range []string{"shadow-password", "hysteria-auth", "tuic-password"} {
		if strings.Contains(text, secret) {
			t.Fatal("capability JSON leaked synthetic credential material")
		}
	}
}

func TestCapabilityAPIUsesBackendMatrixAndRejectsClientOverride(t *testing.T) {
	app := newIntegrationApp(t)
	recorder := httptest.NewRecorder()
	app.apiV1GetSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/subscription-delivery-settings", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), targetMihomoVersion) || !strings.Contains(recorder.Body.String(), generationReasonUnsafeControl) {
		t.Fatalf("capability response status=%d or required metadata absent", recorder.Code)
	}
	var settings subscriptionDeliverySettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode capability response: %v", err)
	}
	if !reflect.DeepEqual(settings.GenerationExclusionReasonCodes, generationExclusionReasonCodes()) {
		t.Fatalf("exclusion code catalog = %#v", settings.GenerationExclusionReasonCodes)
	}

	input := `{"response_headers":[],"announcement":"","remarks":{},"capabilities":[{"protocol":"fake","generation":"1","outputs":{}}],"generation_exclusion_reason_codes":["fake_reason"]}`
	recorder = httptest.NewRecorder()
	app.apiV1UpdateSubscriptionDeliverySettings(recorder, httptest.NewRequest(http.MethodPut, "/api/v1/subscription-delivery-settings", strings.NewReader(input)))
	if recorder.Code != http.StatusBadRequest || strings.Contains(recorder.Body.String(), `"protocol":"fake"`) || strings.Contains(recorder.Body.String(), "fake_reason") {
		t.Fatalf("client capability override was not rejected safely: status=%d", recorder.Code)
	}
}

func TestOpenAPIDeliveryContractsRemainParseableAndReadOnly(t *testing.T) {
	raw, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	var document any
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse OpenAPI contract: %v", err)
	}
	text := string(raw)
	for _, required := range []string{
		"SubscriptionDeliverySettingsUpdate:",
		"SubscriptionDeliverySettings:",
		"SubscriptionGenerationFailure:",
		"readOnly: true",
		"additionalProperties: false",
		"runtime_interoperability:",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("OpenAPI delivery contract is missing %s", required)
		}
	}
}

func TestPlainDeliveryPreservesExactMixedRawAndBase64WrapsFinalBody(t *testing.T) {
	if _, err := profiles.Parse(deliveryHY2); err != nil {
		t.Fatalf("delivery Hysteria fixture: %v", err)
	}
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	items := []struct {
		raw, protocol, compatibility string
	}{
		{externalTestVLESS, "vless", "full"},
		{externalTestVMess, "vmess", "full"},
		{externalTestTrojan, "trojan", "full"},
		{deliverySSClean, "shadowsocks", "full"},
		{deliveryHY2, "hysteria2", "full"},
		{strings.Replace(deliveryHY2, "#HY2", "&ech=preserved-extension#HY2-extension", 1), "hysteria2", "full"},
		{deliveryTUICMihomo, "tuic", "full"},
		{externalTestTUICV4, "tuic", "read_only"},
	}
	wantLines := make([]string, 0, len(items))
	for index, item := range items {
		insertAssignedDeliveryKey(t, app, userID, nil, "item", item.raw, item.protocol, item.compatibility, index)
		wantLines = append(wantLines, item.raw)
	}
	want := strings.Join(wantLines, "\n")
	generated, _, denyCode, denyReason, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denyCode != 0 || generated.Body != want || len(generated.Exclusions) != 0 {
		t.Fatalf("plain output mismatch code=%d reason=%q err=%v exclusions=%d", denyCode, denyReason, err, len(generated.Exclusions))
	}

	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token/subbody", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	recorder := httptest.NewRecorder()
	app.handleSubscriptionSubBody(recorder, request)
	decoded, decodeErr := base64.StdEncoding.DecodeString(recorder.Body.String())
	if recorder.Code != http.StatusOK || decodeErr != nil || string(decoded) != want {
		t.Fatalf("base64 output status=%d decode=%v or decoded body mismatch", recorder.Code, decodeErr)
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
		insertAssignedDeliveryKey(t, app, userID, nil, item.protocol, item.raw, item.protocol, "full", index)
	}
	infoResult, err := app.db.Exec(`INSERT INTO vless_keys(label, status, key_kind, template_text, sort_order) VALUES('Info', 'active', 'informational', 'Hello {user_name}', 100)`)
	if err != nil {
		t.Fatalf("insert informational key: %v", err)
	}
	infoID, _ := infoResult.LastInsertId()
	if _, err := app.db.Exec(`INSERT INTO user_keys(user_id, key_id) VALUES(?, ?)`, userID, infoID); err != nil {
		t.Fatalf("assign informational key: %v", err)
	}

	generated, _, denyCode, denyReason, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denyCode != 0 {
		t.Fatalf("link generation code=%d reason=%q err=%v", denyCode, denyReason, err)
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
	if _, _, _, _, err := app.generateSelectedSubscription("subscription-token", "xray-json"); err != nil {
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
	keyID := insertAssignedDeliveryKey(t, app, userID, nil, "Panel only", raw, "xray-json", "full", 0)
	if _, err := app.db.Exec(`UPDATE vless_keys SET client_display_name = 'Subscriber node' WHERE id = ?`, keyID); err != nil {
		t.Fatal(err)
	}
	var envelopeBefore string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeBefore); err != nil {
		t.Fatal(err)
	}

	generated, _, denyCode, denyReason, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denyCode != 0 {
		t.Fatalf("link projection code=%d reason=%q err=%v", denyCode, denyReason, err)
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

	jsonGenerated, _, denyCode, _, err := app.generateSelectedSubscription("subscription-token", "xray-json")
	if err != nil || denyCode != 0 || !strings.Contains(jsonGenerated.Body, `"dns"`) || !strings.Contains(jsonGenerated.Body, `"routing"`) {
		t.Fatalf("JSON mode lost original document: code=%d err=%v body=%q", denyCode, err, jsonGenerated.Body)
	}
	var envelopeAfter string
	if err := app.db.QueryRow(`SELECT encrypted_url FROM vless_key_secrets WHERE vless_key_id = ?`, keyID).Scan(&envelopeAfter); err != nil {
		t.Fatal(err)
	}
	if envelopeAfter != envelopeBefore {
		t.Fatal("switching output formats rewrote encrypted profile state")
	}
}

func TestDeliveryDedupIsSemanticCurrentKeyAndPersistenceReadOnly(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	firstSource := seedExternalProfileSource(t, app, "https://delivery.example/first")
	secondSource := seedExternalProfileSource(t, app, "https://delivery.example/second")
	firstRaw := "hy2://auth@HY.EXAMPLE:443?sni=tls.example#First-name"
	secondRaw := "hysteria2://auth@hy.example?sni=tls.example#Second-name"
	// Insert the higher-priority source second to prove row insertion order does
	// not override the documented source-id then row-id winner policy.
	secondID := insertAssignedDeliveryKey(t, app, userID, secondSource, "second", secondRaw, "hysteria2", "full", 0)
	firstID := insertAssignedDeliveryKey(t, app, userID, firstSource, "first", firstRaw, "hysteria2", "full", 0)
	if _, err := app.db.Exec(`UPDATE vless_keys SET profile_fingerprint = CASE id WHEN ? THEN 'pf1_rotated_old' ELSE 'pf1_current_but_different' END WHERE id IN (?, ?)`, secondID, firstID, secondID); err != nil {
		t.Fatal(err)
	}
	type persistedDeliveryState struct {
		ID          int64
		SourceID    sql.NullInt64
		Raw         string
		Label       string
		Status      string
		Fingerprint string
		CreatedAt   string
	}
	loadState := func() []persistedDeliveryState {
		t.Helper()
		rows, queryErr := app.db.Query(`
			SELECT k.id, k.external_source_id, s.encrypted_url, k.label, k.status, COALESCE(k.profile_fingerprint, ''), CAST(k.created_at AS TEXT)
			FROM vless_keys k LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id WHERE k.id IN (?, ?) ORDER BY k.id
		`, firstID, secondID)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		defer rows.Close()
		var result []persistedDeliveryState
		for rows.Next() {
			var item persistedDeliveryState
			if scanErr := rows.Scan(&item.ID, &item.SourceID, &item.Raw, &item.Label, &item.Status, &item.Fingerprint, &item.CreatedAt); scanErr != nil {
				t.Fatal(scanErr)
			}
			result = append(result, item)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			t.Fatal(rowsErr)
		}
		return result
	}
	before := loadState()

	generated, _, _, _, err := app.generateSelectedSubscription("subscription-token", "plain")
	expectedWinner, expectedErr := shareURIWithDisplayName(firstRaw, "first")
	if err != nil || expectedErr != nil || generated.Body != expectedWinner {
		t.Fatalf("semantic winner mismatch (err=%v)", err)
	}
	var rowCount, assignmentCount int
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE id IN (?, ?)`, firstID, secondID).Scan(&rowCount)
	_ = app.db.QueryRow(`SELECT COUNT(*) FROM user_keys WHERE user_id = ? AND key_id IN (?, ?)`, userID, firstID, secondID).Scan(&assignmentCount)
	if rowCount != 2 || assignmentCount != 2 {
		t.Fatalf("delivery mutated source-owned persistence: rows=%d assignments=%d", rowCount, assignmentCount)
	}
	if after := loadState(); !reflect.DeepEqual(before, after) {
		t.Fatal("delivery changed source-owned row state")
	}

	aliasA := deliveryTUICSingBox
	aliasB := strings.ReplaceAll(aliasA, "server_name=tls.example", "sni=tls.example")
	aliasB = strings.ReplaceAll(aliasB, "allow_insecure=true", "skip-cert-verify=true")
	identityA, errA := app.deliveryIdentity(aliasA)
	identityB, errB := app.deliveryIdentity(aliasB)
	if errA != nil || errB != nil || identityA != identityB {
		t.Fatalf("TUIC aliases did not normalize: err=%v/%v", errA, errB)
	}

	for _, pair := range [][2]string{
		{deliverySSClean, strings.Replace(deliverySSClean, "c2hhZG93LXBhc3N3b3Jk", "ZGlmZmVyZW50LXBhc3N3b3Jk", 1)},
		{"hy2://auth@hy.example:443?sni=a.example", "hy2://auth@hy.example:443?sni=b.example"},
		{"hy2://auth@hy.example:443?obfs=salamander&obfs-password=one", "hy2://auth@hy.example:443?obfs=salamander&obfs-password=two"},
		{deliveryTUICSingBox, strings.Replace(deliveryTUICSingBox, "tuic-password", "different-password", 1)},
	} {
		left, leftErr := app.deliveryIdentity(pair[0])
		right, rightErr := app.deliveryIdentity(pair[1])
		if leftErr != nil || rightErr != nil || left == right {
			t.Errorf("connectivity-changing profiles collapsed: err=%v/%v", leftErr, rightErr)
		}
	}
}

func TestUnsafeAndMalformedStoredRowsAreSafelyExcluded(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	insertAssignedDeliveryKey(t, app, userID, nil, "valid", deliverySSClean, "shadowsocks", "full", 0)
	unsafeSecret := "trojan://never-log-this-password@example.com:443\nss://injected"
	malformedSecret := "hysteria2://never-log-this-auth@host:0"
	insertAssignedDeliveryKey(t, app, userID, nil, "unsafe", unsafeSecret, "trojan", "legacy", 1)
	insertAssignedDeliveryKey(t, app, userID, nil, "malformed", malformedSecret, "hysteria2", "full", 2)

	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	generated, _, _, _, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || generated.Body != deliverySSClean || len(generated.Exclusions) != 2 {
		t.Fatalf("safe partial output mismatch exclusions=%d err=%v", len(generated.Exclusions), err)
	}
	codes := exclusionCodes(generated.Exclusions)
	if !codes[generationReasonUnsafeControl] || !codes[generationReasonInvalidStored] {
		t.Fatalf("unexpected exclusion codes: %#v", codes)
	}
	formatted := strings.Join([]string{logs.String(), generated.Exclusions[0].Error(), generated.Exclusions[1].Error()}, " ")
	for _, secret := range []string{"never-log-this-password", "never-log-this-auth", unsafeSecret, malformedSecret} {
		if strings.Contains(formatted, secret) {
			t.Fatal("generation diagnostics leaked credential-bearing material")
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token/subbody/plain", nil)
	request.SetPathValue("subscription_id", "subscription-token")
	recorder := httptest.NewRecorder()
	app.handleSubscriptionSubBodyPlain(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != deliverySSClean {
		t.Fatalf("safe partial handler status=%d or body mismatch", recorder.Code)
	}
	if recorder.Header().Get("SubShare-Excluded-Count") != "2" || !strings.Contains(recorder.Header().Get("SubShare-Exclusion-Codes"), generationReasonUnsafeControl) {
		t.Fatalf("safe exclusion headers: %#v", recorder.Header())
	}
	var auditMetadata string
	if err := app.db.QueryRow(`SELECT COALESCE(GROUP_CONCAT(metadata_json, ''), '') FROM audit_events`).Scan(&auditMetadata); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{unsafeSecret, malformedSecret, deliverySSClean, base64.StdEncoding.EncodeToString([]byte(deliverySSClean))} {
		if strings.Contains(logs.String(), forbidden) || strings.Contains(auditMetadata, forbidden) {
			t.Fatalf("logs or audit contain credential-bearing delivery material")
		}
	}
}

func TestSubscriptionControlValidationRejectsCRLFNULAndDEL(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  string
	}{
		{"cr", "ss://safe\rsecond"},
		{"lf", "ss://safe\nsecond"},
		{"nul", "ss://safe\x00second"},
		{"del", "ss://safe\x7fsecond"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !hasUnsafeSubscriptionControl(test.raw) {
				t.Fatal("unsafe control was accepted")
			}
		})
	}
}

func TestStructuredDeliveryWithOnlyExcludedProfilesReturnsAllExcludedFailure(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	insertAssignedDeliveryKey(t, app, userID, nil, "TUIC v4", externalTestTUICV4, "tuic", "read_only", 0)

	generated, _, denyCode, denyReason, err := app.generateSelectedSubscription("subscription-token", "mihomo")
	if err != nil {
		t.Fatal(err)
	}
	if denyCode != http.StatusUnprocessableEntity || denyReason != generationReasonAllExcluded || generated.Body != "" || generated.GeneratedCount != 0 {
		t.Fatalf("empty structured result code=%d reason=%q or non-empty body", denyCode, denyReason)
	}
	if len(generated.Exclusions) != 1 || generated.Exclusions[0].Reason != generationReasonCompatibility {
		t.Fatalf("empty structured exclusions = %#v", generated.Exclusions)
	}
}

func TestStructuredDeliveryAllExcludedHandlerPolicy(t *testing.T) {
	tests := []struct {
		name          string
		userAgent     string
		raw           string
		protocol      string
		compatibility string
		reason        string
		forbidden     []string
	}{
		{
			name: "TUIC v5 requested as Xray", userAgent: "xray-core",
			raw: validationTUICBasic, protocol: "tuic", compatibility: "full",
			reason:    generationReasonUnsupportedProtocol,
			forbidden: []string{"synthetic-password", "tuic-basic.example", "TUIC-basic"},
		},
		{
			name: "TUIC v4 requested as Mihomo", userAgent: "mihomo",
			raw: externalTestTUICV4, protocol: "tuic", compatibility: "read_only",
			reason:    generationReasonCompatibility,
			forbidden: []string{"legacy-token-value", "legacy.example", "Legacy"},
		},
		{
			name: "Shadowsocks plugin requested as Xray", userAgent: "xray-core",
			raw: deliverySSPlugin, protocol: "shadowsocks", compatibility: "full",
			reason:    generationReasonPlugin,
			forbidden: []string{"shadow-password", "ss.example", "SS-plugin"},
		},
		{
			name: "Hysteria insecure requested as Xray", userAgent: "xray-core",
			raw: "hy2://synthetic-auth@unsafe-tls.example:443?insecure=1#Unsafe-TLS", protocol: "hysteria2", compatibility: "full",
			reason:    generationReasonUnrepresentable,
			forbidden: []string{"synthetic-auth", "unsafe-tls.example", "Unsafe-TLS"},
		},
		{
			name: "malformed stored profile", userAgent: "xray-core",
			raw: "hysteria2://synthetic-auth@malformed.example:0#Malformed", protocol: "hysteria2", compatibility: "full",
			reason:    generationReasonInvalidStored,
			forbidden: []string{"synthetic-auth", "malformed.example", "Malformed"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			userID := seedSubscriptionUser(t, app, model.UserStatusActive)
			insertAssignedDeliveryKey(t, app, userID, nil, "sensitive display", test.raw, test.protocol, test.compatibility, 0)
			request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
			request.SetPathValue("subscription_id", "subscription-token")
			request.Header.Set("User-Agent", test.userAgent)
			recorder := httptest.NewRecorder()
			app.handleSubscription(recorder, request)

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d, want 422", recorder.Code)
			}
			var payload subscriptionGenerationFailure
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode safe all-excluded response: %v", err)
			}
			if payload.ErrorCode != generationReasonAllExcluded || payload.EligibleCount != 1 || payload.ExcludedCount != 1 || payload.ExclusionCounts[test.reason] != 1 {
				t.Fatalf("unexpected safe all-excluded summary: code=%q eligible=%d excluded=%d", payload.ErrorCode, payload.EligibleCount, payload.ExcludedCount)
			}
			if recorder.Header().Get("SubShare-Excluded-Count") != "1" || recorder.Header().Get("SubShare-Exclusion-Codes") != test.reason || recorder.Header().Get("SubShare-Exclusion-Counts") != test.reason+"=1" {
				t.Fatal("all-excluded headers do not match the response summary")
			}
			diagnostics := recorder.Body.String() + recorder.Header().Get("SubShare-Exclusion-Codes") + recorder.Header().Get("SubShare-Exclusion-Counts")
			for _, forbidden := range append(test.forbidden, test.raw, "sensitive display", "key:") {
				if strings.Contains(diagnostics, forbidden) {
					t.Fatal("all-excluded response leaked per-profile material")
				}
			}
		})
	}
}

func TestStructuredDeliveryMixedAndNoEligiblePolicies(t *testing.T) {
	t.Run("mixed supported and excluded", func(t *testing.T) {
		app := newIntegrationApp(t)
		userID := seedSubscriptionUser(t, app, model.UserStatusActive)
		insertAssignedDeliveryKey(t, app, userID, nil, "supported", deliverySSClean, "shadowsocks", "full", 0)
		insertAssignedDeliveryKey(t, app, userID, nil, "excluded", validationTUICBasic, "tuic", "full", 1)

		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		request.SetPathValue("subscription_id", "subscription-token")
		request.Header.Set("User-Agent", "xray-core")
		recorder := httptest.NewRecorder()
		app.handleSubscription(recorder, request)
		if recorder.Code != http.StatusOK || recorder.Header().Get("SubShare-Excluded-Count") != "1" || recorder.Header().Get("SubShare-Exclusion-Codes") != generationReasonUnsupportedProtocol {
			t.Fatalf("mixed response status=%d or exclusion summary mismatch", recorder.Code)
		}
		if err := validateGeneratedStructuredBody("xray-json", recorder.Body.String()); err != nil {
			t.Fatal("mixed response did not retain its supported Xray profile")
		}
	})

	t.Run("no assigned profiles", func(t *testing.T) {
		app := newIntegrationApp(t)
		seedSubscriptionUser(t, app, model.UserStatusActive)
		request := httptest.NewRequest(http.MethodGet, "/sub/subscription-token", nil)
		request.SetPathValue("subscription_id", "subscription-token")
		request.Header.Set("User-Agent", "xray-core")
		recorder := httptest.NewRecorder()
		app.handleSubscription(recorder, request)
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("no-assignment response status=%d, want 503", recorder.Code)
		}
	})
}

func TestExclusionHeadersAreAggregatedDeterministicallyAndBounded(t *testing.T) {
	exclusions := make([]generationExclusion, 0, 1500)
	for index := 0; index < 1000; index++ {
		exclusions = append(exclusions, generationExclusion{Reason: generationReasonUnsupportedProtocol})
	}
	for index := 0; index < 500; index++ {
		exclusions = append(exclusions, generationExclusion{Reason: generationReasonCompatibility})
	}
	first := http.Header{}
	second := http.Header{}
	applyGenerationExclusionHeaders(first, exclusions)
	applyGenerationExclusionHeaders(second, append([]generationExclusion(nil), exclusions...))
	if !reflect.DeepEqual(first, second) {
		t.Fatal("exclusion headers were not deterministic")
	}
	if first.Get("SubShare-Excluded-Count") != "1500" || first.Get("SubShare-Exclusion-Codes") != generationReasonCompatibility+","+generationReasonUnsupportedProtocol {
		t.Fatal("exclusion headers were not aggregated in stable order")
	}
	if first.Get("SubShare-Exclusion-Counts") != generationReasonCompatibility+"=500,"+generationReasonUnsupportedProtocol+"=1000" {
		t.Fatal("exclusion count summary mismatch")
	}
	if len(first.Get("SubShare-Exclusion-Counts")) > 512 {
		t.Fatal("aggregated exclusion header exceeded its fixed safe bound")
	}
}

func TestMihomoMappingsAndSafeExclusions(t *testing.T) {
	entries := []deliveryEntry{
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
	first, err := renderMihomoEntries(entries)
	if err != nil {
		t.Fatal(err)
	}
	second, err := renderMihomoEntries(entries)
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
	if !codes[generationReasonClientVersion] || !codes[generationReasonCompatibility] || !codes[generationReasonUnrepresentable] || !codes[generationReasonAmbiguous] {
		t.Fatalf("Mihomo exclusions: %#v", first.Exclusions)
	}
	for _, exclusion := range first.Exclusions {
		if strings.Contains(exclusion.Error(), "gecko-secret") || strings.Contains(exclusion.Error(), "legacy-token-value") {
			t.Fatalf("secret-bearing exclusion: %v", exclusion)
		}
	}
}

func TestSingBoxMappingsProvenanceAndExclusions(t *testing.T) {
	entries := []deliveryEntry{
		{ID: 1, Raw: deliverySSPlugin, Kind: model.KeyKindReal},
		{ID: 2, Raw: "hysteria2://auth@[2001:db8::2]:443,5000-5002?sni=hy.example&insecure=1&obfs=salamander&obfs-password=secret#HY", Kind: model.KeyKindReal},
		{ID: 3, Raw: deliveryTUICSingBox, Kind: model.KeyKindReal},
		{ID: 4, Raw: deliveryTUICMihomo, Kind: model.KeyKindReal},
		{ID: 5, Raw: "hy2://auth@hy.example:443?pinSHA256=abababababababababababababababababababababababababababababababab", Kind: model.KeyKindReal},
		{ID: 6, Raw: externalTestTUICV4, Kind: model.KeyKindReal},
		{ID: 7, Raw: "hysteria2://auth@hy.example:443?obfs=gecko&obfs-password=gecko-secret", Kind: model.KeyKindReal},
	}
	generated, err := renderSingBoxEntries(entries)
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
	if !codes[generationReasonUnrepresentable] || !codes[generationReasonCompatibility] || !codes[generationReasonClientVersion] {
		t.Fatalf("sing-box exclusions: %#v", generated.Exclusions)
	}
}

func TestXrayPartialOutputPreservesLegacyAndSupportsPluginFreeShadowsocks(t *testing.T) {
	entries := []deliveryEntry{
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
	generated, err := renderXrayEntries(entries)
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
	if !codes[generationReasonPlugin] || !codes[generationReasonUnsupportedProtocol] || !codes[generationReasonCompatibility] || !codes[generationReasonUnrepresentable] {
		t.Fatalf("Xray exclusions: %#v", generated.Exclusions)
	}
}

func TestXrayRejectsRemovedInsecureTLSField(t *testing.T) {
	generated, err := renderXrayEntries([]deliveryEntry{{
		ID: 1, Raw: "hy2://synthetic-auth@xray-insecure.example:443?insecure=1", Kind: model.KeyKindReal,
	}})
	if err != nil {
		t.Fatalf("Xray insecure exclusion failed: %v", err)
	}
	if generated.GeneratedCount != 0 || generated.Body != "[]" || len(generated.Exclusions) != 1 || generated.Exclusions[0].Reason != generationReasonUnrepresentable {
		t.Fatal("removed Xray allowInsecure field was not safely excluded")
	}
	if strings.Contains(generated.Exclusions[0].Error(), "synthetic-auth") {
		t.Fatal("Xray insecure exclusion leaked authentication material")
	}
}

func TestStructuredNamesAreSafeUniqueAndSerializationValidationRejectsInjection(t *testing.T) {
	names := &uniqueNames{}
	if first, second := names.next("node\nsecret", ""), names.next("node\nsecret", ""); first != "nodesecret" || second != "nodesecret (2)" {
		t.Fatalf("safe names = %q, %q", first, second)
	}
	if err := validateGeneratedStructuredBody("mihomo", "proxies:\n  - name: ["); err == nil {
		t.Fatal("invalid Mihomo YAML was accepted")
	}
	if err := validateGeneratedStructuredBody("sing-box", `{"outbounds":[`); err == nil {
		t.Fatal("invalid sing-box JSON was accepted")
	}
}

func TestFinalStructuredTemplateValidationRejectsUnsafeTransforms(t *testing.T) {
	tests := []struct {
		name   string
		format string
		body   string
	}{
		{"YAML scalar coercion", "mihomo", "proxies:\n  - name: true\n    type: ss\n"},
		{"YAML duplicate key", "mihomo", "proxies:\n  - name: first\n    name: second\n    type: ss\n"},
		{"YAML malformed indentation", "mihomo", "proxies:\n - name: edge\n   type: ss\n    server: example.test\n"},
		{"YAML removed required structure", "mihomo", "proxy-groups: []\n"},
		{"sing-box JSON type replacement", "sing-box", `{"outbounds":"removed"}`},
		{"sing-box JSON empty required structure", "sing-box", `{"outbounds":[]}`},
		{"sing-box JSON invalid outbound type", "sing-box", `{"outbounds":[{"type":7}]}`},
		{"Xray JSON object replaces array", "xray-json", `{"outbounds":[{"protocol":"vless"}]}`},
		{"Xray JSON removed required structure", "xray-json", `[{"routing":{}}]`},
		{"Xray JSON invalid protocol type", "xray-json", `[{"outbounds":[{"protocol":false}]}]`},
		{"unsafe NUL", "sing-box", "{\"outbounds\":[{\"type\":\"direct\"}]}\x00"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateGeneratedStructuredBody(test.format, test.body); err == nil || err.Error() != generationReasonSerialization {
				t.Fatal("unsafe or structurally invalid templated output was accepted")
			}
		})
	}

	valid := map[string]string{
		"mihomo":    `{"proxies":[{"name":"edge","type":"ss"}]}`,
		"sing-box":  `{"outbounds":[{"tag":"edge","type":"shadowsocks"}]}`,
		"xray-json": `[{"outbounds":[{"tag":"edge","protocol":"shadowsocks"}]}]`,
	}
	for format, body := range valid {
		final := applyTemplateContent("{{subscription}}", body, "ignored")
		if err := validateGeneratedStructuredBody(format, final); err != nil {
			t.Fatalf("valid final %s template was rejected", format)
		}
	}
}

func TestStructuredTemplateRequiresSubscriptionPlaceholder(t *testing.T) {
	for _, format := range []string{"mihomo", "sing-box", "xray-json"} {
		_, err := validateTemplateInput(templateInput{Name: "safe", Format: format, Content: `{"replaced":true}`})
		if err == nil {
			t.Fatalf("%s template without subscription placeholder was accepted", format)
		}
	}
}
