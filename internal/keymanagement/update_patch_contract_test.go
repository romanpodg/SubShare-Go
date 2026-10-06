package keymanagement

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

const patchSS = "ss://aes-256-gcm:old-password@edge.example:443?plugin=v2ray-plugin%3Btls&x-extra=first&x-extra=second#Original"
const patchHY2 = "hysteria2://old-auth@edge.example:443?sni=old.example&insecure=1&pinSHA256=" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + "&obfs=salamander&obfs-password=old-obfs&x-extra=first&x-extra=second#Original"
const patchTUIC = "tuic://11111111-1111-4111-8111-111111111111:old-password@edge.example:443?sni=old.example&alpn=h3&skip-cert-verify=1&congestion-controller=cubic&udp-relay-mode=native&zero-rtt=1&heartbeat=10s&x-extra=first&x-extra=second#Original"

type patchFieldContract struct {
	protocol, raw, group, field, query string
	before, setValue, after, cleared   string
	clearError                         bool
}

func TestStructuredPatchTriStateFieldContracts(t *testing.T) {
	fields := []patchFieldContract{
		{"shadowsocks", patchSS, "shadowsocks", "method", "method", "aes-256-gcm", `"chacha20-ietf-poly1305"`, "chacha20-ietf-poly1305", "aes-256-gcm", false},
		{"shadowsocks", patchSS, "shadowsocks", "password", "password", "old-password", `"new-password"`, "new-password", "", true},
		{"shadowsocks", patchSS, "shadowsocks", "plugin_name", "plugin", "v2ray-plugin;tls", `"obfs-local"`, "obfs-local;tls", "", false},
		{"shadowsocks", patchSS, "shadowsocks", "plugin_options", "plugin", "v2ray-plugin;tls", `"mode=websocket"`, "v2ray-plugin;mode=websocket", "v2ray-plugin", false},
		{"hysteria2", patchHY2, "hysteria2", "authentication", "authentication", "old-auth", `"new-auth"`, "new-auth", "", true},
		{"hysteria2", patchHY2, "hysteria2", "sni", "sni", "old.example", `"new.example"`, "new.example", "", false},
		{"hysteria2", patchHY2, "hysteria2", "insecure", "insecure", "1", `false`, "0", "0", false},
		{"hysteria2", patchHY2, "hysteria2", "certificate_sha256", "pinSHA256", strings.Repeat("a", 64), `"` + strings.Repeat("b", 64) + `"`, strings.Repeat("b", 64), "", false},
		{"hysteria2", patchHY2, "hysteria2", "obfuscation_type", "obfs", "salamander", `"gecko"`, "gecko", "", false},
		{"hysteria2", patchHY2, "hysteria2", "obfuscation_password", "obfs-password", "old-obfs", `"new-obfs"`, "new-obfs", "", true},
		{"tuic", patchTUIC, "tuic", "uuid", "uuid", "11111111-1111-4111-8111-111111111111", `"22222222-2222-4222-8222-222222222222"`, "22222222-2222-4222-8222-222222222222", "", true},
		{"tuic", patchTUIC, "tuic", "password", "password", "old-password", `"new-password"`, "new-password", "", true},
		{"tuic", patchTUIC, "tuic", "sni", "sni", "old.example", `"new.example"`, "new.example", "", false},
		{"tuic", patchTUIC, "tuic", "alpn", "alpn", "h3", `["h3","h2"]`, "h3,h2", "", false},
		{"tuic", patchTUIC, "tuic", "skip_cert_verify", "skip-cert-verify", "1", `false`, "0", "0", false},
		{"tuic", patchTUIC, "tuic", "congestion_controller", "congestion-controller", "cubic", `"bbr"`, "bbr", "", true},
		{"tuic", patchTUIC, "tuic", "udp_relay_mode", "udp-relay-mode", "native", `"quic"`, "quic", "", true},
		{"tuic", strings.Replace(patchTUIC, "udp-relay-mode=native&", "", 1), "tuic", "udp_over_stream", "udp-over-stream", "", `true`, "1", "", false},
		{"tuic", patchTUIC, "tuic", "zero_rtt", "zero-rtt", "1", `false`, "0", "0", false},
		{"tuic", patchTUIC, "tuic", "heartbeat", "heartbeat", "10s", `"20s"`, "20s", "", false},
	}
	for _, field := range fields {
		t.Run(field.protocol+"/"+field.field, func(t *testing.T) {
			for _, operation := range []string{"absent", "omitted", "set", "clear"} {
				t.Run(operation, func(t *testing.T) { assertPatchFieldContract(t, field, operation) })
			}
		})
	}
}

