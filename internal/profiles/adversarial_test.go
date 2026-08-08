package profiles

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"testing"
)

func TestDuplicateParameterPolicy(t *testing.T) {
	t.Parallel()
	key := []byte("duplicate-policy-key")
	tests := []struct {
		name       string
		raw        string
		canonical  string
		selected   func(*Profile) string
		unambigous string
		reversed   string
	}{
		{
			name:       "Hysteria known",
			raw:        "hy2://auth@example.com?sni=first.example&sni=second.example",
			canonical:  "sni",
			selected:   func(profile *Profile) string { return profile.Data.(Hysteria2Data).SNI },
			unambigous: "hy2://auth@example.com?sni=first.example",
			reversed:   "hy2://auth@example.com?sni=second.example&sni=first.example",
		},
		{
			name:       "TUIC contradictory aliases",
			raw:        "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=first.example&server_name=second.example",
			canonical:  "sni",
			selected:   func(profile *Profile) string { return profile.Data.(TUICData).SNI },
			unambigous: "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=first.example",
			reversed:   "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?server_name=second.example&sni=first.example",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			profile, err := Parse(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := test.selected(profile); got != "first.example" {
				t.Fatalf("structured selection = %q, want first value", got)
			}
			if !hasWarning(profile, WarningAmbiguousParameter) {
				t.Fatalf("warnings = %#v", profile.Warnings)
			}
			if len(profile.QueryParameters) != 2 || profile.QueryParameters[0].Value.Reveal() != "first.example" || profile.QueryParameters[1].Value.Reveal() != "second.example" {
				t.Fatalf("source-order values were not retained")
			}
			original, err := Serialize(profile, OriginalSerialization)
			if err != nil || original.URI.Reveal() != test.raw || !original.Exact {
				t.Fatalf("original=%#v err=%v", original, err)
			}
			canonical, err := Serialize(profile, CanonicalSerialization)
			if err != nil {
				t.Fatal(err)
			}
			if canonical.Exact || strings.Count(canonical.URI.Reveal(), test.canonical+"=") != 2 {
				t.Fatalf("canonical=%q exact=%v", canonical.URI.Reveal(), canonical.Exact)
			}
			assertDifferentFingerprint(t, key, test.raw, test.unambigous)
			assertDifferentFingerprint(t, key, test.raw, test.reversed)
		})
	}
}

func TestUnknownDuplicateOrderAndParameterGroupOrdering(t *testing.T) {
	t.Parallel()
	key := []byte("unknown-duplicate-key")
	for _, prefix := range []string{
		"hy2://auth@example.com?",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?",
	} {
		left, err := Parse(prefix + "z=9&extension=first&a=1&extension=second")
		if err != nil {
			t.Fatal(err)
		}
		if !hasWarning(left, WarningAmbiguousParameter) || len(left.UnknownQueryParameters) != 4 {
			t.Fatalf("unknown duplicates not surfaced: %#v", left.SafeMetadata())
		}
		canonical, err := Serialize(left, CanonicalSerialization)
		if err != nil {
			t.Fatal(err)
		}
		firstIndex := strings.Index(canonical.URI.Reveal(), "extension=first")
		secondIndex := strings.Index(canonical.URI.Reveal(), "extension=second")
		if firstIndex < 0 || secondIndex <= firstIndex {
			t.Fatalf("duplicate order changed: %q", canonical.URI.Reveal())
		}
		reordered, _ := Parse(prefix + "a=1&extension=first&extension=second&z=9")
		leftFingerprint, _ := Fingerprint(left, key)
		reorderedFingerprint, _ := Fingerprint(reordered, key)
		if leftFingerprint != reorderedFingerprint {
			t.Fatal("ordering between distinct query keys changed the fingerprint")
		}
		assertDifferentFingerprint(t, key,
			prefix+"extension=first&extension=second",
			prefix+"extension=second&extension=first",
		)
	}
}

