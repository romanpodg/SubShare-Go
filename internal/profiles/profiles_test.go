package profiles

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

var testFingerprintKey = []byte("subshare-profile-test-fingerprint-key")

func TestValidProfileFixtures(t *testing.T) {
	t.Parallel()
	type fixture struct {
		Name        string   `json:"name"`
		URI         string   `json:"uri"`
		Protocol    Protocol `json:"protocol"`
		Server      string   `json:"server"`
		Port        string   `json:"port"`
		DisplayName string   `json:"display_name"`
	}
	raw, err := os.ReadFile("testdata/valid_profiles.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []fixture
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, item := range fixtures {
		item := item
		t.Run(item.Name, func(t *testing.T) {
			t.Parallel()
			profile, err := Parse(item.URI)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if profile.Protocol != item.Protocol || profile.Server != item.Server || profile.Port.Expression != item.Port || profile.DisplayName != item.DisplayName {
				t.Fatalf("profile = %#v", profile.SafeMetadata())
			}
			if err := Validate(profile); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestShadowsocksSIP002UserInfoAndSpecialPasswords(t *testing.T) {
	t.Parallel()
	specialPassword := "p@ss:/+=% with spaces 密码"
	credential := "chacha20-ietf-poly1305:" + specialPassword
	base64Padded := base64.URLEncoding.EncodeToString([]byte(credential))
	base64Raw := base64.RawURLEncoding.EncodeToString([]byte(credential))
	plain := percentEncode("chacha20-ietf-poly1305") + ":" + percentEncode(specialPassword)

	tests := []struct {
		name string
		uri  string
	}{
		{"base64url padded", "ss://" + base64Padded + "@192.0.2.10:8388#IPv4"},
		{"base64url no padding", "ss://" + base64Raw + "@ss.example:8388#Domain"},
		{"plain percent encoded", "ss://" + plain + "@[2001:db8::10]:8388#IPv6"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			profile, err := Parse(test.uri)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			data := profile.Data.(ShadowsocksData)
			if data.Method != "chacha20-ietf-poly1305" || data.Password.Reveal() != specialPassword {
				t.Fatalf("credential was not preserved: method=%q password=%q", data.Method, data.Password.Reveal())
			}
		})
	}
}

func TestShadowsocksPluginLegacyAndAEAD2022(t *testing.T) {
	t.Parallel()
	credential := "aes-256-gcm:legacy/password+value="
	legacy := "ss://" + base64.StdEncoding.EncodeToString([]byte(credential+"@[2001:db8::20]:443")) +
		"?plugin=v2ray-plugin%3Bmode%3Dwebsocket%3Bhost%3Dcdn.example#Legacy"
	legacyProfile, err := Parse(legacy)
	if err != nil {
		t.Fatalf("parse legacy: %v", err)
	}
	legacyData := legacyProfile.Data.(ShadowsocksData)
	if !legacyData.LegacyInput || legacyData.UserInfoStyle != ShadowsocksUserInfoLegacy || legacyData.Plugin == nil || legacyData.Plugin.Options.Reveal() != "mode=websocket;host=cdn.example" {
		t.Fatalf("legacy/plugin data = %#v", legacyData)
	}
	if !hasWarning(legacyProfile, WarningLegacyInput) {
		t.Fatal("legacy input warning is missing")
	}

	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	uri := "ss://2022-blake3-aes-256-gcm:" + percentEncode(key) + "@ss2022.example:443#AEAD-2022"
	profile, err := Parse(uri)
	if err != nil {
		t.Fatalf("parse AEAD-2022: %v", err)
	}
	serialized, err := Serialize(profile, CanonicalSerialization)
	if err != nil {
		t.Fatalf("serialize AEAD-2022: %v", err)
	}
	if !strings.HasPrefix(serialized.URI.Reveal(), "ss://2022-blake3-aes-256-gcm:") {
		t.Fatalf("AEAD-2022 was rewritten as encoded userinfo: %s", serialized.URI.Reveal())
	}
	if serialized.Exact {
		t.Fatal("canonical serialization incorrectly claimed exact source output")
	}
}

func TestHysteria2AliasesDefaultsPortExpressionsAndExtensions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		uri  string
		port string
		host string
	}{
		{"alias default port", "hy2://auth%40value@HY.EXAMPLE/?mystery=one&mystery=two#Alias", "443", "hy.example"},
		{"port set and ranges", "hysteria2://auth@example.com:123,5000-6000,7044,8000-9000", "123,5000-6000,7044,8000-9000", "example.com"},
		{"ipv6", "hysteria2://auth@[2001:db8::30]:8443", "8443", "2001:db8::30"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			profile, err := Parse(test.uri)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if profile.Port.Expression != test.port || profile.Server != test.host {
				t.Fatalf("safe metadata = %#v", profile.SafeMetadata())
			}
		})
	}

	pin := strings.Repeat("ab", 32)
	uri := "hysteria2://user%3Apass@hy.example:443,5000-6000/?sni=TLS.EXAMPLE&insecure=1&pinSHA256=" + pin + "&obfs=salamander&obfs-password=obfs%20secret&x-ext=a&x-ext=b#Node"
	profile, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	data := profile.Data.(Hysteria2Data)
	if data.Authentication.Reveal() != "user:pass" || data.SNI != "tls.example" || !data.Insecure || data.CertificateSHA256 != pin || data.ObfuscationType != "salamander" || data.ObfuscationPassword.Reveal() != "obfs secret" {
		t.Fatalf("Hysteria data = %#v", data)
	}
	if len(profile.UnknownQueryParameters) != 2 || !hasWarning(profile, WarningAmbiguousParameter) {
		t.Fatalf("unknown/duplicate preservation failed: unknown=%d warnings=%v", len(profile.UnknownQueryParameters), profile.Warnings)
	}
}

func TestTUICV5AliasesAndCurrentClientFields(t *testing.T) {
	t.Parallel()
	uri := "tuic://33333333-3333-4333-8333-333333333333:p%40ss%3Aword%2F%2B%3D%25%20%E5%AF%86%E7%A0%81@TUIC.EXAMPLE:10443?" +
		"server_name=TLS.EXAMPLE&alpn=h3,hq-29&skip_cert_verify=true&disable_sni=false&" +
		"congestion_control=new_reno&udp_relay_mode=quic&zero_rtt_handshake=1&" +
		"heartbeat-interval=10000&request-timeout=8000&fast_open=1&max_open_streams=20&" +
		"max_udp_relay_packet_size=1500&extension=first&extension=second#TUIC"
	profile, err := Parse(uri)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	data := profile.Data.(TUICData)
	if data.Generation != 5 || data.Password.Reveal() != "p@ss:word/+=% 密码" || data.SNI != "tls.example" || strings.Join(data.ALPN, ",") != "h3,hq-29" || !data.SkipCertificateVerification || data.CongestionController != "new_reno" || data.UDPRelayMode != "quic" || !data.ZeroRTT || data.Heartbeat != "10s" || data.RequestTimeout != "8s" || !data.FastOpen || data.MaxOpenStreams != 20 || data.MaxUDPRelayPacketSize != 1500 {
		t.Fatalf("TUIC data = %#v", data)
	}
	if len(profile.UnknownQueryParameters) != 2 || !hasWarning(profile, WarningAmbiguousParameter) {
		t.Fatalf("unknown/duplicate preservation failed: unknown=%d warnings=%v", len(profile.UnknownQueryParameters), profile.Warnings)
	}
}

func TestTUICQueryCredentialDialectAndV4Compatibility(t *testing.T) {
	t.Parallel()
	v5, err := Parse("tuic://tuic.example:443?uuid=33333333-3333-4333-8333-333333333333&password=query%20password")
	if err != nil {
		t.Fatalf("query credential dialect: %v", err)
	}
	if data := v5.Data.(TUICData); data.Generation != 5 || data.Password.Reveal() != "query password" {
		t.Fatalf("unexpected v5 data: %#v", data)
	}

	v4, err := Parse("tuic://legacy-token-value@tuic.example:443?sni=edge.example#Legacy")
	if err != nil {
		t.Fatalf("v4 compatibility parse: %v", err)
	}
	data := v4.Data.(TUICData)
	if data.Generation != 4 || data.Token.Reveal() != "legacy-token-value" || v4.Capabilities.Status != CapabilityReadOnly || v4.Capabilities.Generate || v4.Capabilities.CanonicalSerialize || !hasWarning(v4, WarningCompatibilityOnly) {
		t.Fatalf("unexpected v4 capability profile: data=%#v capability=%#v", data, v4.Capabilities)
	}
	original, err := Serialize(v4, OriginalSerialization)
	if err != nil || !original.Exact || original.URI.Reveal() != "tuic://legacy-token-value@tuic.example:443?sni=edge.example#Legacy" {
		t.Fatalf("original v4 serialization = %#v err=%v", original, err)
	}
	if _, err := Serialize(v4, CanonicalSerialization); ErrorCodeOf(err) != ErrorCompatibilityOnlyInput {
		t.Fatalf("canonical v4 error = %v", err)
	}

	ambiguous := "tuic://legacy-token@tuic.example:443?token=other&uuid=33333333-3333-4333-8333-333333333333&password=secret"
	if _, err := Parse(ambiguous); ErrorCodeOf(err) != ErrorAmbiguousTUICDialect {
		t.Fatalf("ambiguous dialect error = %v", err)
	}
	duplicateCredential := "tuic://tuic.example:443?uuid=33333333-3333-4333-8333-333333333333&uuid=44444444-4444-4444-8444-444444444444&password=secret"
	if _, err := Parse(duplicateCredential); ErrorCodeOf(err) != ErrorAmbiguousTUICDialect {
		t.Fatalf("conflicting duplicate credential error = %v", err)
	}
}

func TestOriginalAndCanonicalSerializationBoundaries(t *testing.T) {
	t.Parallel()
	raw := "  hy2://auth@Example.COM/?z=last&unknown=%2Fvalue&unknown=%2fvalue#Display%20Name  "
	profile, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	original, err := Serialize(profile, OriginalSerialization)
	if err != nil || !original.Exact || original.URI.Reveal() != raw {
		t.Fatalf("original = %#v err=%v", original, err)
	}
	canonical, err := Serialize(profile, CanonicalSerialization)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.Exact || !canonical.SemanticallyStable || !strings.HasPrefix(canonical.URI.Reveal(), "hysteria2://auth@example.com:443/") || strings.Count(canonical.URI.Reveal(), "unknown=") != 2 {
		t.Fatalf("canonical = %#v", canonical)
	}
	if profile.OriginalURI.Reveal() != raw {
		t.Fatal("canonical serialization mutated the original URI")
	}
}

func TestCanonicalSerializationKeepsSelectedKnownDuplicateFirst(t *testing.T) {
	t.Parallel()
	profile, err := Parse("hy2://auth@example.com?sni=first.example&sni=second.example")
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := Serialize(profile, CanonicalSerialization)
	if err != nil {
		t.Fatal(err)
	}
	reparsed, err := Parse(serialized.URI.Reveal())
	if err != nil {
		t.Fatal(err)
	}
	if got := reparsed.Data.(Hysteria2Data).SNI; got != "first.example" {
		t.Fatalf("selected duplicate changed after canonical serialization: %q", got)
	}
	if strings.Count(serialized.URI.Reveal(), "sni=") != 2 {
		t.Fatalf("duplicate was not preserved: %s", serialized.URI.Reveal())
	}
}

func TestParseSerializeParseSemanticStability(t *testing.T) {
	t.Parallel()
	uris := []string{
		"vless://id@example.com?b=2&a=1#one",
		"vmess://eyJ2IjoiMiIsInBzIjoiVk1lc3MgZml4dHVyZSIsImFkZCI6IlZNRVNTLkVYQU1QTEUuQ09NIiwicG9ydCI6IjQ0MyIsImlkIjoiMjIyMjIyMjItMjIyMi00MjIyLTgyMjItMjIyMjIyMjIyMjIyIiwibmV0Ijoid3MiLCJwYXRoIjoiL3NvY2tldCIsInRscyI6InRscyJ9",
		"trojan://p%40ss@example.com:443?security=tls#two",
		"ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:password")) + "@ss.example:8388/?plugin=obfs-local%3Bobfs%3Dhttp&ext=1#three",
		"hysteria2://auth@example.com:443,5000-6000/?sni=tls.example&insecure=1&extra=a#four",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?congestion_control=bbr&zero_rtt_handshake=1&extra=a#five",
	}
	for _, uri := range uris {
		uri := uri
		t.Run(strings.SplitN(uri, ":", 2)[0], func(t *testing.T) {
			t.Parallel()
			first, err := Parse(uri)
			if err != nil {
				t.Fatal(err)
			}
			firstFingerprint, err := Fingerprint(first, testFingerprintKey)
			if err != nil {
				t.Fatal(err)
			}
			serialized, err := Serialize(first, CanonicalSerialization)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Parse(serialized.URI.Reveal())
			if err != nil {
				t.Fatalf("reparse canonical URI: %v", err)
			}
			secondFingerprint, err := Fingerprint(second, testFingerprintKey)
			if err != nil {
				t.Fatal(err)
			}
			if firstFingerprint != secondFingerprint {
				t.Fatalf("fingerprints differ: %s != %s", firstFingerprint, secondFingerprint)
			}
		})
	}
}

