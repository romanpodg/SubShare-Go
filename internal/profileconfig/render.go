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
	if draft.Protocol == "vmess" {
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
	if draft.Protocol == "shadowsocks" || draft.Protocol == "ss" {
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
	if draft.Protocol == "hysteria2" || draft.Protocol == "hy2" {
		port := strconv.Itoa(draft.Port)
		if strings.TrimSpace(draft.PortExpression) != "" {
			port = strings.TrimSpace(draft.PortExpression)
		}
		host := draft.Server
		if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
			host = "[" + host + "]"
		}
		query := url.Values{}
		if draft.SNI != "" {
			query.Set("sni", draft.SNI)
		}
		if draft.ALPN != "" {
			query.Set("alpn", draft.ALPN)
		}
		if draft.AllowInsecure {
			query.Set("insecure", "1")
		}
		if draft.CertificateSHA256 != "" {
			query.Set("pinSHA256", draft.CertificateSHA256)
		}
		if draft.ObfuscationType != "" {
			query.Set("obfs", draft.ObfuscationType)
		}
		if draft.ObfuscationPassword != "" {
			query.Set("obfs-password", draft.ObfuscationPassword)
		}
		link := "hysteria2://" + url.User(draft.Identifier).String() + "@" + host + ":" + port
		if encoded := query.Encode(); encoded != "" {
			link += "?" + encoded
		}
		if draft.Remark != "" {
			link += "#" + url.PathEscape(draft.Remark)
		}
		return link, nil
	}
	if draft.Protocol == "tuic" {
		if draft.TUICPassword == "" {
			return "", fmt.Errorf("canonical tuic configuration is incomplete")
		}
		query := url.Values{}
		if draft.SNI != "" {
			query.Set("sni", draft.SNI)
		}
		if draft.ALPN != "" {
			query.Set("alpn", draft.ALPN)
		}
		if draft.AllowInsecure {
			query.Set("skip_cert_verify", "1")
		}
		if draft.CongestionController != "" {
			query.Set("congestion_control", draft.CongestionController)
		}
		if draft.UDPRelayMode != "" {
			query.Set("udp_relay_mode", draft.UDPRelayMode)
		}
		if draft.UDPOverStream {
			query.Set("udp_over_stream", "1")
		}
		if draft.ZeroRTT {
			query.Set("zero_rtt_handshake", "1")
		}
		if draft.Heartbeat != "" {
			query.Set("heartbeat", draft.Heartbeat)
		}
		link := url.URL{
			Scheme:   "tuic",
			User:     url.UserPassword(draft.Identifier, draft.TUICPassword),
			Host:     net.JoinHostPort(draft.Server, strconv.Itoa(draft.Port)),
			RawQuery: query.Encode(),
			Fragment: draft.Remark,
		}
		return link.String(), nil
	}
	if draft.Protocol != "vless" && draft.Protocol != "trojan" {
		return "", fmt.Errorf("unsupported canonical protocol %q", draft.Protocol)
	}
	query := url.Values{}
	query.Set("type", firstNonEmpty(draft.Network, "tcp"))
	if draft.Security != "" && draft.Security != "none" {
		query.Set("security", draft.Security)
	}
	if draft.Path != "" {
		query.Set("path", draft.Path)
	}
	if draft.Host != "" {
		query.Set("host", draft.Host)
	}
	if draft.SNI != "" {
		query.Set("sni", draft.SNI)
	}
	if draft.ALPN != "" {
		query.Set("alpn", draft.ALPN)
	}
	if draft.Flow != "" {
		query.Set("flow", draft.Flow)
	}
	if draft.Protocol == "vless" {
		query.Set("encryption", firstNonEmpty(draft.Encryption, "none"))
	}
	if draft.Fingerprint != "" {
		query.Set("fp", draft.Fingerprint)
	}
	if draft.PublicKey != "" {
		query.Set("pbk", draft.PublicKey)
	}
	if draft.ShortID != "" {
		query.Set("sid", draft.ShortID)
	}
	if draft.SpiderX != "" {
		query.Set("spx", draft.SpiderX)
	}
	if draft.GRPCServiceName != "" {
		query.Set("serviceName", draft.GRPCServiceName)
	}
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
	return link.String(), nil
}