func TestExplicitDefaultKnownDuplicateRemainsSelected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		raw      string
		first    string
		second   string
		selected func(*Profile) bool
	}{
		{
			name:     "Hysteria insecure false before true",
			raw:      "hy2://auth@example.com?insecure=0&insecure=1",
			first:    "insecure=0",
			second:   "insecure=1",
			selected: func(profile *Profile) bool { return !profile.Data.(Hysteria2Data).Insecure },
		},
		{
			name:     "TUIC certificate verification default before override",
			raw:      "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?skip_cert_verify=false&skip-cert-verify=true",
			first:    "skip-cert-verify=0",
			second:   "skip-cert-verify=true",
			selected: func(profile *Profile) bool { return !profile.Data.(TUICData).SkipCertificateVerification },
		},
		{
			name:     "TUIC congestion default before override",
			raw:      "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?congestion_control=cubic&congestion-controller=bbr",
			first:    "congestion-controller=cubic",
			second:   "congestion-controller=bbr",
			selected: func(profile *Profile) bool { return profile.Data.(TUICData).CongestionController == "cubic" },
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			profile, err := Parse(test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if !test.selected(profile) || !hasWarning(profile, WarningAmbiguousParameter) {
				t.Fatalf("selected/warnings = %v %#v", test.selected(profile), profile.Warnings)
			}
			before, _ := Fingerprint(profile, testFingerprintKey)
			serialized, err := Serialize(profile, CanonicalSerialization)
			if err != nil {
				t.Fatal(err)
			}
			firstIndex := strings.Index(serialized.URI.Reveal(), test.first)
			secondIndex := strings.Index(serialized.URI.Reveal(), test.second)
			if firstIndex < 0 || secondIndex <= firstIndex {
				t.Fatalf("selected default was not retained first: %q", serialized.URI.Reveal())
			}
			reparsed, err := Parse(serialized.URI.Reveal())
			if err != nil {
				t.Fatal(err)
			}
			if !test.selected(reparsed) {
				t.Fatal("canonical reparse promoted a conflicting duplicate")
			}
			after, _ := Fingerprint(reparsed, testFingerprintKey)
			if before != after {
				t.Fatal("canonical reparse changed duplicate fingerprint")
			}
		})
	}
}

func TestConnectivityFieldsParticipateInFingerprints(t *testing.T) {
	t.Parallel()
	key := []byte("connectivity-field-key")
	pinA := strings.Repeat("aa", 32)
	pinB := strings.Repeat("bb", 32)
	hysteriaBase := "hy2://auth@example.com:443?sni=tls.example&insecure=1&pinSHA256=" + pinA + "&obfs=salamander&obfs-password=obfs&extension=one"
	for _, changed := range []string{
		"hy2://other@example.com:443?sni=tls.example&insecure=1&pinSHA256=" + pinA + "&obfs=salamander&obfs-password=obfs&extension=one",
		"hy2://auth@example.com:443?sni=other.example&insecure=1&pinSHA256=" + pinA + "&obfs=salamander&obfs-password=obfs&extension=one",
		"hy2://auth@example.com:443?sni=tls.example&insecure=0&pinSHA256=" + pinA + "&obfs=salamander&obfs-password=obfs&extension=one",
		"hy2://auth@example.com:443?sni=tls.example&insecure=1&pinSHA256=" + pinB + "&obfs=salamander&obfs-password=obfs&extension=one",
		"hy2://auth@example.com:443?sni=tls.example&insecure=1&pinSHA256=" + pinA + "&obfs=gecko&obfs-password=obfs&extension=one",
		"hy2://auth@example.com:443?sni=tls.example&insecure=1&pinSHA256=" + pinA + "&obfs=salamander&obfs-password=other&extension=one",
		"hy2://auth@example.com:443?sni=tls.example&insecure=1&pinSHA256=" + pinA + "&obfs=salamander&obfs-password=obfs&extension=two",
	} {
		assertDifferentFingerprint(t, key, hysteriaBase, changed)
	}

	ssBase := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@ss.example:8388?plugin=plugin%3Bcredential%3Done"
	ssChanged := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@ss.example:8388?plugin=plugin%3Bcredential%3Dtwo"
	assertDifferentFingerprint(t, key, ssBase, ssChanged)

	tuicBase := "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=tls.example&skip-cert-verify=1&congestion-controller=bbr&udp-relay-mode=quic&zero-rtt=1&extension=one"
	for _, changed := range []string{
		"tuic://33333333-3333-4333-8333-333333333333:other@example.com:443?sni=tls.example&skip-cert-verify=1&congestion-controller=bbr&udp-relay-mode=quic&zero-rtt=1&extension=one",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=other.example&skip-cert-verify=1&congestion-controller=bbr&udp-relay-mode=quic&zero-rtt=1&extension=one",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=tls.example&skip-cert-verify=0&congestion-controller=bbr&udp-relay-mode=quic&zero-rtt=1&extension=one",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=tls.example&skip-cert-verify=1&congestion-controller=cubic&udp-relay-mode=quic&zero-rtt=1&extension=one",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=tls.example&skip-cert-verify=1&congestion-controller=bbr&udp-relay-mode=native&zero-rtt=1&extension=one",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=tls.example&skip-cert-verify=1&congestion-controller=bbr&udp-relay-mode=quic&zero-rtt=0&extension=one",
	} {
		assertDifferentFingerprint(t, key, tuicBase, changed)
	}
}