func TestFingerprintDeterminismNormalizationAndCredentialSeparation(t *testing.T) {
	t.Parallel()
	left, err := Parse("hysteria2://secret@EXAMPLE.COM/?z=2&a=1&dup=x&dup=y#Left")
	if err != nil {
		t.Fatal(err)
	}
	right, err := Parse("hy2://secret@example.com:443/?a=1&dup=x&z=2&dup=y#Right")
	if err != nil {
		t.Fatal(err)
	}
	leftFingerprint, _ := Fingerprint(left, testFingerprintKey)
	rightFingerprint, _ := Fingerprint(right, testFingerprintKey)
	if leftFingerprint != rightFingerprint {
		t.Fatalf("equivalent profiles differ: %s != %s", leftFingerprint, rightFingerprint)
	}
	repeated, _ := Fingerprint(left, testFingerprintKey)
	if repeated != leftFingerprint {
		t.Fatal("fingerprint is not deterministic")
	}
	different, err := Parse("hysteria2://different@example.com:443/?a=1&dup=x&dup=y&z=2#Left")
	if err != nil {
		t.Fatal(err)
	}
	differentFingerprint, _ := Fingerprint(different, testFingerprintKey)
	if differentFingerprint == leftFingerprint {
		t.Fatal("different credentials produced the same fingerprint")
	}
	if _, err := Fingerprint(left, nil); ErrorCodeOf(err) != ErrorMissingFingerprintKey {
		t.Fatalf("missing key error = %v", err)
	}
}

