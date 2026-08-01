package profiles

import (
	"fmt"
	"strings"
	"testing"
)

const fuzzSecretMarker = "FUZZ_SECRET_DO_NOT_ECHO"

func FuzzRegistryParse(f *testing.F) {
	for _, seed := range fuzzSeeds() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		fuzzParseAndSerialize(t, raw, nil)
	})
}

func FuzzShadowsocksParse(f *testing.F) {
	for _, seed := range []string{
		"ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@ss.example:8388#valid",
		"ss://aes-256-gcm:p%40ss%3Aword@example.com:8388?plugin=local%3Bkey%3Dvalue",
		"ss://%%%%@example.com:8388",
		"ss://aes-256-gcm:" + fuzzSecretMarker + "@example.com:0",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		fuzzParseAndSerialize(t, raw, shadowsocksAdapter{})
	})
}

func FuzzHysteria2Parse(f *testing.F) {
	for _, seed := range []string{
		"hy2://auth@example.com",
		"hysteria2://auth@[2001:db8::1]:443,5000-6000?sni=tls.example&x=1&x=2",
		"hy2://" + fuzzSecretMarker + "@example.com:65536",
		"hysteria://auth@example.com:443",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		fuzzParseAndSerialize(t, raw, hysteria2Adapter{})
	})
}

func FuzzTUICParse(f *testing.F) {
	for _, seed := range []string{
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?congestion_control=bbr",
		"tuic://legacy-token@example.com:443",
		"tuic://33333333-3333-4333-8333-333333333333:" + fuzzSecretMarker + "@example.com:70000",
		"tuic://not-a-uuid:password@example.com:443",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		fuzzParseAndSerialize(t, raw, tuicAdapter{})
	})
}

func fuzzSeeds() []string {
	return []string{
		"vless://11111111-1111-4111-8111-111111111111@example.com:443?type=ws",
		"trojan://password@example.com:443",
		"ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@ss.example:8388",
		"hy2://auth@example.com:443,5000-6000?unknown=one&unknown=two",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?zero_rtt_handshake=1",
		"missing-scheme",
		"hy2://" + fuzzSecretMarker + "@example.com:0",
	}
}

func fuzzParseAndSerialize(t *testing.T, raw string, adapter Adapter) {
	t.Helper()
	// Keep each fuzz execution bounded. The public registry separately rejects
	// inputs above maxProfileURIBytes.
	if len(raw) > 8*1024 {
		t.Skip()
	}
	parse := Parse
	if adapter != nil {
		parse = adapter.Parse
	}
	first, firstErr := parse(raw)
	second, secondErr := parse(raw)
	if (firstErr == nil) != (secondErr == nil) {
		t.Fatal("repeated parsing was nondeterministic")
	}
	if firstErr != nil {
		if firstErr.Error() != secondErr.Error() {
			t.Fatal("repeated errors were nondeterministic")
		}
		if strings.Contains(raw, fuzzSecretMarker) && strings.Contains(firstErr.Error(), fuzzSecretMarker) {
			t.Fatal("secret marker copied into parse error")
		}
		return
	}
	if err := Validate(first); err != nil {
		t.Fatalf("successful parse failed validation: %v", err)
	}
	assertFuzzFormattingSafe(t, raw, first)
	if !first.Capabilities.CanonicalSerialize {
		original, err := Serialize(first, OriginalSerialization)
		if err != nil || !original.Exact || original.URI.Reveal() != raw {
			t.Fatalf("compatibility original failed: %#v %v", original, err)
		}
		return
	}
	canonicalOne, err := Serialize(first, CanonicalSerialization)
	if err != nil {
		t.Fatalf("canonical serialization: %v", err)
	}
	canonicalTwo, err := Serialize(second, CanonicalSerialization)
	if err != nil || canonicalOne.URI.Reveal() != canonicalTwo.URI.Reveal() {
		t.Fatalf("canonical serialization was nondeterministic: %v", err)
	}
	if canonicalOne.Exact {
		t.Fatal("canonical serialization claimed exact output")
	}
	maximumOutput := len(raw)*4 + 4096
	if len(canonicalOne.URI.Reveal()) > maximumOutput {
		t.Fatalf("small input expanded unexpectedly: input=%d output=%d", len(raw), len(canonicalOne.URI.Reveal()))
	}
	reparsed, err := Parse(canonicalOne.URI.Reveal())
	if err != nil {
		t.Fatalf("canonical output could not be reparsed: %v", err)
	}
	canonicalThree, err := Serialize(reparsed, CanonicalSerialization)
	if err != nil || canonicalThree.URI.Reveal() != canonicalOne.URI.Reveal() {
		t.Fatalf("canonical serialization was not idempotent: %v", err)
	}
	key := []byte("bounded-fuzz-fingerprint-key")
	firstFingerprint, err := Fingerprint(first, key)
	if err != nil {
		t.Fatal(err)
	}
	reparsedFingerprint, err := Fingerprint(reparsed, key)
	if err != nil || firstFingerprint != reparsedFingerprint {
		t.Fatalf("canonical reparse changed fingerprint: %v", err)
	}
}

func assertFuzzFormattingSafe(t *testing.T, raw string, profile *Profile) {
	t.Helper()
	if !strings.Contains(raw, fuzzSecretMarker) {
		return
	}
	for _, rendered := range []string{
		fmt.Sprint(profile),
		fmt.Sprintf("%+v", profile),
		fmt.Sprintf("%#v", profile),
		fmt.Sprint(profile.OriginalURI),
	} {
		if strings.Contains(rendered, fuzzSecretMarker) {
			t.Fatal("secret marker copied into formatted output")
		}
	}
}