func TestSecretRedactionAcrossFormattingAndSerialization(t *testing.T) {
	t.Parallel()
	secret := "SECRET_password_token_psk_plugin_DO_NOT_LOG"
	uuid := "77777777-7777-4777-8777-777777777777"
	raw := "tuic://" + uuid + ":" + secret + "@example.com:443?unknown=" + secret + "#Safe"
	profile, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := Serialize(profile, CanonicalSerialization)
	if err != nil {
		t.Fatal(err)
	}
	fingerprintInput, err := CanonicalFingerprintInput(profile)
	if err != nil {
		t.Fatal(err)
	}
	data := profile.Data.(TUICData)
	values := []any{profile.OriginalURI, profile, *profile, data, canonical, canonical.URI, fingerprintInput, profile.QueryParameters, profile.UnknownQueryParameters}
	verbs := []string{"%s", "%v", "%+v", "%#v"}
	for _, value := range values {
		for _, verb := range verbs {
			assertNoSecret(t, fmt.Sprintf(verb, value), secret, uuid, raw)
		}
		assertNoSecret(t, fmt.Sprint(value), secret, uuid, raw)
	}
	var logged bytes.Buffer
	log.New(&logged, "profile ", log.LstdFlags).Printf("%+v %#v", profile, canonical)
	assertNoSecret(t, logged.String(), secret, uuid, raw)

	for _, value := range []any{profile, profile.SafeMetadata(), data, canonical, fingerprintInput} {
		encoded, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		assertNoSecret(t, string(encoded), secret, uuid, raw)
	}
	encodedProfile, _ := json.Marshal(profile)
	if strings.Contains(string(encodedProfile), "OriginalURI") || strings.Contains(string(encodedProfile), "QueryParameters") || strings.Contains(string(encodedProfile), "Data") {
		t.Fatalf("full internal profile leaked into JSON: %s", encodedProfile)
	}

	bad := "tuic://" + uuid + ":" + secret + "@example.com:70000?unknown=" + secret
	_, parseErr := Parse(bad)
	if parseErr == nil {
		t.Fatal("expected malformed URI error")
	}
	wrapped := fmt.Errorf("validation failed: %w", parseErr)
	for _, rendered := range []string{parseErr.Error(), fmt.Sprintf("%s", parseErr), fmt.Sprintf("%v", wrapped), fmt.Sprintf("%+v", wrapped), fmt.Sprintf("%#v", wrapped)} {
		assertNoSecret(t, rendered, secret, uuid, bad)
	}

	ss, err := Parse("ss://aes-256-gcm:" + percentEncode(secret) + "@example.com:8388?plugin=local%3Bpsk%3D" + secret)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, fmt.Sprintf("%s %v %+v %#v", ss, ss, ss, ss), secret)
}

func TestEverySecretBearingAggregateRedacts(t *testing.T) {
	t.Parallel()
	secret := "AGGREGATE_SECRET_DO_NOT_LOG"
	vmessPayload := `{"add":"example.com","port":"443","id":"` + secret + `","ps":"safe"}`
	inputs := []string{
		"vless://" + secret + "@example.com:443",
		"trojan://" + secret + "@example.com:443",
		"vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessPayload)),
		"ss://aes-256-gcm:" + secret + "@example.com:8388?plugin=local%3Bpsk%3D" + secret,
		"hy2://" + secret + "@example.com:443?obfs=salamander&obfs-password=" + secret,
		"tuic://33333333-3333-4333-8333-333333333333:" + secret + "@example.com:443?" + secret + "=value",
		"tuic://" + secret + "@example.com:443",
	}
	for _, raw := range inputs {
		profile, err := Parse(raw)
		if err != nil {
			t.Fatalf("parse %s profile: %v", strings.SplitN(raw, ":", 2)[0], err)
		}
		values := []any{profile.Data, profile.QueryParameters, profile.UnknownQueryParameters}
		if data, ok := profile.Data.(ShadowsocksData); ok && data.Plugin != nil {
			values = append(values, *data.Plugin)
		}
		if data, ok := profile.Data.(TUICData); ok {
			values = append(values, data.FieldObservations)
		}
		for _, value := range values {
			for _, verb := range []string{"%s", "%v", "%+v", "%#v"} {
				assertNoSecret(t, fmt.Sprintf(verb, value), secret)
			}
			assertNoSecret(t, fmt.Sprint(value), secret)
			encoded, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			assertNoSecret(t, string(encoded), secret)
		}
	}
}

