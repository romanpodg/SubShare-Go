package profileconfig

import (
	"context"
	"encoding/base64"
	"net/netip"
	"strings"
	"testing"
)

func TestSupportedConfigScheme(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"vless://uuid@example.com:443", "vless"},
		{"vmess://eyJhZGQiOiJleGFtcGxlLmNvbSJ9", "vmess"},
		{"trojan://password@example.com:443", "trojan"},
		{"ss://YjY0@example.com:8388", "ss"},
		{"hy2://pass@example.com:443", "hy2"},
		{"hysteria2://pass@example.com:443", "hysteria2"},
		{"tuic://uuid:pass@example.com:443", "tuic"},
		{"{\"outbounds\":[]}", "xray-json"},
		{"   ", ""},
		{"unknown://test", "unknown"},
	}

	for _, tt := range tests {
		got := SupportedConfigScheme(tt.input)
		if got != tt.want {
			t.Errorf("SupportedConfigScheme(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateRealConfigURL(t *testing.T) {
	if err := ValidateRealConfigURL("vless://uuid@example.com:443"); err != nil {
		t.Fatalf("unexpected error for vless: %v", err)
	}
	if err := ValidateRealConfigURL("vmess://eyJhZGQiOiJleGFtcGxlLmNvbSJ9"); err != nil {
		t.Fatalf("unexpected error for vmess: %v", err)
	}
	if err := ValidateRealConfigURL("trojan://pass@example.com:443"); err != nil {
		t.Fatalf("unexpected error for trojan: %v", err)
	}
	if err := ValidateRealConfigURL("invalid"); err == nil {
		t.Fatal("expected error for invalid scheme")
	}
}

func TestParseConfigTarget_VLESS_Trojan_VMess_XrayJSON(t *testing.T) {
	// VLESS
	host, port, err := ParseConfigTarget("vless://user@vless.example.com:8443")
	if err != nil || host != "vless.example.com" || port != "8443" {
		t.Fatalf("vless target = (%q, %q, %v), want (vless.example.com, 8443, nil)", host, port, err)
	}

	// Trojan
	host, port, err = ParseConfigTarget("trojan://pass@trojan.example.com:443")
	if err != nil || host != "trojan.example.com" || port != "443" {
		t.Fatalf("trojan target = (%q, %q, %v), want (trojan.example.com, 443, nil)", host, port, err)
	}

	// VMess (valid Base64 JSON)
	vmessLink := "vmess://eyJhZGQiOiJ2bWVzcy5leGFtcGxlLmNvbSIsInBvcnQiOiI4MDgwIiwiaWQiOiJ1dWlkIn0="
	host, port, err = ParseConfigTarget(vmessLink)
	if err != nil || host != "vmess.example.com" || port != "8080" {
		t.Fatalf("vmess target = (%q, %q, %v), want (vmess.example.com, 8080, nil)", host, port, err)
	}

	// Xray JSON
	xrayJSON := `{
		"outbounds": [{
			"protocol": "vless",
			"settings": {
				"vnext": [{
					"address": "xray.example.com",
					"port": 443,
					"users": [{"id": "user-uuid"}]
				}]
			}
		}]
	}`
	host, port, err = ParseConfigTarget(xrayJSON)
	if err != nil || host != "xray.example.com" || port != "443" {
		t.Fatalf("xray-json target = (%q, %q, %v), want (xray.example.com, 443, nil)", host, port, err)
	}
}

func TestParseLinkConfiguration_VLESS_OmittedAndExplicitDefaults(t *testing.T) {
	// Omitted defaults
	draft, err := ParseLinkConfiguration("vless://my-uuid@vless.example.com?security=reality&pbk=pubkey123#MyVLESS")
	if err != nil {
		t.Fatalf("parse vless: %v", err)
	}
	if draft.Protocol != "vless" || draft.Server != "vless.example.com" || draft.Port != 443 || draft.Identifier != "my-uuid" {
		t.Fatalf("vless draft basic fields mismatch: %#v", draft)
	}
	if draft.Security != "reality" || draft.PublicKey != "pubkey123" || draft.Remark != "MyVLESS" {
		t.Fatalf("vless draft metadata mismatch: %#v", draft)
	}
	if draft.Encryption != "none" || draft.Network != "tcp" {
		t.Fatalf("vless default encryption/network mismatch: %#v", draft)
	}

	// Explicit parameters
	draft2, err := ParseLinkConfiguration("vless://my-uuid@vless.example.com:8443?type=ws&path=%2Fpath&security=tls&sni=sni.example.com&allowInsecure=1#Remark")
	if err != nil {
		t.Fatalf("parse vless explicit: %v", err)
	}
	if draft2.Port != 8443 || draft2.Network != "ws" || draft2.Path != "/path" || draft2.Security != "tls" || draft2.SNI != "sni.example.com" || !draft2.AllowInsecure {
		t.Fatalf("vless draft explicit mismatch: %#v", draft2)
	}
}

func TestParseLinkConfiguration_VMess_DecodingAndRecoverPlus(t *testing.T) {
	// vmess payload with '+' in encryption
	payload := `{"add":"vmess.example.com","port":"443","id":"uuid-123","ps":"VMessServer","scy":"aes 128 gcm"}`
	encoded := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))
	draft, err := ParseLinkConfiguration(encoded)
	if err != nil {
		t.Fatalf("parse vmess: %v", err)
	}
	if draft.Protocol != "vmess" || draft.Server != "vmess.example.com" || draft.Identifier != "uuid-123" {
		t.Fatalf("vmess draft basic mismatch: %#v", draft)
	}
	// "aes 128 gcm" should be recovered to "aes+128+gcm"
	if draft.VMessSecurity != "aes+128+gcm" {
		t.Fatalf("vmess security recovered = %q, want 'aes+128+gcm'", draft.VMessSecurity)
	}
}

