package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseRuleMatches(t *testing.T) {
	t.Parallel()

	header := http.Header{
		"User-Agent":  []string{"Happ/3.0 (Android)"},
		"X-Device-Os": []string{"android"},
	}
	rule := responseRule{
		Operator: "AND",
		Conditions: []responseRuleCondition{
			{HeaderName: "user-agent", Operator: "CONTAINS", Value: "happ", CaseSensitive: false},
			{HeaderName: "x-device-os", Operator: "EQUALS", Value: "android", CaseSensitive: false},
		},
	}
	if !responseRuleMatches(rule, header) {
		t.Fatal("expected AND rule to match")
	}
	rule.Conditions[1].Value = "ios"
	if responseRuleMatches(rule, header) {
		t.Fatal("expected AND rule not to match")
	}
	rule.Operator = "OR"
	if !responseRuleMatches(rule, header) {
		t.Fatal("expected OR rule to match")
	}
}

func TestValidateResponseRuleInputRejectsUnsafeValues(t *testing.T) {
	t.Parallel()

	base := responseRuleInput{
		Name:         "Happ",
		Enabled:      true,
		Priority:     10,
		Operator:     "AND",
		ResponseType: "xray-json",
		Conditions: []responseRuleCondition{
			{HeaderName: "user-agent", Operator: "REGEX", Value: "[", CaseSensitive: false},
		},
	}
	if _, err := validateResponseRuleInput(base); err == nil {
		t.Fatal("invalid regex was accepted")
	}

	base.Conditions[0].Operator = "CONTAINS"
	base.Conditions[0].Value = "happ"
	base.Headers = []responseHeader{{Key: "Set-Cookie", Value: "admin=true"}}
	if _, err := validateResponseRuleInput(base); err == nil {
		t.Fatal("unsafe response header was accepted")
	}
	for _, key := range []string{"announce", "Announce", "ANNOUNCE", "profile-title", "profile-update-interval", "profile-web-page-url", "support-url", "subscription-userinfo"} {
		base.Headers = []responseHeader{{Key: key, Value: "override"}}
		if _, err := validateResponseRuleInput(base); err == nil {
			t.Errorf("reserved response header %q was accepted", key)
		}
	}

	base.Headers = []responseHeader{{Key: "X-Provider-ID", Value: "provider"}}
	if _, err := validateResponseRuleInput(base); err != nil {
		t.Fatalf("valid response rule rejected: %v", err)
	}
}

func TestApplyRuleHeadersProtectsCanonicalMetadataFromLegacyStoredRules(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	recorder.Header().Set("announce", "base64:Y2Fub25pY2Fs")
	applyRuleHeaders(recorder, []responseHeader{
		{Key: "ANNOUNCE", Value: "custom"},
		{Key: "Profile-Title", Value: "custom"},
		{Key: "X-Safe-Custom", Value: "kept"},
	})
	if got := recorder.Header().Get("announce"); got != "base64:Y2Fub25pY2Fs" {
		t.Fatalf("canonical announce was overwritten: %q", got)
	}
	if got := recorder.Header().Get("Profile-Title"); got != "" {
		t.Fatalf("reserved profile title was applied: %q", got)
	}
	if got := recorder.Header().Get("X-Safe-Custom"); got != "kept" {
		t.Fatalf("safe custom header=%q", got)
	}
}

func TestApplyTemplateContent(t *testing.T) {
	t.Parallel()

	got := applyTemplateContent("title: {{title}}\nproxies:\n{{subscription}}", "- proxy-a", "SubShare")
	want := "title: SubShare\nproxies:\n- proxy-a"
	if got != want {
		t.Fatalf("applyTemplateContent() = %q, want %q", got, want)
	}
}

func TestRenderClientFormats(t *testing.T) {
	t.Parallel()
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=reality&sni=edge.example.com&fp=chrome&pbk=public&sid=abcd&type=ws&path=%2Fws&host=cdn.example.com#Edge"

	mihomo, err := renderMihomoSubscription(raw)
	if err != nil {
		t.Fatalf("render Mihomo: %v", err)
	}
	if !strings.Contains(mihomo, `"type": "vless"`) || !strings.Contains(mihomo, `"reality-opts"`) {
		t.Fatalf("unexpected Mihomo output: %s", mihomo)
	}

	singBox, err := renderSingBoxSubscription(raw)
	if err != nil {
		t.Fatalf("render Sing-box: %v", err)
	}
	if !strings.Contains(singBox, `"outbounds"`) || !strings.Contains(singBox, `"public_key": "public"`) {
		t.Fatalf("unexpected Sing-box output: %s", singBox)
	}
}

func TestResponseRuleRejectsTemplateFormatMismatch(t *testing.T) {
	app := newIntegrationApp(t)
	result, err := app.db.Exec(`
		INSERT INTO subscription_templates(slug, name, format, content, enabled)
		VALUES('plain-only', 'Plain only', 'plain', '', 1)
	`)
	if err != nil {
		t.Fatalf("insert template: %v", err)
	}
	templateID, _ := result.LastInsertId()
	input := responseRuleInput{
		Name:         "Mismatch",
		Enabled:      true,
		Priority:     10,
		Operator:     "AND",
		ResponseType: "mihomo",
		TemplateID:   &templateID,
		Conditions: []responseRuleCondition{
			{HeaderName: "user-agent", Operator: "CONTAINS", Value: "mihomo"},
		},
		Headers: []responseHeader{},
	}
	body, _ := json.Marshal(input)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/response-rules", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	app.saveResponseRule(recorder, request, nil)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "template_format_mismatch") {
		t.Fatalf("unexpected error: %s", recorder.Body.String())
	}
}

func TestTemplatePreviewValidatesStructuredOutput(t *testing.T) {
	t.Parallel()
	input := templateInput{Name: "Sing-box", Format: "sing-box", Enabled: true}
	preview, contentType, err := renderTemplatePreview(input)
	if err != nil {
		t.Fatalf("render preview: %v", err)
	}
	if contentType != "application/json; charset=utf-8" || !json.Valid([]byte(preview)) {
		t.Fatalf("unexpected preview: type=%q body=%s", contentType, preview)
	}

	input.Content = `{"broken": {{subscription}}`
	if _, _, err := renderTemplatePreview(input); err == nil {
		t.Fatal("invalid rendered JSON was accepted")
	}
}
