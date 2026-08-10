package profileconfig

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// AsObject converts any value to map[string]any if possible.
func AsObject(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	obj, ok := value.(map[string]any)
	return obj, ok
}

// AsArray converts any value to []any if possible.
func AsArray(value any) []any {
	arr, ok := value.([]any)
	if !ok {
		return nil
	}
	return arr
}

// AnyToString converts primitive JSON values into trimmed string representations.
func AnyToString(value any) string {
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

// IsGenericXrayOutboundTag identifies routing identifiers that are not
// subscriber-facing node names.
func IsGenericXrayOutboundTag(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "proxy", "direct", "block", "dns", "freedom", "blackhole", "outbound":
		return true
	default:
		return false
	}
}

// AnyToPort extracts a valid numeric port string from primitive JSON values, defaulting to "443".
func AnyToPort(value any) string {
	raw := AnyToString(value)
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

// ParseXrayJSONTarget extracts host and port from XRAY-JSON outbounds.
func ParseXrayJSONTarget(raw string) (host, port string, err error) {
	var parsed any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return "", "", fmt.Errorf("invalid XRAY-JSON syntax")
	}

	root, ok := AsObject(parsed)
	if !ok {
		return "", "", fmt.Errorf("XRAY-JSON must be an object")
	}

	outbounds := AsArray(root["outbounds"])
	if len(outbounds) == 0 {
		return "", "", fmt.Errorf("XRAY-JSON must contain outbounds")
	}

	for _, outboundRaw := range outbounds {
		outbound, ok := AsObject(outboundRaw)
		if !ok {
			continue
		}
		protocol := strings.ToLower(AnyToString(outbound["protocol"]))
		settings, _ := AsObject(outbound["settings"])

		switch protocol {
		case "vless", "vmess":
			vnext := AsArray(settings["vnext"])
			if len(vnext) == 0 {
				continue
			}
			node, _ := AsObject(vnext[0])
			host = AnyToString(node["address"])
			port = AnyToPort(node["port"])

			users := AsArray(node["users"])
			user, _ := AsObject(nil)
			if len(users) > 0 {
				user, _ = AsObject(users[0])
			}
			identifier := AnyToString(user["id"])
			if host == "" || identifier == "" {
				continue
			}
			return host, port, nil
		case "trojan":
			servers := AsArray(settings["servers"])
			if len(servers) == 0 {
				continue
			}
			serverNode, _ := AsObject(servers[0])
			host = AnyToString(serverNode["address"])
			port = AnyToPort(serverNode["port"])
			password := AnyToString(serverNode["password"])
			if host == "" || password == "" {
				continue
			}
			return host, port, nil
		}
	}

	return "", "", fmt.Errorf("XRAY-JSON must contain outbound with protocol vless/vmess/trojan and valid server settings")
}

