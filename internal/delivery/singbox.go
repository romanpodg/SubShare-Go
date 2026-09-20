package delivery

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func RenderSingBox(entries []Entry) (Generated, error) {
	return renderStructured(entries, structuredMapper{
		format:     "sing-box",
		payloadKey: "outbounds",
		profile:    singBoxOutbound,
		legacy:     singBoxLegacyOutbound,
	})
}

func singBoxLegacyOutbound(draft profileconfig.LinkConfigurationDraft, name string) map[string]any {
	outbound := map[string]any{"type": draft.Protocol, "tag": name, "server": draft.Server, "server_port": draft.Port}
	applySingBoxLegacyCredentials(outbound, draft)
	if tlsOptions := singBoxLegacyTLS(draft); tlsOptions != nil {
		outbound["tls"] = tlsOptions
	}
	if transport := singBoxLegacyTransport(draft); transport != nil {
		outbound["transport"] = transport
	}
	return outbound
}

func applySingBoxLegacyCredentials(outbound map[string]any, draft profileconfig.LinkConfigurationDraft) {
	switch draft.Protocol {
	case "vless":
		outbound["uuid"] = draft.Identifier
		setIfNotEmpty(outbound, "flow", draft.Flow)
	case "vmess":
		outbound["uuid"] = draft.Identifier
		outbound["security"] = firstNonEmpty(draft.VMessSecurity, "auto")
		if alterID, err := strconv.Atoi(draft.VMessAlterID); err == nil && alterID > 0 {
			outbound["alter_id"] = alterID
		}
	case "trojan":
		outbound["password"] = draft.Identifier
	}
}

func singBoxLegacyTLS(draft profileconfig.LinkConfigurationDraft) map[string]any {
	security := strings.ToLower(strings.TrimSpace(draft.Security))
	if security != "tls" && security != "reality" {
		return nil
	}
	tlsOptions := map[string]any{"enabled": true, "insecure": draft.AllowInsecure}
	setIfNotEmpty(tlsOptions, "server_name", draft.SNI)
	if draft.Fingerprint != "" {
		tlsOptions["utls"] = map[string]any{"enabled": true, "fingerprint": draft.Fingerprint}
	}
	if security == "reality" {
		reality := map[string]any{"enabled": true}
		setIfNotEmpty(reality, "public_key", draft.PublicKey)
		setIfNotEmpty(reality, "short_id", draft.ShortID)
		tlsOptions["reality"] = reality
	}
	return tlsOptions
}

func singBoxLegacyTransport(draft profileconfig.LinkConfigurationDraft) map[string]any {
	switch strings.TrimSpace(draft.Network) {
	case "ws":
		transport := map[string]any{"type": "ws"}
		setIfNotEmpty(transport, "path", draft.Path)
		if draft.Host != "" {
			transport["headers"] = map[string]string{"Host": draft.Host}
		}
		return transport
	case "grpc":
		transport := map[string]any{"type": "grpc"}
		setIfNotEmpty(transport, "service_name", draft.GRPCServiceName)
		return transport
	}
	return nil
}

func singBoxOutbound(profile *profiles.Profile, name string) (map[string]any, string) {
	switch data := profile.Data.(type) {
	case profiles.ShadowsocksData:
		return singBoxShadowsocks(profile, data, name)
	case profiles.Hysteria2Data:
		return singBoxHysteria2(profile, data, name)
	case profiles.TUICData:
		return singBoxTUIC(profile, data, name)
	default:
		return nil, ReasonUnsupportedProtocol
	}
}

func singBoxShadowsocks(profile *profiles.Profile, data profiles.ShadowsocksData, name string) (map[string]any, string) {
	if !supportsShadowsocksMethod(shadowsocksMethods, data.Method) {
		return nil, ReasonClientVersion
	}
	outbound := map[string]any{"type": "shadowsocks", "tag": name, "server": profile.Server, "server_port": portNumber(profile.Port), "method": data.Method, "password": data.Password.Reveal()}
	if data.Plugin != nil && !applySingBoxShadowsocksPlugin(outbound, data.Plugin) {
		return nil, ReasonPlugin
	}
	return outbound, ""
}

