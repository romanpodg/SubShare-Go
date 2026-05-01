package main

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func firstOutbound(t *testing.T, raw string) map[string]any {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}
	outbounds, ok := parsed["outbounds"].([]any)
	if !ok || len(outbounds) == 0 {
		t.Fatalf("outbounds are missing")
	}
	outbound, ok := outbounds[0].(map[string]any)
	if !ok {
		t.Fatalf("first outbound is not an object")
	}
	return outbound
}

func TestNormalizeConfigurationForSubscriptionOutput_VLESS(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?type=ws&security=tls&path=%2Fws&host=cdn.example.com&sni=example.com&alpn=h2,http%2F1.1#My%20Server"
	converted, err := normalizeConfigurationForSubscriptionOutput(raw, "xray-json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outbound := firstOutbound(t, converted)
	if protocol, _ := outbound["protocol"].(string); protocol != "vless" {
		t.Fatalf("protocol = %v, want vless", outbound["protocol"])
	}
}

func TestNormalizeConfigurationForSubscriptionOutput_Trojan(t *testing.T) {
	raw := "trojan://my-password@example.com:443?security=tls&sni=example.com#Trojan"
	converted, err := normalizeConfigurationForSubscriptionOutput(raw, "xray-json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outbound := firstOutbound(t, converted)
	if protocol, _ := outbound["protocol"].(string); protocol != "trojan" {
		t.Fatalf("protocol = %v, want trojan", outbound["protocol"])
	}
}

func TestNormalizeConfigurationForSubscriptionOutput_VMESS(t *testing.T) {
	payload := `{"v":"2","ps":"VM","add":"vm.example.com","port":"443","id":"22222222-2222-2222-2222-222222222222","net":"ws","tls":"tls","path":"/vm","host":"cdn.vm.example.com","sni":"vm.example.com","alpn":"h2,http/1.1"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	raw := "vmess://" + encoded

	converted, err := normalizeConfigurationForSubscriptionOutput(raw, "xray-json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outbound := firstOutbound(t, converted)
	if protocol, _ := outbound["protocol"].(string); protocol != "vmess" {
		t.Fatalf("protocol = %v, want vmess", outbound["protocol"])
	}
}

func TestNormalizeConfigurationForSubscriptionOutput_BuildsFullClientTemplate(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?type=tcp&security=reality&pbk=pubKey123&sni=yahoo.com#Node"
	converted, err := normalizeConfigurationForSubscriptionOutput(raw, "xray-json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(converted), &parsed); err != nil {
		t.Fatalf("failed to parse converted config: %v", err)
	}

	if remarks, _ := parsed["remarks"].(string); remarks != "Node" {
		t.Fatalf("remarks = %q, want %q", remarks, "Node")
	}

	dns, _ := parsed["dns"].(map[string]any)
	servers, _ := dns["servers"].([]any)
	if len(servers) != 2 {
		t.Fatalf("dns.servers len = %d, want 2", len(servers))
	}

	inbounds, _ := parsed["inbounds"].([]any)
	if len(inbounds) != 2 {
		t.Fatalf("inbounds len = %d, want 2", len(inbounds))
	}

	outbounds, _ := parsed["outbounds"].([]any)
	if len(outbounds) != 3 {
		t.Fatalf("outbounds len = %d, want 3", len(outbounds))
	}
	direct, _ := outbounds[1].(map[string]any)
	if tag, _ := direct["tag"].(string); tag != "direct" {
		t.Fatalf("outbounds[1].tag = %q, want %q", tag, "direct")
	}
	block, _ := outbounds[2].(map[string]any)
	if tag, _ := block["tag"].(string); tag != "block" {
		t.Fatalf("outbounds[2].tag = %q, want %q", tag, "block")
	}

	outbound := firstOutbound(t, converted)
	streamSettings, _ := outbound["streamSettings"].(map[string]any)
	if _, ok := streamSettings["tcpSettings"].(map[string]any); !ok {
		t.Fatalf("expected tcpSettings object for tcp transport")
	}
	realitySettings, _ := streamSettings["realitySettings"].(map[string]any)
	showRaw, showExists := realitySettings["show"]
	showBool, showOk := showRaw.(bool)
	if !showExists || !showOk || showBool != false {
		t.Fatalf("expected realitySettings.show=false, got %#v", showRaw)
	}
	if publicKey, _ := realitySettings["publicKey"].(string); publicKey != "pubKey123" {
		t.Fatalf("publicKey = %q, want %q", publicKey, "pubKey123")
	}
}

func TestNormalizeConfigurationForSubscriptionOutput_PassthroughJSON(t *testing.T) {
	raw := ` { "outbounds": [{"protocol":"vless","settings":{"vnext":[{"address":"example.com","port":443,"users":[{"id":"33333333-3333-3333-3333-333333333333"}]}]}}], "inbounds": [] } `
	converted, err := normalizeConfigurationForSubscriptionOutput(raw, "xray-json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted == raw {
		t.Fatalf("expected trimmed output")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(converted), &parsed); err != nil {
		t.Fatalf("converted JSON is invalid: %v", err)
	}
}

func TestNormalizeConfigurationForSubscriptionOutput_KeepLinksMode(t *testing.T) {
	raw := " vless://11111111-1111-1111-1111-111111111111@example.com:443 "
	converted, err := normalizeConfigurationForSubscriptionOutput(raw, "links", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted != "vless://11111111-1111-1111-1111-111111111111@example.com:443" {
		t.Fatalf("unexpected converted output: %q", converted)
	}
}

func TestNormalizeConfigurationForSubscriptionOutput_Invalid(t *testing.T) {
	_, err := normalizeConfigurationForSubscriptionOutput("not-a-valid-config", "xray-json", "")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNormalizeConfigurationForSubscriptionOutput_RealityFieldsAndFallbackRemark(t *testing.T) {
	raw := "vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@edge.example.com:443?type=tcp&security=reality&flow=xtls-rprx-vision&fp=chrome&pbk=pubkey123&sid=abcd1234&spx=%2Fprobe&sni=www.microsoft.com"
	converted, err := normalizeConfigurationForSubscriptionOutput(raw, "xray-json", "Finland")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(converted), &parsed); err != nil {
		t.Fatalf("failed to unmarshal converted config: %v", err)
	}
	if remarks, _ := parsed["remarks"].(string); remarks != "Finland" {
		t.Fatalf("remarks = %q, want %q", remarks, "Finland")
	}

	outbound := firstOutbound(t, converted)
	settings, _ := outbound["settings"].(map[string]any)
	vnext, _ := settings["vnext"].([]any)
	node, _ := vnext[0].(map[string]any)
	users, _ := node["users"].([]any)
	user, _ := users[0].(map[string]any)
	if flow, _ := user["flow"].(string); flow != "xtls-rprx-vision" {
		t.Fatalf("flow = %q, want %q", flow, "xtls-rprx-vision")
	}

	streamSettings, _ := outbound["streamSettings"].(map[string]any)
	realitySettings, _ := streamSettings["realitySettings"].(map[string]any)
	if fingerprint, _ := realitySettings["fingerprint"].(string); fingerprint != "chrome" {
		t.Fatalf("fingerprint = %q, want %q", fingerprint, "chrome")
	}
	if publicKey, _ := realitySettings["publicKey"].(string); publicKey != "pubkey123" {
		t.Fatalf("publicKey = %q, want %q", publicKey, "pubkey123")
	}
	if password, _ := realitySettings["password"].(string); password != "pubkey123" {
		t.Fatalf("password = %q, want %q", password, "pubkey123")
	}
	if shortID, _ := realitySettings["shortId"].(string); shortID != "abcd1234" {
		t.Fatalf("shortId = %q, want %q", shortID, "abcd1234")
	}
	if spiderX, _ := realitySettings["spiderX"].(string); spiderX != "/probe" {
		t.Fatalf("spiderX = %q, want %q", spiderX, "/probe")
	}
}

func TestNormalizeConfigurationForSubscriptionOutput_RealityPublicKeyKeepsPlus(t *testing.T) {
	raw := "vless://aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa@edge.example.com:443?security=reality&pbk=AbCd+EfGh%2F123%3D%3D&sid=abcd1234&sni=www.microsoft.com#Reality"
	converted, err := normalizeConfigurationForSubscriptionOutput(raw, "xray-json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outbound := firstOutbound(t, converted)
	streamSettings, _ := outbound["streamSettings"].(map[string]any)
	realitySettings, _ := streamSettings["realitySettings"].(map[string]any)
	if publicKey, _ := realitySettings["publicKey"].(string); publicKey != "AbCd+EfGh/123==" {
		t.Fatalf("publicKey = %q, want %q", publicKey, "AbCd+EfGh/123==")
	}
	if password, _ := realitySettings["password"].(string); password != "AbCd+EfGh/123==" {
		t.Fatalf("password = %q, want %q", password, "AbCd+EfGh/123==")
	}
}

func TestNormalizeConfigurationForSubscriptionOutput_VmessSecurityAlterIDAndHeaderType(t *testing.T) {
	payload := `{"v":"2","ps":"VM","add":"vm.example.com","port":"443","id":"22222222-2222-2222-2222-222222222222","aid":"64","scy":"chacha20-poly1305","net":"tcp","type":"http","host":"cdn.vm.example.com,www.vm2.example.com","path":"/vm,/vm2","tls":"tls","sni":"vm.example.com","alpn":"h2,http/1.1","fp":"chrome"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	raw := "vmess://" + encoded

	converted, err := normalizeConfigurationForSubscriptionOutput(raw, "xray-json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outbound := firstOutbound(t, converted)
	settings, _ := outbound["settings"].(map[string]any)
	vnext, _ := settings["vnext"].([]any)
	node, _ := vnext[0].(map[string]any)
	users, _ := node["users"].([]any)
	user, _ := users[0].(map[string]any)
	if security, _ := user["security"].(string); security != "chacha20-poly1305" {
		t.Fatalf("security = %q, want %q", security, "chacha20-poly1305")
	}
	if alterID, _ := user["alterId"].(float64); int(alterID) != 64 {
		t.Fatalf("alterId = %v, want 64", user["alterId"])
	}

	streamSettings, _ := outbound["streamSettings"].(map[string]any)
	tcpSettings, _ := streamSettings["tcpSettings"].(map[string]any)
	header, _ := tcpSettings["header"].(map[string]any)
	if headerType, _ := header["type"].(string); headerType != "http" {
		t.Fatalf("tcp header type = %q, want %q", headerType, "http")
	}
	request, _ := header["request"].(map[string]any)
	paths, _ := request["path"].([]any)
	if len(paths) != 2 {
		t.Fatalf("expected 2 request paths, got %d", len(paths))
	}
	headers, _ := request["headers"].(map[string]any)
	hosts, _ := headers["Host"].([]any)
	if len(hosts) != 2 {
		t.Fatalf("expected 2 request hosts, got %d", len(hosts))
	}
}