// ParseXrayJSONDrafts parses all supported outbounds in XRAY-JSON into LinkConfigurationDraft slice.
func ParseXrayJSONDrafts(raw string) ([]LinkConfigurationDraft, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &root); err != nil {
		return nil, fmt.Errorf("invalid XRAY-JSON syntax")
	}
	outbounds := AsArray(root["outbounds"])
	if len(outbounds) == 0 {
		return nil, fmt.Errorf("XRAY-JSON must contain outbounds")
	}
	drafts := make([]LinkConfigurationDraft, 0, len(outbounds))
	for outboundIndex, outboundRaw := range outbounds {
		outbound, ok := AsObject(outboundRaw)
		if !ok {
			continue
		}
		protocol := strings.ToLower(AnyToString(outbound["protocol"]))
		if protocol != "vless" && protocol != "vmess" && protocol != "trojan" {
			continue
		}
		settings, _ := AsObject(outbound["settings"])
		stream, _ := AsObject(outbound["streamSettings"])
		base := LinkConfigurationDraft{
			Protocol: protocol,
			Remark:   AnyToString(outbound["tag"]),
			Network:  NormalizeXrayNetwork(AnyToString(stream["network"])),
			Security: NormalizeXraySecurity(AnyToString(stream["security"])),
		}
		if base.Network == "" {
			base.Network = "tcp"
		}
		tlsSettings, _ := AsObject(stream["tlsSettings"])
		realitySettings, _ := AsObject(stream["realitySettings"])
		base.SNI = firstNonEmpty(AnyToString(realitySettings["serverName"]), AnyToString(tlsSettings["serverName"]))
		base.Fingerprint = firstNonEmpty(AnyToString(realitySettings["fingerprint"]), AnyToString(tlsSettings["fingerprint"]))
		base.ALPN = firstNonEmpty(xrayStringList(realitySettings["alpn"]), xrayStringList(tlsSettings["alpn"]))
		base.PublicKey = AnyToString(realitySettings["publicKey"])
		base.ShortID = AnyToString(realitySettings["shortId"])
		base.SpiderX = AnyToString(realitySettings["spiderX"])
		if insecure, ok := tlsSettings["allowInsecure"].(bool); ok {
			base.AllowInsecure = insecure
		}
		wsSettings, _ := AsObject(stream["wsSettings"])
		base.Path = AnyToString(wsSettings["path"])
		if headers, ok := AsObject(wsSettings["headers"]); ok {
			base.Host = AnyToString(headers["Host"])
			if base.Host == "" {
				base.Host = AnyToString(headers["host"])
			}
		}
		grpcSettings, _ := AsObject(stream["grpcSettings"])
		base.GRPCServiceName = firstNonEmpty(AnyToString(grpcSettings["serviceName"]), AnyToString(grpcSettings["service_name"]))
		if base.Network == "grpc" {
			base.Host = AnyToString(grpcSettings["authority"])
		}
		if base.Network == "httpupgrade" {
			httpUpgradeSettings, _ := AsObject(stream["httpupgradeSettings"])
			base.Path = AnyToString(httpUpgradeSettings["path"])
			base.Host = AnyToString(httpUpgradeSettings["host"])
		}
		if base.Network == "xhttp" {
			xhttpSettings, _ := AsObject(stream["xhttpSettings"])
			base.Path = AnyToString(xhttpSettings["path"])
			base.Host = AnyToString(xhttpSettings["host"])
		}
		if base.Network == "tcp" || base.Network == "raw" {
			rawSettings, _ := AsObject(stream["rawSettings"])
			if rawSettings == nil {
				rawSettings, _ = AsObject(stream["tcpSettings"])
			}
			header, _ := AsObject(rawSettings["header"])
			base.HeaderType = NormalizeHeaderType(AnyToString(header["type"]))
			if request, ok := AsObject(header["request"]); ok {
				base.Path = xrayStringList(request["path"])
				if headers, ok := AsObject(request["headers"]); ok {
					base.Host = firstNonEmpty(xrayStringList(headers["Host"]), xrayStringList(headers["host"]))
				}
			}
		}

		switch protocol {
		case "vless", "vmess":
			nodes := AsArray(settings["vnext"])
			if len(nodes) == 0 {
				return nil, fmt.Errorf("XRAY-JSON outbound %d (%s) has no vnext servers", outboundIndex, protocol)
			}
			for nodeIndex, nodeRaw := range nodes {
				node, ok := AsObject(nodeRaw)
				if !ok {
					return nil, fmt.Errorf("XRAY-JSON outbound %d (%s) vnext %d must be an object", outboundIndex, protocol, nodeIndex)
				}
				users := AsArray(node["users"])
				if len(users) == 0 {
					return nil, fmt.Errorf("XRAY-JSON outbound %d (%s) vnext %d has no users", outboundIndex, protocol, nodeIndex)
				}
				server := AnyToString(node["address"])
				port, portErr := strconv.Atoi(AnyToPort(node["port"]))
				if server == "" || portErr != nil || port <= 0 || port > 65535 {
					return nil, fmt.Errorf("XRAY-JSON outbound %d (%s) vnext %d has invalid server or port", outboundIndex, protocol, nodeIndex)
				}
				for userIndex, userRaw := range users {
					user, ok := AsObject(userRaw)
					if !ok {
						return nil, fmt.Errorf("XRAY-JSON outbound %d (%s) vnext %d user %d must be an object", outboundIndex, protocol, nodeIndex, userIndex)
					}
					draft := base
					draft.Server = server
					draft.Port = port
					draft.Identifier = AnyToString(user["id"])
					draft.Flow = AnyToString(user["flow"])
					draft.Encryption = AnyToString(user["encryption"])
					draft.VMessSecurity = AnyToString(user["security"])
					draft.VMessAlterID = AnyToString(user["alterId"])
					if draft.Identifier == "" {
						return nil, fmt.Errorf("XRAY-JSON outbound %d (%s) vnext %d user %d has no id", outboundIndex, protocol, nodeIndex, userIndex)
					}
					drafts = append(drafts, draft)
				}
			}
		case "trojan":
			servers := AsArray(settings["servers"])
			if len(servers) == 0 {
				return nil, fmt.Errorf("XRAY-JSON outbound %d (trojan) has no servers", outboundIndex)
			}
			for serverIndex, serverRaw := range servers {
				server, ok := AsObject(serverRaw)
				if !ok {
					return nil, fmt.Errorf("XRAY-JSON outbound %d (trojan) server %d must be an object", outboundIndex, serverIndex)
				}
				draft := base
				draft.Server = AnyToString(server["address"])
				draft.Port, _ = strconv.Atoi(AnyToPort(server["port"]))
				draft.Identifier = AnyToString(server["password"])
				if draft.Server == "" || draft.Port <= 0 || draft.Port > 65535 || draft.Identifier == "" {
					return nil, fmt.Errorf("XRAY-JSON outbound %d (trojan) server %d is invalid", outboundIndex, serverIndex)
				}
				drafts = append(drafts, draft)
			}
		}
	}
	if len(drafts) == 0 {
		return nil, fmt.Errorf("XRAY-JSON contains no supported outbound")
	}
	return drafts, nil
}

