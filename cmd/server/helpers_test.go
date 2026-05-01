package main

import (
	"encoding/json"
	"testing"
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
