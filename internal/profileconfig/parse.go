package profileconfig

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func ClientDisplayNameFromKeyURL(rawURL, fallback string) string {
	rawURL = strings.TrimSpace(rawURL)
	fallback = strings.TrimSpace(fallback)
	if rawURL == "" {
		return fallback
	}
	if profile, err := profiles.Parse(rawURL); err == nil {
		if name := strings.TrimSpace(profile.DisplayName); name != "" {
			return name
		}
		return fallback
	}

	switch SupportedConfigScheme(rawURL) {
	case "vless", "vmess", "trojan":
		draft, err := ParseLinkConfiguration(rawURL)
		if err != nil {
			return fallback
		}
		name := strings.TrimSpace(firstNonEmpty(draft.Remark, draft.ServerDescription, fallback))
		if name != "" {
			return name
		}
	case "xray-json":
		drafts, err := ParseXrayJSONDrafts(rawURL)
		if err != nil || len(drafts) == 0 {
			return fallback
		}
		name := strings.TrimSpace(firstNonEmpty(drafts[0].Remark, drafts[0].ServerDescription, fallback))
		if name != "" {
			return name
		}
	}

	return fallback
}

// EffectiveClientDisplayName resolves subscriber-facing metadata without
// confusing source-owned XRAY routing tags with human-readable source names.
// A stored value is always an explicit administrator override. Source-owned
// rows otherwise follow their synchronized label; local rows retain legacy
// URI fragment and VMess ps fallback behavior.
func EffectiveClientDisplayName(storedOverride, rawURL, label string, sourceOwned bool) string {
	if override := strings.TrimSpace(storedOverride); override != "" {
		return override
	}
	label = strings.TrimSpace(label)
	if sourceOwned {
		return label
	}
	return ClientDisplayNameFromKeyURL(rawURL, label)
}

// SupportedConfigScheme returns the scheme identifier or "xray-json" for supported configurations.
func SupportedConfigScheme(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") {
		return "xray-json"
	}
	lower := strings.ToLower(trimmed)
	// Recognize supported share schemes before net/url validation. Hysteria 2
	// port-hopping authorities intentionally contain commas and ranges that
	// net/url rejects as an ordinary numeric URL port.
	for _, scheme := range []string{"vless", "vmess", "trojan", "ss", "hysteria2", "hy2", "tuic"} {
		if strings.HasPrefix(lower, scheme+"://") {
			return scheme
		}
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Scheme))
}

// ValidateRealConfigURL checks that raw is a supported configuration scheme or valid XRAY-JSON.
func ValidateRealConfigURL(raw string) error {
	switch SupportedConfigScheme(raw) {
	case "vless", "vmess", "trojan":
		return nil
	case "xray-json":
		return ValidateXrayJSONConfiguration(raw)
	case "":
		return fmt.Errorf("configuration is empty or invalid")
	default:
		return fmt.Errorf("configuration must start with vless://, vmess://, trojan:// or valid XRAY-JSON")
	}
}

// ParseConfigTarget extracts the host and port from raw configuration.
func ParseConfigTarget(raw string) (host, port string, err error) {
	trimmed := strings.TrimSpace(raw)
	switch SupportedConfigScheme(trimmed) {
	case "vless", "trojan":
		parsed, parseErr := url.Parse(trimmed)
		if parseErr != nil {
			return "", "", parseErr
		}
		host = strings.TrimSpace(parsed.Hostname())
		port = strings.TrimSpace(parsed.Port())
	case "vmess":
		payloadRaw := strings.TrimSpace(strings.TrimPrefix(trimmed, "vmess://"))
		payload, parseErr := DecodeVMESSPayload(payloadRaw)
		if parseErr != nil {
			return "", "", parseErr
		}
		host = strings.TrimSpace(payload.Address)
		port = strings.TrimSpace(payload.Port)
	case "xray-json":
		return ParseXrayJSONTarget(trimmed)
	default:
		return "", "", fmt.Errorf("unsupported configuration scheme")
	}

	if host == "" {
		return "", "", fmt.Errorf("missing host")
	}
	if port == "" {
		port = "443"
	}
	return host, port, nil
}

// DecodeVMESSPayload decodes Base64-encoded vmess payload into VMessConfigPayload struct.
func DecodeVMESSPayload(encoded string) (VMessConfigPayload, error) {
	normalized := strings.TrimSpace(encoded)
	normalized = strings.ReplaceAll(normalized, "-", "+")
	normalized = strings.ReplaceAll(normalized, "_", "/")
	if mod := len(normalized) % 4; mod != 0 {
		normalized += strings.Repeat("=", 4-mod)
	}

	decoded, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return VMessConfigPayload{}, err
	}

	var payload VMessConfigPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return VMessConfigPayload{}, err
	}
	return payload, nil
}

