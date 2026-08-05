package profileconfig

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
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
