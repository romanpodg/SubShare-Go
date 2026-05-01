package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"xary-sub/internal/vless"
)

type vmessConfigPayload struct {
	Address string `json:"add"`
	Port    string `json:"port"`
	ID      string `json:"id"`
	Name    string `json:"ps"`
}

type linkConfigurationDraft struct {
	Protocol          string
	Server            string
	Port              int
	Identifier        string
	Remark            string
	ServerDescription string
	Network           string
	Security          string
	Path              string
	Host              string
	SNI               string
	ALPN              string
	Flow              string
	Encryption        string
	Fingerprint       string
	PublicKey         string
	ShortID           string
	SpiderX           string
	AllowInsecure     bool
	GRPCServiceName   string
	VMessSecurity     string
	VMessAlterID      string
	HeaderType        string
}

func supportedConfigScheme(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "{") {
		return "xray-json"
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "vmess://") {
		return "vmess"
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Scheme))
}

func validateRealConfigURL(raw string) error {
	switch supportedConfigScheme(raw) {
	case "vless", "vmess", "trojan":
		return nil
	case "xray-json":
		return validateXrayJSONConfiguration(raw)
	case "":
		return fmt.Errorf("configuration is empty or invalid")
	default:
		return fmt.Errorf("configuration must start with vless://, vmess://, trojan:// or valid XRAY-JSON")
	}
}

func parseConfigTarget(raw string) (host, port string, err error) {
	trimmed := strings.TrimSpace(raw)
	switch supportedConfigScheme(trimmed) {
	case "vless", "trojan":
		parsed, parseErr := url.Parse(trimmed)
		if parseErr != nil {
			return "", "", parseErr
		}
		host = strings.TrimSpace(parsed.Hostname())
		port = strings.TrimSpace(parsed.Port())
	case "vmess":
		payloadRaw := strings.TrimSpace(strings.TrimPrefix(trimmed, "vmess://"))
		payload, parseErr := decodeVMESSPayload(payloadRaw)
		if parseErr != nil {
			return "", "", parseErr
		}
		host = strings.TrimSpace(payload.Address)
		port = strings.TrimSpace(payload.Port)
	case "xray-json":
		return parseXrayJSONTarget(trimmed)
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

func asObject(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	obj, ok := value.(map[string]any)
	return obj, ok
}

func asArray(value any) []any {
	arr, ok := value.([]any)
	if !ok {
		return nil
	}
	return arr
}

func anyToString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return strings.TrimSpace(fmt.Sprintf("%g", typed))
	default:
		return ""
	}
}

func anyToPort(value any) string {
	raw := anyToString(value)
	if raw == "" {
		return "443"
	}
	portNumber := 0
	_, err := fmt.Sscanf(raw, "%d", &portNumber)
	if err != nil || portNumber <= 0 || portNumber > 65535 {
		return "443"
	}
	return fmt.Sprintf("%d", portNumber)
}

func parseXrayJSONTarget(raw string) (host, port string, err error) {
	var parsed any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return "", "", fmt.Errorf("invalid XRAY-JSON syntax")
	}

	root, ok := asObject(parsed)
	if !ok {
		return "", "", fmt.Errorf("XRAY-JSON must be an object")
	}

	outbounds := asArray(root["outbounds"])
	if len(outbounds) == 0 {
		return "", "", fmt.Errorf("XRAY-JSON must contain outbounds")
	}

	for _, outboundRaw := range outbounds {
		outbound, ok := asObject(outboundRaw)
		if !ok {
			continue
		}
		protocol := strings.ToLower(anyToString(outbound["protocol"]))
		settings, _ := asObject(outbound["settings"])

		switch protocol {
		case "vless", "vmess":
			vnext := asArray(settings["vnext"])
			if len(vnext) == 0 {
				continue
			}
			node, _ := asObject(vnext[0])
			host = anyToString(node["address"])
			port = anyToPort(node["port"])

			users := asArray(node["users"])
			user, _ := asObject(nil)
			if len(users) > 0 {
				user, _ = asObject(users[0])
			}
			identifier := anyToString(user["id"])
			if host == "" || identifier == "" {
				continue
			}
			return host, port, nil
		case "trojan":
			servers := asArray(settings["servers"])
			if len(servers) == 0 {
				continue
			}
			serverNode, _ := asObject(servers[0])
			host = anyToString(serverNode["address"])
			port = anyToPort(serverNode["port"])
			password := anyToString(serverNode["password"])
			if host == "" || password == "" {
				continue
			}
			return host, port, nil
		}
	}

	return "", "", fmt.Errorf("XRAY-JSON must contain outbound with protocol vless/vmess/trojan and valid server settings")
}