// DecodeVMESSPayloadMap decodes Base64-encoded vmess payload into map[string]any.
func DecodeVMESSPayloadMap(encoded string) (map[string]any, error) {
	normalized := strings.TrimSpace(encoded)
	normalized = strings.ReplaceAll(normalized, "-", "+")
	normalized = strings.ReplaceAll(normalized, "_", "/")
	if mod := len(normalized) % 4; mod != 0 {
		normalized += strings.Repeat("=", 4-mod)
	}

	decoded, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// ParseCommaSeparatedValues splits comma-separated string into trimmed non-empty slices.
func ParseCommaSeparatedValues(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

// ParsePortNumber parses port string returning 443 on invalid or out-of-range input.
func ParsePortNumber(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 443
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 || port > 65535 {
		return 443
	}
	return port
}

// DecodeBase64String decodes base64-encoded string, handling base64: prefix and URL/std variants.
func DecodeBase64String(raw string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "base64:"))
	if trimmed == "" {
		return ""
	}

	normalized := strings.ReplaceAll(trimmed, "-", "+")
	normalized = strings.ReplaceAll(normalized, "_", "/")
	normalized = strings.ReplaceAll(normalized, " ", "+")
	if mod := len(normalized) % 4; mod != 0 {
		normalized += strings.Repeat("=", 4-mod)
	}

	decoded, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(decoded))
}

// DecodeQueryComponent unescapes URL query components.
func DecodeQueryComponent(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		return raw
	}
	return strings.TrimSpace(decoded)
}

// ParseFragmentMetadata extracts remark and serverDescription from URL fragment.
func ParseFragmentMetadata(rawFragment string) (string, string) {
	fragment := strings.TrimSpace(rawFragment)
	if fragment == "" {
		return "", ""
	}

	parts := strings.SplitN(fragment, "?", 2)
	remark := DecodeQueryComponent(parts[0])
	if remark == "" {
		remark = strings.TrimSpace(parts[0])
	}
	if len(parts) < 2 {
		return remark, ""
	}

	values, err := url.ParseQuery(parts[1])
	if err != nil {
		return remark, ""
	}
	descriptionRaw := strings.TrimSpace(values.Get("serverDescription"))
	if descriptionRaw == "" {
		return remark, ""
	}

	description := DecodeBase64String(descriptionRaw)
	if description == "" {
		description = DecodeQueryComponent(descriptionRaw)
	}
	return remark, description
}

// ParseBooleanFlag parses boolean string flags ("1", "true", "yes", "on").
func ParseBooleanFlag(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// RecoverLegacyPlusValue heals unescaped '+' turned into spaces by url.ParseQuery.
func RecoverLegacyPlusValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Contains(value, " ") && !strings.Contains(value, "+") {
		value = strings.ReplaceAll(value, " ", "+")
	}
	return strings.TrimSpace(value)
}

// ParseLinkConfiguration parses a vless, vmess, or trojan share link into a LinkConfigurationDraft.
func ParseLinkConfiguration(raw string) (LinkConfigurationDraft, error) {
	trimmed := strings.TrimSpace(raw)
	scheme := SupportedConfigScheme(trimmed)
	switch scheme {
	case "vless", "trojan":
		return parseUserInfoDraft(trimmed, scheme)
	case "vmess":
		return parseVMessDraft(trimmed)
	default:
		return LinkConfigurationDraft{}, fmt.Errorf("unsupported configuration scheme")
	}
}

// parseUserInfoDraft handles the userinfo@host share-link shape used by vless and trojan.
func parseUserInfoDraft(trimmed, scheme string) (LinkConfigurationDraft, error) {
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return LinkConfigurationDraft{}, err
	}
	identifier := ""
	if parsed.User != nil {
		identifier = strings.TrimSpace(parsed.User.Username())
	}
	server := strings.TrimSpace(parsed.Hostname())
	if server == "" {
		return LinkConfigurationDraft{}, fmt.Errorf("missing host")
	}
	if identifier == "" {
		if scheme == "trojan" {
			return LinkConfigurationDraft{}, fmt.Errorf("missing password")
		}
		return LinkConfigurationDraft{}, fmt.Errorf("missing uuid")
	}
	params := parsed.Query()
	publicKey := RecoverLegacyPlusValue(firstNonEmpty(params.Get("pbk"), params.Get("publicKey"), params.Get("password")))
	fingerprint := RecoverLegacyPlusValue(firstNonEmpty(params.Get("fp"), params.Get("fingerprint")))
	remark, serverDescription := ParseFragmentMetadata(parsed.Fragment)
	security := NormalizeXraySecurity(params.Get("security"))
	if strings.EqualFold(strings.TrimSpace(params.Get("tls")), "tls") {
		security = "tls"
	}
	if scheme == "trojan" && strings.TrimSpace(params.Get("security")) == "" && strings.TrimSpace(params.Get("tls")) == "" {
		security = "tls"
	}
	encryption := strings.TrimSpace(params.Get("encryption"))
	if scheme == "vless" && encryption == "" {
		encryption = "none"
	}
	return LinkConfigurationDraft{
		Protocol:          scheme,
		Server:            server,
		Port:              ParsePortNumber(parsed.Port()),
		Identifier:        identifier,
		Remark:            remark,
		ServerDescription: serverDescription,
		Network:           NormalizeXrayNetwork(firstNonEmpty(params.Get("type"), params.Get("net"))),
		Security:          security,
		Path:              strings.TrimSpace(firstNonEmpty(params.Get("path"), params.Get("serviceName"))),
		Host:              strings.TrimSpace(firstNonEmpty(params.Get("host"), params.Get("authority"))),
		SNI:               strings.TrimSpace(firstNonEmpty(params.Get("sni"), params.Get("servername"), params.Get("serverName"))),
		ALPN:              strings.TrimSpace(params.Get("alpn")),
		Flow:              strings.TrimSpace(params.Get("flow")),
		Encryption:        encryption,
		Fingerprint:       fingerprint,
		PublicKey:         publicKey,
		ShortID:           strings.TrimSpace(firstNonEmpty(params.Get("sid"), params.Get("shortId"))),
		SpiderX:           strings.TrimSpace(firstNonEmpty(params.Get("spx"), params.Get("spiderX"))),
		AllowInsecure:     ParseBooleanFlag(firstNonEmpty(params.Get("allowInsecure"), params.Get("allowinsecure"), params.Get("allow_insecure"))),
		GRPCServiceName:   strings.TrimSpace(firstNonEmpty(params.Get("serviceName"), params.Get("path"))),
		HeaderType:        NormalizeHeaderType(firstNonEmpty(params.Get("headerType"), params.Get("header_type"), params.Get("typeHeader"))),
	}, nil
}