func TestShadowsocksDialectSeparation(t *testing.T) {
	t.Parallel()
	method := "aes-256-gcm"
	password := "colon:@/+=% spaces Unicode 密码"
	credential := method + ":" + password
	modern := []string{
		"ss://" + base64.URLEncoding.EncodeToString([]byte(credential)) + "@192.0.2.1:8388#padded",
		"ss://" + base64.RawURLEncoding.EncodeToString([]byte(credential)) + "@[2001:db8::1]:8388#raw",
		"ss://" + percentEncode(method) + ":" + percentEncode(password) + "@ss.example:8388#plain",
	}
	for _, raw := range modern {
		profile, err := Parse(raw)
		if err != nil {
			t.Fatalf("modern parse %q: %v", raw, err)
		}
		data := profile.Data.(ShadowsocksData)
		if data.Password.Reveal() != password || data.LegacyInput {
			t.Fatalf("modern dialect misclassified: %#v", data)
		}
	}

	legacyCredential := credential + "@ss.example:8388"
	for name, encoded := range map[string]string{
		"standard padded":   base64.StdEncoding.EncodeToString([]byte(legacyCredential)),
		"standard unpadded": base64.RawStdEncoding.EncodeToString([]byte(legacyCredential)),
		"url safe padded":   base64.URLEncoding.EncodeToString([]byte(legacyCredential)),
		"url safe raw":      base64.RawURLEncoding.EncodeToString([]byte(legacyCredential)),
	} {
		t.Run(name, func(t *testing.T) {
			profile, err := Parse("ss://" + encoded + "#legacy")
			if err != nil {
				t.Fatal(err)
			}
			if data := profile.Data.(ShadowsocksData); !data.LegacyInput || data.Password.Reveal() != password || !hasWarning(profile, WarningLegacyInput) {
				t.Fatalf("legacy classification = %#v", data)
			}
		})
	}

	standardOnlyCredential := ""
	standardOnly := ""
	for char := rune(0x80); char < 0x2000; char++ {
		candidate := method + ":" + string(char)
		encoded := base64.StdEncoding.EncodeToString([]byte(candidate))
		if strings.ContainsAny(encoded, "+/") {
			standardOnlyCredential = candidate
			standardOnly = encoded
			break
		}
	}
	if standardOnly == "" {
		t.Fatal("test could not produce standard-only base64")
	}
	if _, err := Parse("ss://" + standardOnly + "@example.com:8388"); ErrorCodeOf(err) != ErrorInvalidEncoding {
		t.Fatalf("modern standard alphabet accepted for %q: %v", standardOnlyCredential, err)
	}

	for _, raw := range []string{"ss://YWVzLTI1Ni1nY206@x:8388", "ss://YWVzLTI1Ni1nY20@x:8388", "ss://%%%%@x:8388", "ss://YWJjZA"} {
		if _, err := Parse(raw); err == nil {
			t.Fatalf("malformed Base64-looking input accepted: %q", raw)
		}
	}

	psk := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	aeadURI := "ss://2022-blake3-aes-128-gcm:" + percentEncode(psk) + "@example.com:443"
	aead, err := Parse(aeadURI)
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := Serialize(aead, CanonicalSerialization)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(serialized.URI.Reveal(), "ss://2022-blake3-aes-128-gcm:") {
		t.Fatalf("AEAD-2022 was encoded incompatibly: %q", serialized.URI.Reveal())
	}
	encodedAEAD := base64.RawURLEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:" + psk))
	if _, err := Parse("ss://" + encodedAEAD + "@example.com:443"); ErrorCodeOf(err) != ErrorInvalidEncoding {
		t.Fatalf("encoded AEAD-2022 error = %v", err)
	}
}