func decodeContractPatch(t *testing.T, raw string) *model.StructuredProfilePatch {
	t.Helper()
	var patch model.StructuredProfilePatch
	if err := json.Unmarshal([]byte(raw), &patch); err != nil {
		t.Fatal(err)
	}
	return &patch
}

func assertPatchFieldContract(t *testing.T, field patchFieldContract, operation string) {
	t.Helper()
	payload, want := patchContractPayload(field, operation)
	got, err := ApplyStructuredPatchToURI(field.protocol, field.raw, "Panel", decodeContractPatch(t, payload))
	if operation == "clear" && field.clearError {
		updateContractEqual(t, "clear returns error", err != nil, true)
		updateContractEqual(t, "no URI on clear error", got, "")
		return
	}
	updateContractError(t, err, nil)
	if operation == "absent" {
		updateContractEqual(t, "empty patch raw bytes", got, field.raw)
	}
	u, err := url.Parse(got)
	updateContractError(t, err, nil)
	updateContractEqual(t, field.field, patchContractValue(t, u, got, field.query), want)
	updateContractEqual(t, "unknown duplicate parameters", u.Query()["x-extra"], []string{"first", "second"})
	updateContractEqual(t, "unchanged embedded label", u.Fragment, "Original")
	updateContractEqual(t, "unchanged endpoint", u.Host, "edge.example:443")
	if operation == "clear" && field.field == "plugin_name" {
		updateContractEqual(t, "plugin query removed", u.Query().Has("plugin"), false)
	}
}

func patchContractPayload(field patchFieldContract, operation string) (string, string) {
	switch operation {
	case "absent":
		return `{}`, field.before
	case "omitted":
		return fmt.Sprintf(`{"%s":{}}`, field.group), field.before
	case "set":
		return fmt.Sprintf(`{"%s":{"%s":{"operation":"set","value":%s}}}`, field.group, field.field, field.setValue), field.after
	default:
		return fmt.Sprintf(`{"%s":{"%s":{"operation":"clear"}}}`, field.group, field.field), field.cleared
	}
}

func patchContractValue(t *testing.T, u *url.URL, raw, field string) string {
	t.Helper()
	switch field {
	case "password", "authentication", "uuid", "method":
		return contractCredentialField(t, raw, field)
	default:
		return u.Query().Get(field)
	}
}

func contractCredentialField(t *testing.T, raw, field string) string {
	t.Helper()
	p, err := profiles.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	switch data := p.Data.(type) {
	case profiles.ShadowsocksData:
		if field == "method" {
			return data.Method
		}
		return data.Password.Reveal()
	case profiles.Hysteria2Data:
		return data.Authentication.Reveal()
	case profiles.TUICData:
		if field == "uuid" {
			return data.UUID.Reveal()
		}
		return data.Password.Reveal()
	}
	t.Fatal("unexpected protocol")
	return ""
}

func TestStructuredPatchCommonFields(t *testing.T) {
	for _, tc := range []struct {
		name, patch, host, fragment string
	}{
		{"server set", `{"server":{"operation":"set","value":"new.example"}}`, "new.example:443", "Original"},
		{"port set", `{"port":{"operation":"set","value":"8443"}}`, "edge.example:8443", "Original"},
		{"display set", `{"display_name":{"operation":"set","value":"New name"}}`, "edge.example:443", "New name"},
		// These common-field clears currently keep the existing value.
		{"server clear", `{"server":{"operation":"clear"}}`, "edge.example:443", "Original"},
		{"port clear", `{"port":{"operation":"clear"}}`, "edge.example:443", "Original"},
		{"display clear", `{"display_name":{"operation":"clear"}}`, "edge.example:443", "Original"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ApplyStructuredPatchToURI("shadowsocks", patchSS, "Panel", decodeContractPatch(t, tc.patch))
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(got)
			updateContractError(t, err, nil)
			updateContractEqual(t, "common endpoint", u.Host, tc.host)
			updateContractEqual(t, "common display name", u.Fragment, tc.fragment)
		})
	}
}