func TestMalformedProfilesAndPortBounds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		uri  string
		code ErrorCode
	}{
		{"unsupported scheme", "hysteria://secret@example.com:443", ErrorUnsupportedScheme},
		{"missing slashes", "hy2:secret@example.com:443", ErrorInvalidAuthority},
		{"missing host", "hy2://secret@:443", ErrorInvalidAuthority},
		{"empty explicit port", "hy2://secret@example.com:", ErrorInvalidPort},
		{"backslash host", "hy2://secret@example\\invalid:443", ErrorInvalidAuthority},
		{"missing auth", "hy2://example.com:443", ErrorMissingCredential},
		{"invalid percent", "hy2://bad%ZZ@example.com:443", ErrorInvalidEncoding},
		{"port zero", "hy2://secret@example.com:0", ErrorInvalidPort},
		{"port too high", "hy2://secret@example.com:65536", ErrorInvalidPort},
		{"bad range", "hy2://secret@example.com:6000-5000", ErrorInvalidPort},
		{"bad list", "hy2://secret@example.com:443,,8443", ErrorInvalidPort},
		{"bad pin", "hy2://secret@example.com:443?pinSHA256=deadbeef", ErrorInvalidProfile},
		{"unbracketed ipv6", "hy2://secret@2001:db8::1:443", ErrorInvalidAuthority},
		{"missing ss password", "ss://aes-256-gcm:@example.com:8388", ErrorMissingCredential},
		{"encoded ss2022", "ss://" + base64.RawURLEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:AAAAAAAAAAAAAAAAAAAAAA==")) + "@example.com:443", ErrorInvalidEncoding},
		{"tuic missing port", "tuic://33333333-3333-4333-8333-333333333333:password@example.com", ErrorInvalidPort},
		{"tuic bad uuid", "tuic://not-a-uuid:password@example.com:443", ErrorInvalidProfile},
		{"tuic missing password", "tuic://33333333-3333-4333-8333-333333333333:@example.com:443", ErrorMissingCredential},
		{"tuic unsupported generation", "tuic://token@example.com:443?udp_relay_mode=invalid", ErrorInvalidProfile},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(test.uri)
			if ErrorCodeOf(err) != test.code {
				t.Fatalf("error = %v, code = %q, want %q", err, ErrorCodeOf(err), test.code)
			}
		})
	}
}