// ProjectXrayJSONDrafts parses each supported proxy outbound independently for
// delivery projection. Invalid proxy outbounds are counted and skipped, while
// routing helpers such as freedom and blackhole are ignored.
func ProjectXrayJSONDrafts(raw string) ([]LinkConfigurationDraft, int, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &root); err != nil {
		return nil, 0, fmt.Errorf("invalid XRAY-JSON syntax")
	}
	outbounds := AsArray(root["outbounds"])
	if len(outbounds) == 0 {
		return nil, 0, fmt.Errorf("XRAY-JSON must contain outbounds")
	}

	drafts := make([]LinkConfigurationDraft, 0, len(outbounds))
	rejected := 0
	for _, outboundRaw := range outbounds {
		outbound, ok := AsObject(outboundRaw)
		if !ok {
			continue
		}
		protocol := strings.ToLower(AnyToString(outbound["protocol"]))
		switch protocol {
		case "vless", "vmess", "trojan":
			encoded, _ := json.Marshal(map[string]any{"outbounds": []any{outbound}})
			parsed, err := ParseXrayJSONDrafts(string(encoded))
			if err != nil {
				rejected++
				continue
			}
			drafts = append(drafts, parsed...)
		case "shadowsocks":
			parsed, err := projectXrayShadowsocks(outbound)
			if err != nil {
				rejected++
				continue
			}
			drafts = append(drafts, parsed...)
		case "hysteria", "hysteria2":
			parsed, err := projectXrayHysteria2(outbound)
			if err != nil {
				rejected++
				continue
			}
			drafts = append(drafts, parsed...)
		case "tuic":
			parsed, err := projectXrayTUIC(outbound)
			if err != nil {
				rejected++
				continue
			}
			drafts = append(drafts, parsed...)
		}
	}
	return drafts, rejected, nil
}

func outboundRemark(outbound map[string]any) string {
	return firstNonEmpty(AnyToString(outbound["tag"]), AnyToString(outbound["remarks"]))
}

func xrayStringList(value any) string {
	items := AsArray(value)
	if len(items) == 0 {
		return AnyToString(value)
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		if text := AnyToString(item); text != "" {
			values = append(values, text)
		}
	}
	return strings.Join(values, ",")
}

func xrayBool(object map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := object[key].(bool); ok {
			return value
		}
		if ParseBooleanFlag(AnyToString(object[key])) {
			return true
		}
	}
	return false
}