func TestStructuredPatchMalformedStoredURI(t *testing.T) {
	const malformed = "legacy bytes that are not a URI"
	got, err := ApplyStructuredPatchToURI("hysteria2", malformed, "Panel", decodeContractPatch(t, `{}`))
	updateContractError(t, err, nil)
	updateContractEqual(t, "empty patch preserves malformed bytes", got, malformed)
	_, err = ApplyStructuredPatchToURI("hysteria2", malformed, "Panel", decodeContractPatch(t, `{"display_name":{"operation":"set","value":"New"}}`))
	if err == nil {
		t.Fatal("partial patch cannot rebuild malformed storage without an endpoint")
	}
	replacement := decodeContractPatch(t, `{"server":{"operation":"set","value":"new.example"},"port":{"operation":"set","value":"443"},"hysteria2":{"authentication":{"operation":"set","value":"new-auth"}}}`)
	got, err = ApplyStructuredPatchToURI("hysteria2", malformed, "Panel", replacement)
	updateContractError(t, err, nil)
	updateContractEqual(t, "complete malformed-URI rebuild", got, "hysteria2://new-auth@new.example:443#Panel")
}

// Characterization: optional Hy2 clears currently serialize empty known query
// values. SNI/obfs then fail parsing. R08 must not silently fix that behavior.
func TestStructuredPatchOptionalClearCanProduceUnparseableURI(t *testing.T) {
	for _, field := range []string{"sni", "obfuscation_type"} {
		t.Run(field, func(t *testing.T) {
			patch := decodeContractPatch(t, fmt.Sprintf(`{"hysteria2":{"%s":{"operation":"clear"}}}`, field))
			got, err := ApplyStructuredPatchToURI("hysteria2", patchHY2, "Panel", patch)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := profiles.Parse(got); err == nil {
				t.Fatal("characterized empty known parameter unexpectedly parsed")
			}
		})
	}
}

func TestStructuredPatchPluginClearRemovesAllCaseVariants(t *testing.T) {
	raw := strings.Replace(patchSS, "&x-extra=first", "&PLUGIN=obfs-local%3Bobfs%3Dhttp&x-extra=first", 1)
	patch := decodeContractPatch(t, `{"shadowsocks":{"plugin_name":{"operation":"clear"},"plugin_options":{"operation":"set","value":"ignored"}}}`)
	got, err := ApplyStructuredPatchToURI("shadowsocks", raw, "Panel", patch)
	updateContractError(t, err, nil)
	u, err := url.Parse(got)
	updateContractError(t, err, nil)
	for key := range u.Query() {
		updateContractEqual(t, "plugin case variants removed", strings.EqualFold(key, "plugin"), false)
	}
	updateContractEqual(t, "plugin clear retains unknown duplicates", u.Query()["x-extra"], []string{"first", "second"})
	updateContractEqual(t, "plugin clear retains password", contractCredentialField(t, got, "password"), "old-password")
}

func TestStructuredPatchSetEmptyRequiredSecretsFails(t *testing.T) {
	for _, tc := range []struct{ protocol, raw, group, field string }{
		{"shadowsocks", patchSS, "shadowsocks", "password"},
		{"hysteria2", patchHY2, "hysteria2", "authentication"},
		{"tuic", patchTUIC, "tuic", "uuid"},
		{"tuic", patchTUIC, "tuic", "password"},
	} {
		t.Run(tc.protocol+"/"+tc.field, func(t *testing.T) {
			patch := decodeContractPatch(t, fmt.Sprintf(`{"%s":{"%s":{"operation":"set","value":""}}}`, tc.group, tc.field))
			got, err := ApplyStructuredPatchToURI(tc.protocol, tc.raw, "Panel", patch)
			updateContractEqual(t, "empty required secret rejected", err != nil, true)
			updateContractEqual(t, "no URI on required secret failure", got, "")
		})
	}
}
