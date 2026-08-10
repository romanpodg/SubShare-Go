package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/middleware"
)

func TestBuildInformationalXrayJSON(t *testing.T) {
	raw := "Подписка обновлена\nСрок продлён"
	encoded := buildInformationalXrayJSON(raw)
	if encoded == "" {
		t.Fatal("expected non-empty informational JSON payload")
	}
	if !json.Valid([]byte(encoded)) {
		t.Fatalf("informational payload is not valid JSON: %s", encoded)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(encoded), &parsed); err != nil {
		t.Fatalf("failed to parse informational payload: %v", err)
	}

	remarks, _ := parsed["remarks"].(string)
	if remarks == "" {
		t.Fatal("expected remarks in informational payload")
	}

	meta, _ := parsed["meta"].(map[string]any)
	serverDescription, _ := meta["serverDescription"].(string)
	if serverDescription != "Подписка обновлена" {
		t.Fatalf("unexpected server description: %q", serverDescription)
	}

	outbounds, _ := parsed["outbounds"].([]any)
	if len(outbounds) == 0 {
		t.Fatal("expected outbounds in informational payload")
	}
	firstOutbound, _ := outbounds[0].(map[string]any)
	protocol, _ := firstOutbound["protocol"].(string)
	if protocol != "vless" {
		t.Fatalf("unexpected outbound protocol: %q", protocol)
	}
}

func TestRenderInfoTemplateSupportedVariablesAndUnknownPreservation(t *testing.T) {
	data := subscriptionTemplateData{
		UserName: "Иван", Telegram: "ivan_example", SubscriptionID: "sample-id",
		ExpiryDate: "25/08/2026", ExpiryDateTime: "25/08/2026 12:00", RealKeysCount: 6,
	}
	template := "{user_name}|{telegram}|{subscription_id}|{expires_date}|{expires_at}|{real_keys_count}|{unknown}|<script>alert(1)</script>"
	want := "Иван|ivan_example|sample-id|25/08/2026|25/08/2026 12:00|6|{unknown}|<script>alert(1)</script>"
	if got := renderInfoTemplate(template, data); got != want {
		t.Fatalf("rendered template=%q want=%q", got, want)
	}
	if got := renderInfoTemplate("", data); got != "" {
		t.Fatalf("empty template=%q", got)
	}
}

func TestResolveBaseURLTrustsOnlyConfiguredProxyAndValidOrigin(t *testing.T) {
	app := &App{}
	middleware.ConfigureTrustedProxyNetworks([]netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")})
	t.Cleanup(func() { middleware.ConfigureTrustedProxyNetworks(nil) })

	untrusted := httptest.NewRequest(http.MethodGet, "http://direct.example/sub", nil)
	untrusted.RemoteAddr = "198.51.100.20:443"
	untrusted.Header.Set("X-Forwarded-Proto", "https")
	untrusted.Header.Set("X-Forwarded-Host", "spoofed.example")
	if got := app.resolveBaseURL(untrusted); got != "http://direct.example" {
		t.Fatalf("untrusted forwarded origin used: %q", got)
	}

	trusted := httptest.NewRequest(http.MethodGet, "http://backend:8080/sub", nil)
	trusted.RemoteAddr = "10.1.2.3:443"
	trusted.Header.Set("X-Forwarded-Proto", "https")
	trusted.Header.Set("X-Forwarded-Host", "vpn.example")
	if got := app.resolveBaseURL(trusted); got != "https://vpn.example" {
		t.Fatalf("trusted origin ignored: %q", got)
	}

	trusted.Header.Set("X-Forwarded-Host", "good.example/evil")
	if got := app.resolveBaseURL(trusted); got != "https://backend:8080" {
		t.Fatalf("invalid forwarded host accepted: %q", got)
	}
}
