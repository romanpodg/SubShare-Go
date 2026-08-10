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

func TestEffectiveClientDisplayNameSeparatesSourceAndLocalFallbacks(t *testing.T) {
	for _, tag := range []string{"proxy", "direct", "block", "dns", "freedom", "blackhole", "outbound"} {
		raw := `{"outbounds":[{"tag":"` + tag + `","protocol":"trojan","settings":{"servers":[{"address":"example.com","port":443,"password":"secret"}]}}]}`
		if got := EffectiveClientDisplayName("", raw, "Human name", true); got != "Human name" {
			t.Fatalf("source tag %q replaced synchronized label: %q", tag, got)
		}
	}

	rawJSON := `{"outbounds":[{"tag":"proxy","protocol":"trojan","settings":{"servers":[{"address":"example.com","port":443,"password":"secret"}]}}]}`
	if got := EffectiveClientDisplayName("proxy", rawJSON, "Human name", true); got != "proxy" {
		t.Fatalf("genuine explicit override was discarded: %q", got)
	}
	if got := EffectiveClientDisplayName("", "vless://uuid@example.com:443#Embedded%20URI", "Panel", false); got != "Embedded URI" {
		t.Fatalf("local URI fragment fallback changed: %q", got)
	}
	vmessPayload := base64.StdEncoding.EncodeToString([]byte(`{"add":"vm.example","port":"443","id":"uuid","ps":"Embedded VMess"}`))
	if got := EffectiveClientDisplayName("", "vmess://"+vmessPayload, "Panel", false); got != "Embedded VMess" {
		t.Fatalf("local VMess ps fallback changed: %q", got)
	}
}

func TestProjectXrayJSONDraftsSupportsProxyOutboundsAndSkipsHelpers(t *testing.T) {
	raw := `{
		"dns":{"servers":["1.1.1.1"]},
		"routing":{"rules":[{"outboundTag":"direct"}]},
		"outbounds":[
			{"tag":"VLESS","protocol":"vless","settings":{"vnext":[{"address":"vless.example","port":443,"users":[{"id":"11111111-1111-4111-8111-111111111111","encryption":"none"}]}]},"streamSettings":{"network":"ws","security":"reality","wsSettings":{"path":"/ws","headers":{"Host":"cdn.example"}},"realitySettings":{"serverName":"sni.example","publicKey":"public-key","shortId":"abcd","fingerprint":"chrome"}}},
			{"tag":"VMess","protocol":"vmess","settings":{"vnext":[{"address":"vmess.example","port":8443,"users":[{"id":"22222222-2222-4222-8222-222222222222","security":"auto"}]}]}},
			{"tag":"Trojan","protocol":"trojan","settings":{"servers":[{"address":"trojan.example","port":443,"password":"trojan-password"}]}},
			{"tag":"SS","protocol":"shadowsocks","settings":{"address":"ss.example","port":8388,"method":"aes-256-gcm","password":"ss-password","plugin":"v2ray-plugin","plugin_opts":"mode=websocket;host=cdn.example"}},
			{"tag":"HY2","protocol":"hysteria","settings":{"version":2,"address":"hy.example","port":443},"streamSettings":{"security":"tls","hysteriaSettings":{"version":2,"auth":"hy-auth"},"tlsSettings":{"serverName":"hy-sni.example"}}},
			{"tag":"TUIC","protocol":"tuic","settings":{"address":"tuic.example","port":443,"uuid":"33333333-3333-4333-8333-333333333333","password":"tuic-password","sni":"tuic-sni.example","alpn":["h3"],"congestion_control":"bbr"}},
			{"tag":"direct","protocol":"freedom","settings":{}},
			{"tag":"block","protocol":"blackhole","settings":{}},
			{"tag":"broken","protocol":"trojan","settings":{"servers":[]}}
		]
	}`
	drafts, rejected, err := ProjectXrayJSONDrafts(raw)
	if err != nil {
		t.Fatalf("project XRAY-JSON: %v", err)
	}
	if len(drafts) != 6 || rejected != 1 {
		t.Fatalf("drafts=%d rejected=%d", len(drafts), rejected)
	}
	wantProtocols := []string{"vless", "vmess", "trojan", "shadowsocks", "hysteria2", "tuic"}
	for index, draft := range drafts {
		if draft.Protocol != wantProtocols[index] {
			t.Fatalf("draft %d protocol=%q", index, draft.Protocol)
		}
		link, buildErr := BuildShareLinkFromDraft(draft)
		if buildErr != nil {
			t.Fatalf("build %s link: %v", draft.Protocol, buildErr)
		}
		if SupportedConfigScheme(link) == "xray-json" || strings.Contains(link, `"outbounds"`) {
			t.Fatalf("raw JSON reached projected link: %q", link)
		}
	}
	if drafts[0].Network != "ws" || drafts[0].Security != "reality" || drafts[0].Path != "/ws" || drafts[0].Host != "cdn.example" || drafts[0].PublicKey != "public-key" {
		t.Fatalf("VLESS transport projection lost representable fields: %#v", drafts[0])
	}
	if drafts[3].Plugin != "v2ray-plugin;mode=websocket;host=cdn.example" {
		t.Fatalf("Shadowsocks plugin projection lost options: %#v", drafts[3])
	}
}

func TestProjectXrayJSONDraftsRejectsAmbiguousHysteria2Masks(t *testing.T) {
	raw := `{"outbounds":[{"protocol":"hysteria","settings":{"version":2,"address":"hy.example","port":443},"streamSettings":{"hysteriaSettings":{"version":2,"auth":"auth"},"finalmask":{"udp":[{"type":"salamander","settings":{"password":"one"}},{"type":"salamander","settings":{"password":"two"}}]}}}]}`
	drafts, rejected, err := ProjectXrayJSONDrafts(raw)
	if err != nil {
		t.Fatalf("project XRAY-JSON: %v", err)
	}
	if len(drafts) != 0 || rejected != 1 {
		t.Fatalf("drafts=%d rejected=%d, want no malformed projection and one rejection", len(drafts), rejected)
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