func projectXrayShadowsocks(outbound map[string]any) ([]LinkConfigurationDraft, error) {
	settings, _ := AsObject(outbound["settings"])
	nodes := AsArray(settings["servers"])
	if len(nodes) == 0 {
		nodes = []any{settings}
	}
	drafts := make([]LinkConfigurationDraft, 0, len(nodes))
	for _, nodeRaw := range nodes {
		node, ok := AsObject(nodeRaw)
		if !ok {
			return nil, fmt.Errorf("invalid shadowsocks server")
		}
		plugin := AnyToString(node["plugin"])
		pluginOptions := firstNonEmpty(
			AnyToString(node["pluginOpts"]),
			AnyToString(node["plugin_opts"]),
			AnyToString(node["pluginOptions"]),
		)
		if pluginObject, ok := AsObject(node["plugin"]); ok {
			plugin = firstNonEmpty(AnyToString(pluginObject["name"]), AnyToString(pluginObject["type"]))
			pluginOptions = firstNonEmpty(
				AnyToString(pluginObject["options"]),
				AnyToString(pluginObject["opts"]),
				pluginOptions,
			)
		}
		if pluginOptions != "" {
			if plugin == "" {
				return nil, fmt.Errorf("shadowsocks plugin options without plugin")
			}
			plugin += ";" + pluginOptions
		}
		port, err := strconv.Atoi(AnyToPort(node["port"]))
		draft := LinkConfigurationDraft{
			Protocol: "shadowsocks", Remark: outboundRemark(outbound),
			Server: firstNonEmpty(AnyToString(node["address"]), AnyToString(node["server"])),
			Port:   port, Identifier: AnyToString(node["password"]),
			ShadowsocksMethod: firstNonEmpty(AnyToString(node["method"]), AnyToString(node["cipher"])),
			Plugin:            plugin,
		}
		if err != nil || draft.Server == "" || draft.Identifier == "" || draft.ShadowsocksMethod == "" {
			return nil, fmt.Errorf("incomplete shadowsocks server")
		}
		drafts = append(drafts, draft)
	}
	return drafts, nil
}

func projectXrayHysteria2(outbound map[string]any) ([]LinkConfigurationDraft, error) {
	settings, _ := AsObject(outbound["settings"])
	stream, _ := AsObject(outbound["streamSettings"])
	hysteriaSettings, _ := AsObject(stream["hysteriaSettings"])
	version := firstNonEmpty(AnyToString(settings["version"]), AnyToString(hysteriaSettings["version"]))
	if version != "" && version != "2" {
		return nil, fmt.Errorf("unsupported hysteria version")
	}
	portText := AnyToPort(settings["port"])
	port, err := strconv.Atoi(portText)
	draft := LinkConfigurationDraft{
		Protocol: "hysteria2", Remark: outboundRemark(outbound),
		Server: AnyToString(settings["address"]), Port: port,
		Identifier: firstNonEmpty(AnyToString(hysteriaSettings["auth"]), AnyToString(settings["auth"])),
	}
	tlsSettings, _ := AsObject(stream["tlsSettings"])
	draft.SNI = AnyToString(tlsSettings["serverName"])
	draft.ALPN = xrayStringList(tlsSettings["alpn"])
	draft.AllowInsecure = xrayBool(tlsSettings, "allowInsecure")
	draft.CertificateSHA256 = AnyToString(tlsSettings["pinnedPeerCertSha256"])
	finalMask, _ := AsObject(stream["finalmask"])
	if quicParams, ok := AsObject(finalMask["quicParams"]); ok {
		if udpHop, ok := AsObject(quicParams["udpHop"]); ok {
			draft.PortExpression = AnyToString(udpHop["ports"])
		}
	}
	if masks := AsArray(finalMask["udp"]); len(masks) > 0 {
		if len(masks) != 1 {
			return nil, fmt.Errorf("ambiguous hysteria2 udp masks")
		}
		mask, ok := AsObject(masks[0])
		if !ok {
			return nil, fmt.Errorf("invalid hysteria2 udp mask")
		}
		draft.ObfuscationType = strings.ToLower(AnyToString(mask["type"]))
		if maskSettings, ok := AsObject(mask["settings"]); ok {
			draft.ObfuscationPassword = AnyToString(maskSettings["password"])
		}
		if draft.ObfuscationType != "salamander" || draft.ObfuscationPassword == "" {
			return nil, fmt.Errorf("unsupported hysteria2 udp mask")
		}
	}
	if err != nil || draft.Server == "" || draft.Identifier == "" {
		return nil, fmt.Errorf("incomplete hysteria2 server")
	}
	return []LinkConfigurationDraft{draft}, nil
}