func TestBuildShareLinkFromDraft_RoundTrip(t *testing.T) {
	original := "vless://test-uuid@vless.example.com:443?encryption=none&security=reality&type=tcp#MyNode"
	draft, err := ParseLinkConfiguration(original)
	if err != nil {
		t.Fatalf("parse link: %v", err)
	}
	rebuilt, err := BuildShareLinkFromDraft(draft)
	if err != nil {
		t.Fatalf("build share link: %v", err)
	}
	draft2, err := ParseLinkConfiguration(rebuilt)
	if err != nil {
		t.Fatalf("re-parse rebuilt link: %v", err)
	}
	if draft.Server != draft2.Server || draft.Port != draft2.Port || draft.Identifier != draft2.Identifier || draft.Security != draft2.Security {
		t.Fatalf("roundtrip draft mismatch: original=%#v rebuilt=%#v", draft, draft2)
	}
}

func TestIPv6_HostParsing(t *testing.T) {
	link := "vless://user-uuid@[2001:db8::1]:8443?type=tcp#IPv6Node"
	host, port, err := ParseConfigTarget(link)
	if err != nil || host != "2001:db8::1" || port != "8443" {
		t.Fatalf("IPv6 target = (%q, %q, %v), want (2001:db8::1, 8443, nil)", host, port, err)
	}

	draft, err := ParseLinkConfiguration(link)
	if err != nil || draft.Server != "2001:db8::1" || draft.Port != 8443 {
		t.Fatalf("IPv6 draft = (%q, %d, %v), want (2001:db8::1, 8443, nil)", draft.Server, draft.Port, err)
	}
}

func TestMalformedHostOrUTF8Input(t *testing.T) {
	// Missing host or invalid payload
	if _, _, err := ParseConfigTarget("vless://:443"); err == nil {
		t.Fatal("expected error for missing host")
	}
	if _, _, err := ParseConfigTarget("vmess://{invalid-json"); err == nil {
		t.Fatal("expected error for invalid vmess json payload")
	}

	// Unescaped or invalid query/fragment handling
	draft, err := ParseLinkConfiguration("vless://uuid@example.com:443#%FF%FE%FD?serverDescription=base64:c2VydmVy")
	if err != nil {
		t.Fatalf("unexpected error parsing malformed UTF8 fragment: %v", err)
	}
	if draft.Server != "example.com" {
		t.Fatalf("server = %q, want 'example.com'", draft.Server)
	}
}

func TestBase64BoundaryCases(t *testing.T) {
	// Test padding missing
	decoded := DecodeBase64String("base64:SGVsbG8gV29ybGQ") // missing '='
	if decoded != "Hello World" {
		t.Fatalf("decoded = %q, want 'Hello World'", decoded)
	}

	// Test URL-safe base64 (- and _)
	decodedURL := DecodeBase64String("base64:SGVsbG8tV29ybGQ_")
	if decodedURL == "" {
		t.Fatal("failed to decode URL-safe base64 string")
	}
}

func TestCheckConfigurationAvailabilityWithResolver(t *testing.T) {
	mockResolver := func(ctx context.Context, host string) ([]netip.Addr, error) {
		if host == "forbidden.local" {
			return nil, context.Canceled
		}
		return []netip.Addr{netip.MustParseAddr("127.0.0.1")}, nil
	}

	// Hy2/TUIC returns unknown / dns_resolved_udp_quic_probe_unsupported
	status, detail, _ := CheckConfigurationAvailabilityWithResolver("hy2://pass@hy2.example.com:443", mockResolver)
	if status != "unknown" || detail != "dns_resolved_udp_quic_probe_unsupported" {
		t.Fatalf("hy2 probe = (%q, %q), want (unknown, dns_resolved_udp_quic_probe_unsupported)", status, detail)
	}

	// Forbidden host
	status, detail, _ = CheckConfigurationAvailabilityWithResolver("vless://user@forbidden.local:443", mockResolver)
	if status != "down" || !strings.Contains(detail, "permitted") {
		t.Fatalf("forbidden host probe = (%q, %q), want down with permitted message", status, detail)
	}
}
