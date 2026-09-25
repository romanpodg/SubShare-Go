package delivery

const (
	TargetMihomoVersion      = "1.19.28"
	TargetSingBoxVersion     = "1.13.12"
	TargetXrayMinimumVersion = "26.3.27"
	TargetXrayCurrentVersion = "26.7.28"
)

type CapabilitySupport string

const (
	CapabilitySupported     CapabilitySupport = "supported"
	CapabilityUnsupported   CapabilitySupport = "unsupported"
	CapabilityConditional   CapabilitySupport = "conditionally_supported"
	CapabilityCompatibility CapabilitySupport = "compatibility_only"
)

type OutputCapability struct {
	Status                  CapabilitySupport `json:"status"`
	ReasonCode              string            `json:"reason_code,omitempty"`
	TargetVersion           string            `json:"target_version,omitempty"`
	MinimumVersion          string            `json:"minimum_version,omitempty"`
	SyntaxValidation        string            `json:"syntax_validation,omitempty"`
	RuntimeInteroperability string            `json:"runtime_interoperability,omitempty"`
}

type ProtocolCapability struct {
	Protocol   string                      `json:"protocol"`
	Generation string                      `json:"generation"`
	Outputs    map[string]OutputCapability `json:"outputs"`
}

// clientTarget pins a structured generator to the client release it was
// validated against and the oldest release it is expected to work with.
type clientTarget struct{ version, minimum string }

var (
	mihomoTarget  = clientTarget{TargetMihomoVersion, TargetMihomoVersion}
	singBoxTarget = clientTarget{TargetSingBoxVersion, TargetSingBoxVersion}
	xrayTarget    = clientTarget{TargetXrayCurrentVersion, TargetXrayMinimumVersion}
)

func structuralCapability(version string) OutputCapability {
	return OutputCapability{Status: CapabilitySupported, TargetVersion: version, SyntaxValidation: "structurally_generated", RuntimeInteroperability: "not_tested"}
}

func clientSupportedCapability(target clientTarget) OutputCapability {
	return OutputCapability{Status: CapabilitySupported, TargetVersion: target.version, MinimumVersion: target.minimum, SyntaxValidation: "official_binary", RuntimeInteroperability: "not_tested"}
}

func conditionalCapability(reason string, target clientTarget) OutputCapability {
	return OutputCapability{Status: CapabilityConditional, ReasonCode: reason, TargetVersion: target.version, MinimumVersion: target.minimum, SyntaxValidation: "official_binary", RuntimeInteroperability: "not_tested"}
}

func probeCapability(reason string) OutputCapability {
	return OutputCapability{Status: CapabilityConditional, ReasonCode: reason, TargetVersion: "subshare-current", SyntaxValidation: "address_probe_only", RuntimeInteroperability: "not_tested"}
}

func unsupportedCapability(reason, version string) OutputCapability {
	return OutputCapability{Status: CapabilityUnsupported, ReasonCode: reason, TargetVersion: version}
}

func compatibilityCapability(reason string) OutputCapability {
	return OutputCapability{Status: CapabilityCompatibility, ReasonCode: reason}
}

func legacyProtocolCapability(protocol string) ProtocolCapability {
	return ProtocolCapability{Protocol: protocol, Generation: "current", Outputs: map[string]OutputCapability{
		"plain": structuralCapability("sip-uri"), "base64": structuralCapability("whole-body-standard-base64"),
		"mihomo": clientSupportedCapability(mihomoTarget), "sing-box": clientSupportedCapability(singBoxTarget),
		"xray-json": clientSupportedCapability(xrayTarget), "structured-editing": structuralCapability("subshare-current"),
		"connectivity-probe": probeCapability("tcp_reachability_only"),
		"compatibility-raw":  unsupportedCapability(ReasonUnsupportedProtocol, ""),
	}}
}

// CapabilityMatrix is the single backend source of truth for
// delivery, editing, and probing claims. Structured generators are pinned to
// explicit client schemas; an absent generator is never interpreted as support.
func CapabilityMatrix() []ProtocolCapability {
	return []ProtocolCapability{
		legacyProtocolCapability("vless"), legacyProtocolCapability("vmess"), legacyProtocolCapability("trojan"),
		{Protocol: "shadowsocks", Generation: "sip002/sip022", Outputs: map[string]OutputCapability{
			"plain": structuralCapability("sip002/sip022"), "base64": structuralCapability("whole-body-standard-base64"),
			"mihomo":             conditionalCapability("method_or_plugin_dependent", mihomoTarget),
			"sing-box":           conditionalCapability("method_or_plugin_dependent", singBoxTarget),
			"xray-json":          conditionalCapability("plugin_free_only", xrayTarget),
			"structured-editing": unsupportedCapability("frontend_editor_deferred", ""),
			"connectivity-probe": probeCapability("tcp_reachability_only"),
			"compatibility-raw":  unsupportedCapability(ReasonUnsupportedProtocol, ""),
		}},
		{Protocol: "hysteria2", Generation: "2", Outputs: map[string]OutputCapability{
			"plain": structuralCapability("hysteria2-uri"), "base64": structuralCapability("whole-body-standard-base64"),
			"mihomo":             conditionalCapability("field_and_version_dependent", mihomoTarget),
			"sing-box":           conditionalCapability("field_and_version_dependent", singBoxTarget),
			"xray-json":          conditionalCapability("obfuscation_or_extension_dependent", xrayTarget),
			"structured-editing": unsupportedCapability("frontend_editor_deferred", ""),
			"connectivity-probe": probeCapability("dns_only_udp_quic_probe_unavailable"),
			"compatibility-raw":  unsupportedCapability(ReasonUnsupportedProtocol, ""),
		}},
		{Protocol: "tuic", Generation: "5", Outputs: map[string]OutputCapability{
			"plain": structuralCapability("compatibility-uri"), "base64": structuralCapability("whole-body-standard-base64"),
			"mihomo":             conditionalCapability("provenance_and_field_dependent", mihomoTarget),
			"sing-box":           conditionalCapability("provenance_and_field_dependent", singBoxTarget),
			"xray-json":          unsupportedCapability(ReasonUnsupportedProtocol, TargetXrayCurrentVersion),
			"structured-editing": unsupportedCapability("frontend_editor_deferred", ""),
			"connectivity-probe": probeCapability("dns_only_udp_quic_probe_unavailable"),
			"compatibility-raw":  unsupportedCapability(ReasonUnsupportedProtocol, ""),
		}},
		{Protocol: "tuic", Generation: "4", Outputs: map[string]OutputCapability{
			"plain": compatibilityCapability("raw_delivery_only"), "base64": compatibilityCapability("raw_delivery_only"),
			"mihomo":             unsupportedCapability(ReasonCompatibility, TargetMihomoVersion),
			"sing-box":           unsupportedCapability(ReasonCompatibility, TargetSingBoxVersion),
			"xray-json":          unsupportedCapability(ReasonCompatibility, TargetXrayCurrentVersion),
			"structured-editing": unsupportedCapability(ReasonCompatibility, ""),
			"connectivity-probe": probeCapability("dns_only_udp_quic_probe_unavailable"),
			"compatibility-raw":  compatibilityCapability("raw_delivery_only"),
		}},
	}
}