func parseVMessDraft(trimmed string) (LinkConfigurationDraft, error) {
	payloadRaw := strings.TrimSpace(strings.TrimPrefix(trimmed, "vmess://"))
	payload, err := DecodeVMESSPayloadMap(payloadRaw)
	if err != nil {
		return LinkConfigurationDraft{}, err
	}
	server := AnyToString(payload["add"])
	if server == "" {
		return LinkConfigurationDraft{}, fmt.Errorf("missing host")
	}
	identifier := AnyToString(payload["id"])
	if identifier == "" {
		return LinkConfigurationDraft{}, fmt.Errorf("missing uuid/id")
	}
	networkRaw := AnyToString(payload["net"])
	if networkRaw == "" {
		networkRaw = AnyToString(payload["type"])
	}
	var security string
	if strings.EqualFold(AnyToString(payload["tls"]), "tls") {
		security = "tls"
	} else {
		security = NormalizeXraySecurity(AnyToString(payload["security"]))
	}
	vmessSecurity := RecoverLegacyPlusValue(firstNonEmpty(AnyToString(payload["encryption"]), AnyToString(payload["scy"]), AnyToString(payload["securityType"])))
	if vmessSecurity == "" {
		vmessSecurity = "auto"
	}
	fingerprint := RecoverLegacyPlusValue(firstNonEmpty(AnyToString(payload["fp"]), AnyToString(payload["fingerprint"])))
	publicKey := RecoverLegacyPlusValue(firstNonEmpty(AnyToString(payload["pbk"]), AnyToString(payload["publicKey"]), AnyToString(payload["password"])))
	remark, serverDescription := ParseFragmentMetadata(AnyToString(payload["remark"]))
	if remark == "" {
		remark = AnyToString(payload["ps"])
	}
	if serverDescription == "" {
		serverDescription = AnyToString(payload["serverDescription"])
	}
	encryption := AnyToString(payload["scy"])
	if encryption == "" {
		encryption = AnyToString(payload["securityType"])
	}
	allowInsecure := ParseBooleanFlag(AnyToString(payload["allowInsecure"]))
	if !allowInsecure {
		allowInsecure = ParseBooleanFlag(AnyToString(payload["allowinsecure"]))
	}
	return LinkConfigurationDraft{
		Protocol:          "vmess",
		Server:            server,
		Port:              ParsePortNumber(AnyToString(payload["port"])),
		Identifier:        identifier,
		Remark:            remark,
		ServerDescription: serverDescription,
		Network:           NormalizeXrayNetwork(networkRaw),
		Security:          security,
		Path:              AnyToString(payload["path"]),
		Host:              AnyToString(payload["host"]),
		SNI:               AnyToString(payload["sni"]),
		ALPN:              AnyToString(payload["alpn"]),
		Flow:              AnyToString(payload["flow"]),
		Encryption:        encryption,
		Fingerprint:       fingerprint,
		PublicKey:         publicKey,
		ShortID:           firstNonEmpty(AnyToString(payload["sid"]), AnyToString(payload["shortId"])),
		SpiderX:           firstNonEmpty(AnyToString(payload["spx"]), AnyToString(payload["spiderX"])),
		AllowInsecure:     allowInsecure,
		GRPCServiceName:   firstNonEmpty(AnyToString(payload["serviceName"]), AnyToString(payload["path"])),
		VMessSecurity:     vmessSecurity,
		VMessAlterID:      strings.TrimSpace(AnyToString(payload["aid"])),
		HeaderType:        NormalizeHeaderType(firstNonEmpty(AnyToString(payload["type"]), AnyToString(payload["headerType"]))),
	}, nil
}