func projectXrayTUIC(outbound map[string]any) ([]LinkConfigurationDraft, error) {
	settings, _ := AsObject(outbound["settings"])
	nodes := AsArray(settings["servers"])
	if len(nodes) == 0 {
		nodes = []any{settings}
	}
	drafts := make([]LinkConfigurationDraft, 0, len(nodes))
	for _, nodeRaw := range nodes {
		node, ok := AsObject(nodeRaw)
		if !ok {
			return nil, fmt.Errorf("invalid tuic server")
		}
		port, err := strconv.Atoi(AnyToPort(node["port"]))
		draft := LinkConfigurationDraft{
			Protocol: "tuic", Remark: outboundRemark(outbound),
			Server: firstNonEmpty(AnyToString(node["address"]), AnyToString(node["server"])), Port: port,
			Identifier: AnyToString(node["uuid"]), TUICPassword: AnyToString(node["password"]),
			SNI:  firstNonEmpty(AnyToString(node["sni"]), AnyToString(node["serverName"])),
			ALPN: xrayStringList(node["alpn"]), AllowInsecure: xrayBool(node, "skip_cert_verify", "allowInsecure"),
			CongestionController: firstNonEmpty(AnyToString(node["congestion_control"]), AnyToString(node["congestionController"])),
			UDPRelayMode:         firstNonEmpty(AnyToString(node["udp_relay_mode"]), AnyToString(node["udpRelayMode"])),
			UDPOverStream:        xrayBool(node, "udp_over_stream"), ZeroRTT: xrayBool(node, "zero_rtt_handshake", "zeroRTT"),
			Heartbeat: AnyToString(node["heartbeat"]),
		}
		if err != nil || draft.Server == "" || draft.Identifier == "" || draft.TUICPassword == "" {
			return nil, fmt.Errorf("incomplete tuic server")
		}
		drafts = append(drafts, draft)
	}
	return drafts, nil
}

// ValidateXrayJSONConfiguration validates XRAY-JSON string.
func ValidateXrayJSONConfiguration(raw string) error {
	if _, _, err := ParseXrayJSONTarget(raw); err != nil {
		return err
	}
	return nil
}

// NormalizeXrayNetwork normalizes network transport type.
func NormalizeXrayNetwork(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "tcp", "raw", "ws", "grpc", "httpupgrade", "xhttp", "h2", "quic":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "tcp"
	}
}

// NormalizeXraySecurity normalizes security transport type.
func NormalizeXraySecurity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none", "tls", "reality":
		return strings.ToLower(strings.TrimSpace(raw))
	case "xtls":
		return "tls"
	default:
		return "none"
	}
}

// NormalizeHeaderType normalizes header type for raw/tcp stream settings.
func NormalizeHeaderType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none", "http":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

// BuildRawHeaderSettings builds raw/tcp header stream settings object.
func BuildRawHeaderSettings(headerType string, host string, path string) map[string]any {
	headerType = NormalizeHeaderType(headerType)
	if headerType == "" {
		return nil
	}

	header := map[string]any{
		"type": headerType,
	}

	if headerType == "http" {
		request := map[string]any{}
		if paths := ParseCommaSeparatedValues(path); len(paths) > 0 {
			request["path"] = paths
		}
		if hosts := ParseCommaSeparatedValues(host); len(hosts) > 0 {
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

// BuildXrayJSONFromLink converts a share link into a complete XRAY-JSON config string.
func BuildXrayJSONFromLink(raw string, fallbackRemark string) (string, error) {
	draft, err := ParseLinkConfiguration(raw)
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
			"network":  NormalizeXrayNetwork(draft.Network),
			"security": NormalizeXraySecurity(draft.Security),
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

	streamSettings, _ := AsObject(outbound["streamSettings"])
	switch NormalizeXrayNetwork(draft.Network) {
	case "tcp", "raw":
		if rawSettings := BuildRawHeaderSettings(draft.HeaderType, draft.Host, draft.Path); len(rawSettings) > 0 {
			// Keep both aliases for broader client compatibility.
			streamSettings["tcpSettings"] = rawSettings
			streamSettings["rawSettings"] = rawSettings
		} else if NormalizeXrayNetwork(draft.Network) == "tcp" {
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

	switch NormalizeXraySecurity(draft.Security) {
	case "tls":
		tlsSettings := map[string]any{}
		if sni := strings.TrimSpace(draft.SNI); sni != "" {
			tlsSettings["serverName"] = sni
		}
		if alpn := ParseCommaSeparatedValues(draft.ALPN); len(alpn) > 0 {
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
