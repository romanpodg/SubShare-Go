package main

import (
	"encoding/base64"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestShareURIWithDisplayNameCoversAllSupportedProtocols(t *testing.T) {
	items := []string{externalTestVLESS, externalTestVMess, externalTestTrojan, deliverySSClean, deliveryHY2, deliveryTUICSingBox}
	for _, raw := range items {
		link, err := delivery.ShareURIWithDisplayName(raw, "Subscriber name")
		if err != nil {
			t.Fatalf("rename %s: %v", profileconfig.SupportedConfigScheme(raw), err)
		}
		if got := profileconfig.ClientDisplayNameFromKeyURL(link, ""); got != "Subscriber name" {
			t.Fatalf("%s client name=%q link=%q", profileconfig.SupportedConfigScheme(raw), got, link)
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
		insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, Label: "item", Raw: item.raw, Protocol: item.protocol, Compatibility: item.compatibility, SortOrder: index})
		wantLines = append(wantLines, item.raw)
	}
	want := strings.Join(wantLines, "\n")
	generated, denial, err := app.generateSelectedSubscription("subscription-token", "plain")
	if err != nil || denial.Code != 0 || generated.Body != want || len(generated.Exclusions) != 0 {
		t.Fatalf("plain output mismatch code=%d reason=%q err=%v exclusions=%d", denial.Code, denial.Reason, err, len(generated.Exclusions))
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

func TestDeliveryDedupIsSemanticCurrentKeyAndPersistenceReadOnly(t *testing.T) {
	app := newIntegrationApp(t)
	userID := seedSubscriptionUser(t, app, model.UserStatusActive)
	firstSource := seedExternalProfileSource(t, app, "https://delivery.example/first")
	secondSource := seedExternalProfileSource(t, app, "https://delivery.example/second")
	firstRaw := "hy2://auth@HY.EXAMPLE:443?sni=tls.example#First-name"
	secondRaw := "hysteria2://auth@hy.example?sni=tls.example#Second-name"
	// Insert the higher-priority source second to prove row insertion order does
	// not override the documented source-id then row-id winner policy.
	secondID := insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, SourceID: &secondSource, Label: "second", Raw: secondRaw, Protocol: "hysteria2", Compatibility: "full", SortOrder: 0})
	firstID := insertAssignedDeliveryKey(t, app, assignedDeliveryKeySeed{UserID: userID, SourceID: &firstSource, Label: "first", Raw: firstRaw, Protocol: "hysteria2", Compatibility: "full", SortOrder: 0})
	if _, err := app.db.Exec(`UPDATE vless_keys SET profile_fingerprint = CASE id WHEN ? THEN 'pf1_rotated_old' ELSE 'pf1_current_but_different' END WHERE id IN (?, ?)`, secondID, firstID, secondID); err != nil {
		t.Fatal(err)
	}
	before := loadPersistedDeliveryState(t, app, firstID, secondID)

	generated, _, err := app.generateSelectedSubscription("subscription-token", "plain")
	const expectedWinner = "hysteria2://auth@hy.example:443/?sni=tls.example#first"
	if err != nil || generated.Body != expectedWinner {
		t.Fatalf("semantic winner mismatch (err=%v)", err)
	}
	rowCount := profileFixtureCount(t, app, `SELECT COUNT(*) FROM vless_keys WHERE id IN (?, ?)`, firstID, secondID)
	assignmentCount := profileFixtureCount(t, app, `SELECT COUNT(*) FROM user_keys WHERE user_id = ? AND key_id IN (?, ?)`, userID, firstID, secondID)
	if rowCount != 2 || assignmentCount != 2 {
		t.Fatalf("delivery mutated source-owned persistence: rows=%d assignments=%d", rowCount, assignmentCount)
	}
	if after := loadPersistedDeliveryState(t, app, firstID, secondID); !reflect.DeepEqual(before, after) {
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