func validateXrayJSONConfiguration(raw string) error {
	if _, _, err := parseXrayJSONTarget(raw); err != nil {
		return err
	}
	return nil
}

func decodeVMESSPayload(encoded string) (vmessConfigPayload, error) {
	normalized := strings.TrimSpace(encoded)
	normalized = strings.ReplaceAll(normalized, "-", "+")
	normalized = strings.ReplaceAll(normalized, "_", "/")
	if mod := len(normalized) % 4; mod != 0 {
		normalized += strings.Repeat("=", 4-mod)
	}

	decoded, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return vmessConfigPayload{}, err
	}

	var payload vmessConfigPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return vmessConfigPayload{}, err
	}
	return payload, nil
}

func decodeVMESSPayloadMap(encoded string) (map[string]any, error) {
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

func normalizeXrayNetwork(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "tcp", "raw", "ws", "grpc", "httpupgrade", "xhttp", "h2", "quic":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "tcp"
	}
}

func normalizeXraySecurity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none", "tls", "reality":
		return strings.ToLower(strings.TrimSpace(raw))
	case "xtls":
		return "tls"
	default:
		return "none"
	}
}

func parseCommaSeparatedValues(raw string) []string {
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

func parsePortNumber(raw string) int {
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

func decodeBase64String(raw string) string {
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

func decodeQueryComponent(raw string) string {
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

func parseFragmentMetadata(rawFragment string) (string, string) {
	fragment := strings.TrimSpace(rawFragment)
	if fragment == "" {
		return "", ""
	}

	parts := strings.SplitN(fragment, "?", 2)
	remark := decodeQueryComponent(parts[0])
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

	description := decodeBase64String(descriptionRaw)
	if description == "" {
		description = decodeQueryComponent(descriptionRaw)
	}
	return remark, description
}

func parseBooleanFlag(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizeHeaderType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none", "http":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func buildRawHeaderSettings(headerType string, host string, path string) map[string]any {
	headerType = normalizeHeaderType(headerType)
	if headerType == "" {
		return nil
	}

	header := map[string]any{
		"type": headerType,
	}

	if headerType == "http" {
		request := map[string]any{}
		if paths := parseCommaSeparatedValues(path); len(paths) > 0 {
			request["path"] = paths
		}
		if hosts := parseCommaSeparatedValues(host); len(hosts) > 0 {
			request["headers"] = map[string]any{
				"Host": hosts,
			}
		}
		if len(request) > 0 {
			header["request"] = request
		}
	}

	return map[string]any{
		"header": header,
	}
}

func recoverLegacyPlusValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	// Some providers publish links without encodeURIComponent for '+'.
	// url.ParseQuery turns '+' into space, which breaks auth/public-key values.
	if strings.Contains(value, " ") && !strings.Contains(value, "+") {
		value = strings.ReplaceAll(value, " ", "+")
	}
	return strings.TrimSpace(value)
}

func parseLinkConfiguration(raw string) (linkConfigurationDraft, error) {
	trimmed := strings.TrimSpace(raw)
	scheme := supportedConfigScheme(trimmed)
	switch scheme {
	case "vless", "trojan":
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return linkConfigurationDraft{}, err
		}
		identifier := ""
		if parsed.User != nil {
			identifier = strings.TrimSpace(parsed.User.Username())
		}
		server := strings.TrimSpace(parsed.Hostname())
		if server == "" {
			return linkConfigurationDraft{}, fmt.Errorf("missing host")
		}
		if identifier == "" {
			if scheme == "trojan" {
				return linkConfigurationDraft{}, fmt.Errorf("missing password")
			}
			return linkConfigurationDraft{}, fmt.Errorf("missing uuid")
		}
		params := parsed.Query()
		publicKey := recoverLegacyPlusValue(firstNonEmpty(params.Get("pbk"), params.Get("publicKey"), params.Get("password")))
		fingerprint := recoverLegacyPlusValue(firstNonEmpty(params.Get("fp"), params.Get("fingerprint")))
		remark, serverDescription := parseFragmentMetadata(parsed.Fragment)
		security := normalizeXraySecurity(params.Get("security"))
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
		return linkConfigurationDraft{
			Protocol:          scheme,
			Server:            server,
			Port:              parsePortNumber(parsed.Port()),
			Identifier:        identifier,
			Remark:            remark,
			ServerDescription: serverDescription,
			Network:           normalizeXrayNetwork(firstNonEmpty(params.Get("type"), params.Get("net"))),
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
			AllowInsecure:     parseBooleanFlag(firstNonEmpty(params.Get("allowInsecure"), params.Get("allowinsecure"), params.Get("allow_insecure"))),
			GRPCServiceName:   strings.TrimSpace(firstNonEmpty(params.Get("serviceName"), params.Get("path"))),
			HeaderType:        normalizeHeaderType(firstNonEmpty(params.Get("headerType"), params.Get("header_type"), params.Get("typeHeader"))),
		}, nil
	case "vmess":
		payloadRaw := strings.TrimSpace(strings.TrimPrefix(trimmed, "vmess://"))
		payload, err := decodeVMESSPayloadMap(payloadRaw)
		if err != nil {
			return linkConfigurationDraft{}, err
		}
		server := anyToString(payload["add"])
		if server == "" {
			return linkConfigurationDraft{}, fmt.Errorf("missing host")
		}
		identifier := anyToString(payload["id"])
		if identifier == "" {
			return linkConfigurationDraft{}, fmt.Errorf("missing uuid/id")
		}
		networkRaw := anyToString(payload["net"])
		if networkRaw == "" {
			networkRaw = anyToString(payload["type"])
		}
		security := "none"
		if strings.EqualFold(anyToString(payload["tls"]), "tls") {
			security = "tls"
		} else {
			security = normalizeXraySecurity(anyToString(payload["security"]))
		}
		vmessSecurity := recoverLegacyPlusValue(firstNonEmpty(anyToString(payload["encryption"]), anyToString(payload["scy"]), anyToString(payload["securityType"])))
		if vmessSecurity == "" {
			vmessSecurity = "auto"
		}
		fingerprint := recoverLegacyPlusValue(firstNonEmpty(anyToString(payload["fp"]), anyToString(payload["fingerprint"])))
		publicKey := recoverLegacyPlusValue(firstNonEmpty(anyToString(payload["pbk"]), anyToString(payload["publicKey"]), anyToString(payload["password"])))
		remark, serverDescription := parseFragmentMetadata(anyToString(payload["remark"]))
		if remark == "" {
			remark = anyToString(payload["ps"])
		}
		if serverDescription == "" {
			serverDescription = anyToString(payload["serverDescription"])
		}
		encryption := anyToString(payload["scy"])
		if encryption == "" {
			encryption = anyToString(payload["securityType"])
		}
		allowInsecure := parseBooleanFlag(anyToString(payload["allowInsecure"]))
		if !allowInsecure {
			allowInsecure = parseBooleanFlag(anyToString(payload["allowinsecure"]))
		}
		return linkConfigurationDraft{
			Protocol:          "vmess",
			Server:            server,
			Port:              parsePortNumber(anyToString(payload["port"])),
			Identifier:        identifier,
			Remark:            remark,
			ServerDescription: serverDescription,
			Network:           normalizeXrayNetwork(networkRaw),
			Security:          security,
			Path:              anyToString(payload["path"]),
			Host:              anyToString(payload["host"]),
			SNI:               anyToString(payload["sni"]),
			ALPN:              anyToString(payload["alpn"]),
			Flow:              anyToString(payload["flow"]),
			Encryption:        encryption,
			Fingerprint:       fingerprint,
			PublicKey:         publicKey,
			ShortID:           firstNonEmpty(anyToString(payload["sid"]), anyToString(payload["shortId"])),
			SpiderX:           firstNonEmpty(anyToString(payload["spx"]), anyToString(payload["spiderX"])),
			AllowInsecure:     allowInsecure,
			GRPCServiceName:   firstNonEmpty(anyToString(payload["serviceName"]), anyToString(payload["path"])),
			VMessSecurity:     vmessSecurity,
			VMessAlterID:      strings.TrimSpace(anyToString(payload["aid"])),
			HeaderType:        normalizeHeaderType(firstNonEmpty(anyToString(payload["type"]), anyToString(payload["headerType"]))),
		}, nil
	default:
		return linkConfigurationDraft{}, fmt.Errorf("unsupported configuration scheme")
	}
}

func buildXrayJSONFromLink(raw string, fallbackRemark string) (string, error) {
	draft, err := parseLinkConfiguration(raw)
	if err != nil {
		return "", err
	}

	tag := strings.TrimSpace(firstNonEmpty(draft.Remark, fallbackRemark))
	if tag == "" {
		tag = "proxy"
	}

	outbound := map[string]any{
		"tag":      tag,
		"protocol": draft.Protocol,
		"settings": map[string]any{},
		"streamSettings": map[string]any{
			"network":  normalizeXrayNetwork(draft.Network),
			"security": normalizeXraySecurity(draft.Security),
		},
	}

	if draft.Protocol == "trojan" {
		outbound["settings"] = map[string]any{
			"servers": []any{
				map[string]any{
					"address":  draft.Server,
					"port":     draft.Port,
					"password": draft.Identifier,
				},
			},
		}
	} else {
		userNode := map[string]any{
			"id": draft.Identifier,
		}
		if draft.Protocol == "vless" {
			userNode["encryption"] = firstNonEmpty(draft.Encryption, "none")
			if flow := strings.TrimSpace(draft.Flow); flow != "" {
				userNode["flow"] = flow
			}
		}
		if draft.Protocol == "vmess" {
			vmessSecurity := strings.TrimSpace(firstNonEmpty(draft.VMessSecurity, draft.Encryption))
			if vmessSecurity == "" {
				vmessSecurity = "auto"
			}
			userNode["security"] = vmessSecurity
			alterIDRaw := strings.TrimSpace(draft.VMessAlterID)
			if alterIDRaw != "" {
				if alterID, convErr := strconv.Atoi(alterIDRaw); convErr == nil && alterID >= 0 {
					userNode["alterId"] = alterID
				} else {
					userNode["alterId"] = alterIDRaw
				}
			}
		}
		outbound["settings"] = map[string]any{
			"vnext": []any{
				map[string]any{
					"address": draft.Server,
					"port":    draft.Port,
					"users": []any{
						userNode,
					},
				},
			},
		}
	}

	streamSettings, _ := asObject(outbound["streamSettings"])
	switch normalizeXrayNetwork(draft.Network) {
	case "tcp", "raw":
		if rawSettings := buildRawHeaderSettings(draft.HeaderType, draft.Host, draft.Path); len(rawSettings) > 0 {
			// Keep both aliases for broader client compatibility.
			streamSettings["tcpSettings"] = rawSettings
			streamSettings["rawSettings"] = rawSettings
		} else if normalizeXrayNetwork(draft.Network) == "tcp" {
			// Many clients expect tcpSettings to exist for tcp transport.
			streamSettings["tcpSettings"] = map[string]any{}
		}
	case "ws":
		wsSettings := map[string]any{
			"path": draft.Path,
		}
		if host := strings.TrimSpace(draft.Host); host != "" {
			wsSettings["headers"] = map[string]any{
				"Host": host,
			}
		}
		streamSettings["wsSettings"] = wsSettings
	case "grpc":
		grpcSettings := map[string]any{
			"serviceName": firstNonEmpty(strings.TrimSpace(draft.GRPCServiceName), strings.TrimSpace(draft.Path)),
		}
		if authority := strings.TrimSpace(draft.Host); authority != "" {
			grpcSettings["authority"] = authority
		}
		streamSettings["grpcSettings"] = grpcSettings
	case "httpupgrade":
		streamSettings["httpupgradeSettings"] = map[string]any{
			"path": strings.TrimSpace(draft.Path),
			"host": strings.TrimSpace(draft.Host),
		}
	case "xhttp":
		streamSettings["xhttpSettings"] = map[string]any{
			"path": strings.TrimSpace(draft.Path),
			"host": strings.TrimSpace(draft.Host),
		}
	}

	switch normalizeXraySecurity(draft.Security) {
	case "tls":
		tlsSettings := map[string]any{}
		if sni := strings.TrimSpace(draft.SNI); sni != "" {
			tlsSettings["serverName"] = sni
		}
		if alpn := parseCommaSeparatedValues(draft.ALPN); len(alpn) > 0 {
			tlsSettings["alpn"] = alpn
		}
		if draft.AllowInsecure {
			tlsSettings["allowInsecure"] = true
		}
		if fp := strings.TrimSpace(draft.Fingerprint); fp != "" {
			tlsSettings["fingerprint"] = fp
		}
		streamSettings["tlsSettings"] = tlsSettings
	case "reality":
		realitySettings := map[string]any{}
		if sni := strings.TrimSpace(draft.SNI); sni != "" {
			realitySettings["serverName"] = sni
		}
		realitySettings["show"] = false
		if fp := strings.TrimSpace(draft.Fingerprint); fp != "" {
			realitySettings["fingerprint"] = fp
		}
		if publicKey := strings.TrimSpace(draft.PublicKey); publicKey != "" {
			realitySettings["publicKey"] = publicKey
			// Keep new and legacy names for better client compatibility.
			realitySettings["password"] = publicKey
		}
		if shortID := strings.TrimSpace(draft.ShortID); shortID != "" {
			realitySettings["shortId"] = shortID
		}
		if spiderX := strings.TrimSpace(draft.SpiderX); spiderX != "" {
			realitySettings["spiderX"] = spiderX
		}
		streamSettings["realitySettings"] = realitySettings
	}

	config := map[string]any{
		"remarks": tag,
		"dns": map[string]any{
			"servers":       []any{"1.1.1.1", "1.0.0.1"},
			"queryStrategy": "UseIP",
		},
		"routing": map[string]any{
			"rules": []any{
				map[string]any{
					"type":        "field",
					"protocol":    []any{"bittorrent"},
					"outboundTag": "direct",
				},
			},
			"domainMatcher":  "hybrid",
			"domainStrategy": "IPIfNonMatch",
		},
		"inbounds": []any{
			map[string]any{
				"tag":      "socks",
				"port":     10808,
				"listen":   "127.0.0.1",
				"protocol": "socks",
				"settings": map[string]any{
					"udp":  true,
					"auth": "noauth",
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"routeOnly":    false,
					"destOverride": []any{"http", "tls", "quic"},
				},
			},
			map[string]any{
				"tag":      "http",
				"port":     10809,
				"listen":   "127.0.0.1",
				"protocol": "http",
				"settings": map[string]any{
					"allowTransparent": false,
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"routeOnly":    false,
					"destOverride": []any{"http", "tls", "quic"},
				},
			},
		},
		"outbounds": []any{
			outbound,
			map[string]any{
				"tag":      "direct",
				"protocol": "freedom",
			},
			map[string]any{
				"tag":      "block",
				"protocol": "blackhole",
			},
		},
	}
	if description := strings.TrimSpace(draft.ServerDescription); description != "" {
		config["meta"] = map[string]any{
			"serverDescription": description,
		}
	}

	encoded, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func normalizeConfigurationForSubscriptionOutput(raw string, format string, fallbackRemark string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("configuration is empty")
	}
	if strings.TrimSpace(format) != "xray-json" {
		return trimmed, nil
	}

	scheme := supportedConfigScheme(trimmed)
	switch scheme {
	case "xray-json":
		var parsed any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return "", fmt.Errorf("invalid XRAY-JSON syntax")
		}
		normalized, err := json.Marshal(parsed)
		if err != nil {
			return "", err
		}
		return string(normalized), nil
	case "vless", "vmess", "trojan":
		return buildXrayJSONFromLink(trimmed, fallbackRemark)
	default:
		return "", fmt.Errorf("unsupported configuration scheme")
	}
}

func checkConfigurationAvailability(raw string) (string, string, int64) {
	host, port, err := parseConfigTarget(raw)
	if err != nil {
		return "down", "invalid configuration", 0
	}

	ips, err := net.LookupHost(host)
	if err != nil {
		return "down", fmt.Sprintf("DNS lookup failed: %s", err.Error()), 0
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if vless.IsPrivateIP(ip) {
			return "down", "health check to private addresses is not allowed", 0
		}
	}

	address := net.JoinHostPort(host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, 4*time.Second)
	if err != nil {
		return "down", err.Error(), 0
	}
	_ = conn.Close()

	latency := time.Since(start).Milliseconds()
	if latency < 0 {
		latency = 0
	}
	return "up", "", latency
}