func TestHysteriaPortExpressionsAndExtensions(t *testing.T) {
	t.Parallel()
	implicit, err := Parse("hy2://auth@example.com")
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := Parse("hy2://auth@example.com:443")
	if err != nil {
		t.Fatal(err)
	}
	if implicit.Port.Expression != "443" || implicit.Port.Explicit || !explicit.Port.Explicit {
		t.Fatalf("implicit=%#v explicit=%#v", implicit.Port, explicit.Port)
	}
	implicitFingerprint, _ := Fingerprint(implicit, testFingerprintKey)
	explicitFingerprint, _ := Fingerprint(explicit, testFingerprintKey)
	if implicitFingerprint != explicitFingerprint {
		t.Fatal("explicit default port changed connectivity fingerprint")
	}

	ordered, err := Parse("hy2://auth@example.com:443,5000-6000,7044,8000-9000")
	if err != nil {
		t.Fatal(err)
	}
	if ordered.Port.Kind != PortExpression || len(ordered.Port.Ranges) != 4 || ordered.Port.Ranges[1] != (PortRange{Start: 5000, End: 6000}) {
		t.Fatalf("port structure = %#v", ordered.Port)
	}
	serialized, _ := Serialize(ordered, CanonicalSerialization)
	if !strings.Contains(serialized.URI.Reveal(), ":443,5000-6000,7044,8000-9000") {
		t.Fatalf("port expression reordered: %q", serialized.URI.Reveal())
	}
	assertDifferentFingerprint(t, testFingerprintKey,
		"hy2://auth@example.com:443,8443",
		"hy2://auth@example.com:8443,443",
	)

	for _, expression := range []string{"0", "65536", "6000-5000", "0-10", "10-0", "1-65536", "", "-", "1-", "-2", "1--2", "1-2-3", "443,,8443", "443,", ",443", "443, 8443", " 443"} {
		raw := "hy2://auth@example.com:" + expression
		if _, err := Parse(raw); ErrorCodeOf(err) != ErrorInvalidPort {
			t.Fatalf("expression %q error = %v", expression, err)
		}
	}

	gecko, err := Parse("hy2://auth@example.com?obfs=gecko&obfs-password=secret&ech=ZXhhbXBsZQ%3D%3D&future=value")
	if err != nil {
		t.Fatal(err)
	}
	if !hasWarning(gecko, WarningClientVersionSensitive) || len(gecko.UnknownQueryParameters) != 2 {
		t.Fatalf("gecko/extensions = %#v unknown=%d", gecko.Warnings, len(gecko.UnknownQueryParameters))
	}
	canonical, _ := Serialize(gecko, CanonicalSerialization)
	if !strings.Contains(canonical.URI.Reveal(), "ech=") || !strings.Contains(canonical.URI.Reveal(), "future=value") {
		t.Fatalf("extensions dropped: %q", canonical.URI.Reveal())
	}
}

