package main

import (
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"testing"
)

func TestExternalProfileSemanticDeduplication(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		duplicates int
		ambiguous  int
		keys       int
	}{
		{
			name: "tag and query order",
			body: "hy2://auth@example.com?sni=tls.example&x=1#one\n" +
				"hysteria2://auth@example.com:443?x=1&sni=tls.example#two",
			duplicates: 1, keys: 1,
		},
		{
			name:       "omitted versus explicit default port",
			body:       "hy2://auth@example.com#one\nhy2://auth@example.com:443#two",
			duplicates: 1, keys: 1,
		},
		{
			name: "same-key duplicate ordering remains distinct",
			body: "hy2://auth@example.com?sni=first.example&sni=second.example\n" +
				"hy2://auth@example.com?sni=second.example&sni=first.example",
			ambiguous: 2, keys: 2,
		},
		{
			name: "unknown duplicate ordering remains distinct",
			body: "hy2://auth@example.com?extension=first&extension=second\n" +
				"hy2://auth@example.com?extension=second&extension=first",
			ambiguous: 2, keys: 2,
		},
		{
			name: "different-key ordering with duplicate order preserved",
			body: "hy2://auth@example.com?extension=first&extension=second&other=value\n" +
				"hy2://auth@example.com?other=value&extension=first&extension=second",
			duplicates: 1, ambiguous: 2, keys: 1,
		},
		{
			name: "conflicting duplicate does not collapse into unambiguous",
			body: "hy2://auth@example.com?sni=first.example\n" +
				"hy2://auth@example.com?sni=first.example&sni=second.example",
			ambiguous: 1, keys: 2,
		},
		{
			name: "TUIC aliases normalize",
			body: "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?congestion-controller=bbr&udp-relay-mode=quic\n" +
				"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?udp_relay_mode=quic&congestion_control=bbr",
			duplicates: 1, keys: 1,
		},
		{
			name: "different credentials",
			body: "hy2://first@example.com\nhy2://second@example.com",
			keys: 2,
		},
		{
			name: "TUIC v4 and v5",
			body: externalTestTUICV4 + "\n" + externalTestTUIC,
			keys: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := parseExternalTestBody(t, test.body)
			if len(parsed.Keys) != test.keys || parsed.Counts.Duplicate != test.duplicates || parsed.Counts.Ambiguous != test.ambiguous {
				t.Fatalf("keys=%d counts=%#v items=%#v", len(parsed.Keys), parsed.Counts, parsed.Items)
			}
		})
	}
}

func TestExternalProfileConnectivityChangesRemainDistinct(t *testing.T) {
	tests := []string{
		"hy2://auth@example.com?sni=one.example\nhy2://auth@example.com?sni=two.example",
		"hy2://auth@example.com?obfs=salamander&obfs-password=one\nhy2://auth@example.com?obfs=salamander&obfs-password=two",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?skip-cert-verify=0\n" +
			"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?skip-cert-verify=1",
		"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?udp-relay-mode=native\n" +
			"tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?udp-relay-mode=quic",
	}
	for _, body := range tests {
		parsed := parseExternalTestBody(t, body)
		if len(parsed.Keys) != 2 || parsed.Counts.Duplicate != 0 || parsed.Keys[0].Fingerprint == parsed.Keys[1].Fingerprint {
			t.Fatalf("connectivity-changing profiles collapsed: %#v", parsed)
		}
	}
	contradictory := parseExternalTestBody(t, "tuic://33333333-3333-4333-8333-333333333333:password@example.com:443?sni=first.example&server_name=second.example")
	if contradictory.Counts.Ambiguous != 1 || !sources.ContainsWarningCode(contradictory.Items[0].Warnings, profiles.WarningAmbiguousParameter) {
		t.Fatalf("contradictory TUIC aliases were not marked ambiguous: %#v", contradictory)
	}
}

func TestExternalProfileFingerprintKeyAndSelectionIntegrity(t *testing.T) {
	if _, err := sources.ParseBody(externalTestSS, nil); err == nil || err.Error() != "profile_fingerprint_key_unavailable" {
		t.Fatalf("nil fingerprint key error = %v", err)
	}
	parsed := parseExternalTestBody(t, externalTestSS+"\n"+externalTestHY2)
	selected, err := sources.FilterSelection(parsed, []string{parsed.Items[1].ItemRef})
	if err != nil || len(selected.Keys) != 1 || selected.Keys[0].Protocol != "hysteria2" {
		t.Fatalf("selection result=%#v err=%v", selected, err)
	}
	for _, refs := range [][]string{{"ir1_tampered"}, {parsed.Items[0].ItemRef, parsed.Items[0].ItemRef}} {
		if _, err := sources.FilterSelection(parsed, refs); err == nil {
			t.Fatalf("tampered selection %#v was accepted", refs)
		}
	}

	identical := parseExternalTestBody(t, externalTestSS+"\n"+externalTestSS)
	if len(identical.Keys) != 1 || len(identical.Items) != 2 || identical.Items[1].Status != sources.StatusDuplicate {
		t.Fatalf("identical-line duplicate result = %#v", identical)
	}
	if identical.Items[0].ItemRef == identical.Items[1].ItemRef {
		t.Fatalf("identical source lines share item reference %q", identical.Items[0].ItemRef)
	}
	if _, err := sources.FilterSelection(identical, []string{identical.Items[1].ItemRef}); err == nil {
		t.Fatal("duplicate-only preview occurrence was accepted for persistence")
	}
}
