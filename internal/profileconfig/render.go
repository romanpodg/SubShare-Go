package profileconfig

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// BuildShareLinkFromDraft serializes a LinkConfigurationDraft into a vless, vmess, or trojan share link.
func BuildShareLinkFromDraft(draft LinkConfigurationDraft) (string, error) {
	if draft.Server == "" || draft.Port <= 0 || draft.Identifier == "" {
		return "", fmt.Errorf("canonical configuration is incomplete")
	}
	switch draft.Protocol {
	case "vmess":
		return buildVMessLink(draft)
	case "shadowsocks", "ss":
		return buildShadowsocksLink(draft)
	case "hysteria2", "hy2":
		return buildHysteria2Link(draft), nil
	case "tuic":
		return buildTUICLink(draft)
	case "vless", "trojan":
		return buildUserInfoLink(draft), nil
	default:
		return "", fmt.Errorf("unsupported canonical protocol %q", draft.Protocol)
	}
}

func buildVMessLink(draft LinkConfigurationDraft) (string, error) {
	payload := map[string]any{
		"v": "2", "ps": draft.Remark, "add": draft.Server,
		"port": strconv.Itoa(draft.Port), "id": draft.Identifier,
		"aid": firstNonEmpty(draft.VMessAlterID, "0"),
		"scy": firstNonEmpty(draft.VMessSecurity, "auto"),
		"net": firstNonEmpty(draft.Network, "tcp"), "type": draft.HeaderType,
		"host": draft.Host, "path": draft.Path, "sni": draft.SNI,
		"alpn": draft.ALPN, "fp": draft.Fingerprint,
	}
	if draft.Security == "tls" || draft.Security == "reality" {
		payload["tls"] = draft.Security
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(encoded), nil
}

func buildShadowsocksLink(draft LinkConfigurationDraft) (string, error) {
	if draft.ShadowsocksMethod == "" {
		return "", fmt.Errorf("canonical shadowsocks configuration is incomplete")
	}
	link := url.URL{
		Scheme:   "ss",
		User:     url.UserPassword(draft.ShadowsocksMethod, draft.Identifier),
		Host:     net.JoinHostPort(draft.Server, strconv.Itoa(draft.Port)),
		Fragment: draft.Remark,
	}
	query := url.Values{}
	if draft.Plugin != "" {
		query.Set("plugin", draft.Plugin)
	}
	link.RawQuery = query.Encode()
	return link.String(), nil
}

// buildHysteria2Link assembles the URI by hand: port-hopping expressions are
// not a valid net/url port.
func buildHysteria2Link(draft LinkConfigurationDraft) string {
	port := strconv.Itoa(draft.Port)
	if strings.TrimSpace(draft.PortExpression) != "" {
		port = strings.TrimSpace(draft.PortExpression)
	}
	host := draft.Server
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	query := url.Values{}
	setQueryIfPresent(query, "sni", draft.SNI)
	setQueryIfPresent(query, "alpn", draft.ALPN)
	if draft.AllowInsecure {
		query.Set("insecure", "1")
	}
	setQueryIfPresent(query, "pinSHA256", draft.CertificateSHA256)
	setQueryIfPresent(query, "obfs", draft.ObfuscationType)
	setQueryIfPresent(query, "obfs-password", draft.ObfuscationPassword)
	link := "hysteria2://" + url.User(draft.Identifier).String() + "@" + host + ":" + port
	if encoded := query.Encode(); encoded != "" {
		link += "?" + encoded
	}
	if draft.Remark != "" {
		link += "#" + url.PathEscape(draft.Remark)
	}
	return link
}

func buildTUICLink(draft LinkConfigurationDraft) (string, error) {
	if draft.TUICPassword == "" {
		return "", fmt.Errorf("canonical tuic configuration is incomplete")
	}
	query := url.Values{}
	setQueryIfPresent(query, "sni", draft.SNI)
	setQueryIfPresent(query, "alpn", draft.ALPN)
	if draft.AllowInsecure {
		query.Set("skip_cert_verify", "1")
	}
	setQueryIfPresent(query, "congestion_control", draft.CongestionController)
	setQueryIfPresent(query, "udp_relay_mode", draft.UDPRelayMode)
	if draft.UDPOverStream {
		query.Set("udp_over_stream", "1")
	}
	if draft.ZeroRTT {
		query.Set("zero_rtt_handshake", "1")
	}
	setQueryIfPresent(query, "heartbeat", draft.Heartbeat)
	link := url.URL{
		Scheme:   "tuic",
		User:     url.UserPassword(draft.Identifier, draft.TUICPassword),
		Host:     net.JoinHostPort(draft.Server, strconv.Itoa(draft.Port)),
		RawQuery: query.Encode(),
		Fragment: draft.Remark,
	}
	return link.String(), nil
}

// buildUserInfoLink renders the vless/trojan identifier@host shape.
func buildUserInfoLink(draft LinkConfigurationDraft) string {
	query := url.Values{}
	query.Set("type", firstNonEmpty(draft.Network, "tcp"))
	if draft.Security != "" && draft.Security != "none" {
		query.Set("security", draft.Security)
	}
	setQueryIfPresent(query, "path", draft.Path)
	setQueryIfPresent(query, "host", draft.Host)
	setQueryIfPresent(query, "sni", draft.SNI)
	setQueryIfPresent(query, "alpn", draft.ALPN)
	setQueryIfPresent(query, "flow", draft.Flow)
	if draft.Protocol == "vless" {
		query.Set("encryption", firstNonEmpty(draft.Encryption, "none"))
	}
	setQueryIfPresent(query, "fp", draft.Fingerprint)
	setQueryIfPresent(query, "pbk", draft.PublicKey)
	setQueryIfPresent(query, "sid", draft.ShortID)
	setQueryIfPresent(query, "spx", draft.SpiderX)
	setQueryIfPresent(query, "serviceName", draft.GRPCServiceName)
	if draft.AllowInsecure {
		query.Set("allowInsecure", "1")
	}
	link := url.URL{
		Scheme:   draft.Protocol,
		User:     url.User(draft.Identifier),
		Host:     net.JoinHostPort(draft.Server, strconv.Itoa(draft.Port)),
		RawQuery: query.Encode(),
		Fragment: draft.Remark,
	}
	return link.String()
}

func setQueryIfPresent(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}