func TestTUICFieldProvenanceAndGenerationSeparation(t *testing.T) {
	t.Parallel()
	raw := "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?" +
		"sni=tls.example&server_name=tls.example&congestion-controller=bbr&udp_relay_mode=quic&" +
		"reduce-rtt=1&heartbeat=10s&vendor_field=value"
	profile, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	data := profile.Data.(TUICData)
	assertObservation(t, data, "userinfo.uuid", "uuid", TUICProvenanceProtocolNative, TUICProvenanceProtocolNative)
	assertObservation(t, data, "sni", "sni", TUICProvenanceCommonClient, TUICProvenanceCommonClient)
	assertObservation(t, data, "server_name", "sni", TUICProvenanceCommonClient, TUICProvenanceSingBox)
	assertObservation(t, data, "congestion-controller", "congestion-controller", TUICProvenanceCommonClient, TUICProvenanceMihomo)
	assertObservation(t, data, "udp_relay_mode", "udp-relay-mode", TUICProvenanceCommonClient, TUICProvenanceSingBox)
	assertObservation(t, data, "reduce-rtt", "zero-rtt", TUICProvenanceCommonClient, TUICProvenanceMihomo)
	assertObservation(t, data, "heartbeat", "heartbeat", TUICProvenanceCommonClient, TUICProvenanceSingBox)
	assertObservation(t, data, "vendor_field", "unknown", TUICProvenanceUnknown, TUICProvenanceUnknown)
	if !hasWarning(profile, WarningDuplicateParameter) || hasWarning(profile, WarningAmbiguousParameter) {
		t.Fatalf("equivalent alias warnings = %#v", profile.Warnings)
	}

	mihomo := "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?congestion-controller=bbr&udp-relay-mode=quic"
	singBox := "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?congestion_control=bbr&udp_relay_mode=quic"
	left, _ := Parse(mihomo)
	right, _ := Parse(singBox)
	leftFingerprint, _ := Fingerprint(left, testFingerprintKey)
	rightFingerprint, _ := Fingerprint(right, testFingerprintKey)
	if leftFingerprint != rightFingerprint {
		t.Fatal("unambiguous client aliases did not normalize semantically")
	}
	canonical, _ := Serialize(right, CanonicalSerialization)
	if !strings.Contains(canonical.URI.Reveal(), "congestion-controller=bbr") || !strings.Contains(canonical.URI.Reveal(), "udp-relay-mode=quic") {
		t.Fatalf("canonical aliases = %q", canonical.URI.Reveal())
	}
	if right.QueryParameters[0].Key != "congestion_control" || right.QueryParameters[0].Value.Reveal() != "bbr" {
		t.Fatal("original alias spelling/value was not retained")
	}

	contradictory, err := Parse("tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=first.example&server_name=second.example")
	if err != nil {
		t.Fatal(err)
	}
	if contradictory.Data.(TUICData).SNI != "first.example" || !hasWarning(contradictory, WarningAmbiguousParameter) {
		t.Fatalf("contradictory aliases = %#v", contradictory.SafeMetadata())
	}

	mihomoOnly, err := Parse("tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?disable-sni=1&max-open-streams=8")
	if err != nil {
		t.Fatal(err)
	}
	mihomoData := mihomoOnly.Data.(TUICData)
	assertObservation(t, mihomoData, "disable-sni", "disable-sni", TUICProvenanceMihomo, TUICProvenanceMihomo)
	assertObservation(t, mihomoData, "max-open-streams", "max-open-streams", TUICProvenanceMihomo, TUICProvenanceMihomo)
	singOnly, err := Parse("tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?udp_over_stream=1")
	if err != nil {
		t.Fatal(err)
	}
	assertObservation(t, singOnly.Data.(TUICData), "udp_over_stream", "udp-over-stream", TUICProvenanceSingBox, TUICProvenanceSingBox)

	for name, test := range map[string]struct {
		raw  string
		code ErrorCode
	}{
		"valid UUID missing password": {"tuic://33333333-3333-4333-8333-333333333333@example.com:443", ErrorMissingCredential},
		"malformed UUID-shaped":       {"tuic://33333333-3333-4333-8333-33333333333z@example.com:443", ErrorInvalidProfile},
		"malformed v5 pair":           {"tuic://not-a-uuid:password@example.com:443", ErrorInvalidProfile},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(test.raw); ErrorCodeOf(err) != test.code {
				t.Fatalf("error = %v", err)
			}
		})
	}

	v4Raw := "tuic://legacy-token@example.com:443?sni=edge.example#v4"
	v4, err := Parse(v4Raw)
	if err != nil {
		t.Fatal(err)
	}
	v4Data := v4.Data.(TUICData)
	if v4Data.Generation != 4 || !v4Data.Token.IsSet() || v4Data.Password.IsSet() || v4Data.UUID.IsSet() {
		t.Fatalf("v4 credential separation = %#v", v4Data)
	}
	assertObservation(t, v4Data, "userinfo.token", "token", TUICProvenanceV4, TUICProvenanceV4)
	if _, err := Serialize(v4, CanonicalSerialization); ErrorCodeOf(err) != ErrorCompatibilityOnlyInput {
		t.Fatalf("v4 canonical error = %v", err)
	}
	original, _ := Serialize(v4, OriginalSerialization)
	if !original.Exact || original.URI.Reveal() != v4Raw {
		t.Fatalf("v4 original = %#v", original)
	}
	explicitV4, err := Parse("tuic://example.com:443?token=33333333-3333-4333-8333-33333333333z")
	if err != nil || explicitV4.Data.(TUICData).Generation != 4 {
		t.Fatalf("explicit v4 token dialect = %#v err=%v", explicitV4, err)
	}
}