func applySingBoxShadowsocksPlugin(outbound map[string]any, plugin *profiles.ShadowsocksPlugin) bool {
	name, ok := singBoxShadowsocksPlugin(plugin)
	if !ok {
		return false
	}
	outbound["plugin"] = name
	if plugin.Options.IsSet() {
		outbound["plugin_opts"] = plugin.Options.Reveal()
	}
	return true
}

func singBoxHysteria2(profile *profiles.Profile, data profiles.Hysteria2Data, name string) (map[string]any, string) {
	if data.ObfuscationType == "gecko" {
		return nil, ReasonClientVersion
	}
	if data.CertificateSHA256 != "" {
		return nil, ReasonUnrepresentable
	}
	outbound := map[string]any{"type": "hysteria2", "tag": name, "server": profile.Server, "password": data.Authentication.Reveal(), "tls": map[string]any{"enabled": true, "server_name": data.SNI, "insecure": data.Insecure}}
	if profile.Port.Kind == profiles.PortExpression {
		outbound["server_ports"] = singBoxPortRanges(profile.Port)
	} else {
		outbound["server_port"] = portNumber(profile.Port)
	}
	if data.ObfuscationType != "" {
		outbound["obfs"] = map[string]any{"type": data.ObfuscationType, "password": data.ObfuscationPassword.Reveal()}
	}
	return outbound, ""
}

func singBoxTUIC(profile *profiles.Profile, data profiles.TUICData, name string) (map[string]any, string) {
	if data.Generation != 5 {
		return nil, ReasonCompatibility
	}
	if tuicHasFieldClass(data, profiles.TUICProvenanceMihomo) {
		return nil, ReasonUnrepresentable
	}
	tlsOptions := map[string]any{"enabled": true, "server_name": data.SNI, "insecure": data.SkipCertificateVerification}
	if len(data.ALPN) > 0 {
		tlsOptions["alpn"] = append([]string(nil), data.ALPN...)
	}
	outbound := map[string]any{"type": "tuic", "tag": name, "server": profile.Server, "server_port": portNumber(profile.Port), "uuid": data.UUID.Reveal(), "password": data.Password.Reveal(), "congestion_control": data.CongestionController, "tls": tlsOptions}
	if data.UDPOverStream {
		outbound["udp_over_stream"] = true
	} else {
		outbound["udp_relay_mode"] = data.UDPRelayMode
	}
	setIfTrue(outbound, "zero_rtt_handshake", data.ZeroRTT)
	setIfNotEmpty(outbound, "heartbeat", data.Heartbeat)
	return outbound, ""
}

func singBoxPortRanges(port profiles.PortSpec) []string {
	result := make([]string, 0, len(port.Ranges))
	for _, item := range port.Ranges {
		if item.Start == item.End {
			value := strconv.Itoa(int(item.Start))
			result = append(result, value+":"+value)
		} else {
			result = append(result, fmt.Sprintf("%d:%d", item.Start, item.End))
		}
	}
	return result
}

func singBoxShadowsocksPlugin(plugin *profiles.ShadowsocksPlugin) (string, bool) {
	name, options, ok := shadowsocksPluginOptions(plugin)
	if !ok {
		return "", false
	}
	switch name {
	case "obfs-local":
		return name, validSingBoxObfsOptions(options)
	case "v2ray-plugin":
		return name, validSingBoxV2rayPluginOptions(options)
	}
	return "", false
}

func validSingBoxObfsOptions(options map[string]string) bool {
	for key := range options {
		if key != "obfs" && key != "obfs-host" {
			return false
		}
	}
	return true
}

func validSingBoxV2rayPluginOptions(options map[string]string) bool {
	for key, value := range options {
		if !validSingBoxV2rayPluginOption(key, value) {
			return false
		}
	}
	return true
}

func validSingBoxV2rayPluginOption(key, value string) bool {
	switch key {
	case "mode", "host", "path":
		return true
	case "tls":
		_, err := strconv.ParseBool(value)
		return err == nil
	case "mux":
		mux, err := strconv.Atoi(value)
		return err == nil && mux >= 1
	}
	return false
}
