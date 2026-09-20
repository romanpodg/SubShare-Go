package delivery

import (
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func RenderMihomo(entries []Entry) (Generated, error) {
	return renderStructured(entries, structuredMapper{
		format:     "mihomo",
		payloadKey: "proxies",
		profile:    mihomoProxy,
		legacy:     mihomoLegacyProxy,
	})
}

func mihomoLegacyProxy(draft profileconfig.LinkConfigurationDraft, name string) map[string]any {
	proxy := map[string]any{"name": name, "type": draft.Protocol, "server": draft.Server, "port": draft.Port, "udp": true}
	applyMihomoLegacyCredentials(proxy, draft)
	applyMihomoTLS(proxy, draft)
	applyMihomoTransport(proxy, draft)
	return proxy
}

func applyMihomoLegacyCredentials(proxy map[string]any, draft profileconfig.LinkConfigurationDraft) {
	switch draft.Protocol {
	case "vless":
		proxy["uuid"] = draft.Identifier
		setIfNotEmpty(proxy, "flow", draft.Flow)
	case "vmess":
		proxy["uuid"] = draft.Identifier
		alterID, _ := strconv.Atoi(firstNonEmpty(draft.VMessAlterID, "0"))
		proxy["alterId"] = alterID
		proxy["cipher"] = firstNonEmpty(draft.VMessSecurity, "auto")
	case "trojan":
		proxy["password"] = draft.Identifier
	}
}

func applyMihomoTLS(proxy map[string]any, draft profileconfig.LinkConfigurationDraft) {
	security := strings.ToLower(strings.TrimSpace(draft.Security))
	if security != "tls" && security != "reality" {
		return
	}
	proxy["tls"] = true
	setIfNotEmpty(proxy, "servername", draft.SNI)
	setIfNotEmpty(proxy, "client-fingerprint", draft.Fingerprint)
	proxy["skip-cert-verify"] = draft.AllowInsecure
	if security == "reality" {
		applyMihomoReality(proxy, draft)
	}
}

func applyMihomoReality(proxy map[string]any, draft profileconfig.LinkConfigurationDraft) {
	reality := map[string]any{}
	setIfNotEmpty(reality, "public-key", draft.PublicKey)
	setIfNotEmpty(reality, "short-id", draft.ShortID)
	if len(reality) > 0 {
		proxy["reality-opts"] = reality
	}
}

func applyMihomoTransport(proxy map[string]any, draft profileconfig.LinkConfigurationDraft) {
	network := strings.TrimSpace(draft.Network)
	if network == "" {
		network = "tcp"
	}
	proxy["network"] = network
	switch network {
	case "ws":
		options := map[string]any{}
		setIfNotEmpty(options, "path", draft.Path)
		if draft.Host != "" {
			options["headers"] = map[string]string{"Host": draft.Host}
		}
		if len(options) > 0 {
			proxy["ws-opts"] = options
		}
	case "grpc":
		if draft.GRPCServiceName != "" {
			proxy["grpc-opts"] = map[string]string{"grpc-service-name": draft.GRPCServiceName}
		}
	}
}

func mihomoProxy(profile *profiles.Profile, name string) (map[string]any, string) {
	switch data := profile.Data.(type) {
	case profiles.ShadowsocksData:
		return mihomoShadowsocks(profile, data, name)
	case profiles.Hysteria2Data:
		return mihomoHysteria2(profile, data, name), ""
	case profiles.TUICData:
		return mihomoTUIC(profile, data, name)
	default:
		return nil, ReasonUnsupportedProtocol
	}
}

func mihomoShadowsocks(profile *profiles.Profile, data profiles.ShadowsocksData, name string) (map[string]any, string) {
	if !supportsShadowsocksMethod(shadowsocksMethods, data.Method) {
		return nil, ReasonClientVersion
	}
	proxy := map[string]any{"name": name, "type": "ss", "server": profile.Server, "port": portNumber(profile.Port), "cipher": data.Method, "password": data.Password.Reveal(), "udp": true}
	if data.Plugin != nil && !applyMihomoShadowsocksPlugin(proxy, data.Plugin) {
		return nil, ReasonPlugin
	}
	return proxy, ""
}

func applyMihomoShadowsocksPlugin(proxy map[string]any, plugin *profiles.ShadowsocksPlugin) bool {
	name, options, ok := mihomoShadowsocksPlugin(plugin)
	if !ok {
		return false
	}
	proxy["plugin"] = name
	if len(options) > 0 {
		proxy["plugin-opts"] = options
	}
	return true
}

func mihomoHysteria2(profile *profiles.Profile, data profiles.Hysteria2Data, name string) map[string]any {
	proxy := map[string]any{"name": name, "type": "hysteria2", "server": profile.Server, "password": data.Authentication.Reveal(), "sni": data.SNI, "skip-cert-verify": data.Insecure}
	if profile.Port.Kind == profiles.PortExpression {
		proxy["ports"] = profile.Port.Expression
	} else {
		proxy["port"] = portNumber(profile.Port)
	}
	setIfNotEmpty(proxy, "fingerprint", data.CertificateSHA256)
	if data.ObfuscationType != "" {
		proxy["obfs"] = data.ObfuscationType
		proxy["obfs-password"] = data.ObfuscationPassword.Reveal()
	}
	return proxy
}

func mihomoTUIC(profile *profiles.Profile, data profiles.TUICData, name string) (map[string]any, string) {
	if data.Generation != 5 {
		return nil, ReasonCompatibility
	}
	if tuicHasFieldClass(data, profiles.TUICProvenanceSingBox) {
		return nil, ReasonUnrepresentable
	}
	proxy := map[string]any{"name": name, "type": "tuic", "server": profile.Server, "port": portNumber(profile.Port), "uuid": data.UUID.Reveal(), "password": data.Password.Reveal(), "congestion-controller": data.CongestionController, "udp-relay-mode": data.UDPRelayMode}
	setIfNotEmpty(proxy, "sni", data.SNI)
	if len(data.ALPN) > 0 {
		proxy["alpn"] = append([]string(nil), data.ALPN...)
	}
	setIfTrue(proxy, "skip-cert-verify", data.SkipCertificateVerification)
	setIfTrue(proxy, "disable-sni", data.DisableSNI)
	setIfTrue(proxy, "reduce-rtt", data.ZeroRTT)
	setIfTrue(proxy, "fast-open", data.FastOpen)
	if !setDurationMilliseconds(proxy, "heartbeat-interval", data.Heartbeat) || !setDurationMilliseconds(proxy, "request-timeout", data.RequestTimeout) {
		return nil, ReasonUnrepresentable
	}
	if data.MaxOpenStreams > 0 {
		proxy["max-open-streams"] = data.MaxOpenStreams
	}
	if data.MaxUDPRelayPacketSize > 0 {
		proxy["max-udp-relay-packet-size"] = data.MaxUDPRelayPacketSize
	}
	return proxy, ""
}

// setDurationMilliseconds stores raw as whole milliseconds; an empty raw is
// simply absent, an unrepresentable one reports false.
func setDurationMilliseconds(target map[string]any, key, raw string) bool {
	if raw == "" {
		return true
	}
	value, ok := durationMilliseconds(raw)
	if ok {
		target[key] = value
	}
	return ok
}

func mihomoShadowsocksPlugin(plugin *profiles.ShadowsocksPlugin) (string, map[string]any, bool) {
	name, options, ok := shadowsocksPluginOptions(plugin)
	if !ok {
		return "", nil, false
	}
	switch name {
	case "obfs-local", "simple-obfs", "obfs":
		output, ok := mihomoObfsOptions(options)
		return "obfs", output, ok
	case "v2ray-plugin":
		output, ok := mihomoV2rayPluginOptions(options)
		return "v2ray-plugin", output, ok
	}
	return "", nil, false
}

func mihomoObfsOptions(options map[string]string) (map[string]any, bool) {
	allowed := map[string]string{"obfs": "mode", "mode": "mode", "obfs-host": "host", "host": "host"}
	output := make(map[string]any)
	for key, value := range options {
		target, found := allowed[key]
		if !found {
			return nil, false
		}
		output[target] = value
	}
	return output, true
}

func mihomoV2rayPluginOptions(options map[string]string) (map[string]any, bool) {
	output := make(map[string]any)
	for key, value := range options {
		parsed, ok := mihomoV2rayPluginOption(key, value)
		if !ok {
			return nil, false
		}
		output[key] = parsed
	}
	return output, true
}

func mihomoV2rayPluginOption(key, value string) (any, bool) {
	switch key {
	case "mode", "host", "path":
		return value, true
	case "tls", "mux":
		parsed, err := strconv.ParseBool(value)
		return parsed, err == nil
	}
	return nil, false
}