func TestSerializationContractsAndFingerprintKeys(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"  hy2://auth@EXAMPLE.com?sni=first.example&sni=second.example&x=%2f&x=%2F#Display%20Name  ",
		"tuic://33333333-3333-4333-8333-333333333333:password@EXAMPLE.com:443?congestion_control=bbr&extension=first&extension=second#Node",
	} {
		profile, err := Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		original, err := Serialize(profile, OriginalSerialization)
		if err != nil || !original.Exact || original.Mode != OriginalSerialization || original.URI.Reveal() != raw {
			t.Fatalf("original=%#v err=%v", original, err)
		}
		first, err := Serialize(profile, CanonicalSerialization)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Serialize(profile, CanonicalSerialization)
		if err != nil || first.URI.Reveal() != second.URI.Reveal() || first.Exact || first.Mode != CanonicalSerialization || !first.SemanticallyStable {
			t.Fatalf("canonical contract first=%#v second=%#v err=%v", first, second, err)
		}
		reparsed, err := Parse(first.URI.Reveal())
		if err != nil {
			t.Fatal(err)
		}
		third, err := Serialize(reparsed, CanonicalSerialization)
		if err != nil || third.URI.Reveal() != first.URI.Reveal() {
			t.Fatalf("canonical not idempotent: %q != %q err=%v", third.URI.Reveal(), first.URI.Reveal(), err)
		}
		beforeFingerprint, _ := Fingerprint(profile, testFingerprintKey)
		afterFingerprint, _ := Fingerprint(reparsed, testFingerprintKey)
		if beforeFingerprint != afterFingerprint {
			t.Fatal("canonical reparse changed semantic fingerprint")
		}
		normalized, err := Canonicalize(profile)
		if err != nil || normalized.Server != "example.com" || normalized.OriginalURI.Reveal() != raw {
			t.Fatalf("normalized profile = %#v err=%v", normalized.SafeMetadata(), err)
		}
	}

	profile, _ := Parse("hy2://auth@example.com")
	for _, key := range [][]byte{nil, {}} {
		if _, err := Fingerprint(profile, key); ErrorCodeOf(err) != ErrorMissingFingerprintKey {
			t.Fatalf("key %#v error = %v", key, err)
		}
	}
	first, _ := Fingerprint(profile, []byte("key-one"))
	second, _ := Fingerprint(profile, []byte("key-two"))
	if first == second {
		t.Fatal("different caller keys produced the same keyed digest")
	}
	if _, err := Parse("hy2://auth@example.com?x=" + strings.Repeat("a", maxProfileURIBytes)); ErrorCodeOf(err) != ErrorInvalidProfile {
		t.Fatalf("oversized input error = %v", err)
	}
}

func assertDifferentFingerprint(t *testing.T, key []byte, leftRaw, rightRaw string) {
	t.Helper()
	left, err := Parse(leftRaw)
	if err != nil {
		t.Fatalf("left parse: %v", err)
	}
	right, err := Parse(rightRaw)
	if err != nil {
		t.Fatalf("right parse: %v", err)
	}
	leftFingerprint, err := Fingerprint(left, key)
	if err != nil {
		t.Fatal(err)
	}
	rightFingerprint, err := Fingerprint(right, key)
	if err != nil {
		t.Fatal(err)
	}
	if leftFingerprint == rightFingerprint {
		t.Fatalf("fingerprints collapsed for %q and %q", leftRaw, rightRaw)
	}
}

func assertNoSecret(t *testing.T, rendered string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(rendered, secret) {
			t.Fatalf("rendered output exposed secret material: %q", rendered)
		}
	}
}

func assertObservation(t *testing.T, data TUICData, source, field string, fieldClass, sourceProvenance TUICFieldProvenance) {
	t.Helper()
	for _, observation := range data.FieldObservations {
		if observation.SourceName.Reveal() == source && observation.Field == field && observation.FieldClass == fieldClass && observation.SourceProvenance == sourceProvenance {
			return
		}
	}
	t.Fatalf("missing observation source=%q field=%q class=%q source_provenance=%q: %#v", source, field, fieldClass, sourceProvenance, data.FieldObservations)
}