func TestSecretsAreRedactedFromFormattingJSONSafeMetadataAndErrors(t *testing.T) {
	t.Parallel()
	secret := "never-log-this-secret-密码"
	profile, err := Parse("hysteria2://" + percentEncode(secret) + "@example.com:443?unknown=" + percentEncode(secret) + "#Safe%20name")
	if err != nil {
		t.Fatal(err)
	}
	for name, rendered := range map[string]string{
		"string":        fmt.Sprint(profile.OriginalURI),
		"go-syntax":     fmt.Sprintf("%#v", profile.OriginalURI),
		"query":         fmt.Sprint(profile.UnknownQueryParameters[0].Value),
		"whole-profile": fmt.Sprintf("%#v", *profile),
	} {
		if strings.Contains(rendered, secret) || strings.Contains(rendered, "hysteria2://") {
			t.Fatalf("%s exposed secret material: %q", name, rendered)
		}
	}
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "hysteria2://") {
		t.Fatalf("JSON exposed secret material: %s", encoded)
	}
	safe := profile.SafeMetadata()
	if safe.Server != "example.com" || safe.DisplayName != "Safe name" {
		t.Fatalf("safe metadata = %#v", safe)
	}

	badInputs := []string{
		"hy2://" + percentEncode(secret) + "@example.com:99999",
		"tuic://33333333-3333-4333-8333-333333333333:" + percentEncode(secret) + "@example.com:443?token=" + percentEncode(secret),
		"ss://aes-256-gcm:" + percentEncode(secret) + "@example.com:0",
	}
	for _, input := range badInputs {
		_, parseErr := Parse(input)
		if parseErr == nil {
			t.Fatalf("expected error for malformed secret input")
		}
		if strings.Contains(parseErr.Error(), secret) || strings.Contains(parseErr.Error(), input) {
			t.Fatalf("error exposed secret: %v", parseErr)
		}
	}
}

func TestUnsupportedTUICGenerationError(t *testing.T) {
	t.Parallel()
	profile := &Profile{
		Protocol: ProtocolTUIC,
		Server:   "example.com",
		Port:     PortSpec{Expression: "443", Kind: PortSingle, Ranges: []PortRange{{Start: 443, End: 443}}},
		Data: TUICData{
			Generation:           6,
			CongestionController: "cubic",
			UDPRelayMode:         "native",
		},
	}
	if err := Validate(profile); ErrorCodeOf(err) != ErrorUnsupportedGeneration {
		t.Fatalf("unsupported generation error = %v", err)
	}
}

func TestRegistryConcurrentReads(t *testing.T) {
	t.Parallel()
	const workers = 32
	var wait sync.WaitGroup
	errors := make(chan error, workers)
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			profile, err := Parse("hy2://auth@example.com?b=2&a=1")
			if err != nil {
				errors <- err
				return
			}
			if _, err := Fingerprint(profile, testFingerprintKey); err != nil {
				errors <- err
			}
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func hasWarning(profile *Profile, code string) bool {
	for _, warning := range profile.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
