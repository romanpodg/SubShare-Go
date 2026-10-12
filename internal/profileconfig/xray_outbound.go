package profileconfig

import (
	"encoding/json"
	"fmt"
	"strings"
)

type xrayDraftBoundary uint8

// xrayDocument is an unvalidated JSON document at the parsing boundary.
type xrayDocument string

const (
	xrayStrictDrafts xrayDraftBoundary = iota
	xrayDeliveryDrafts
)

func readXrayDraftOutbounds(raw xrayDocument) ([]any, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &root); err != nil {
		return nil, fmt.Errorf("invalid XRAY-JSON syntax")
	}
	outbounds := AsArray(root["outbounds"])
	if len(outbounds) == 0 {
		return nil, fmt.Errorf("XRAY-JSON must contain outbounds")
	}
	return outbounds, nil
}

func parseXrayDraftBoundary(raw xrayDocument, boundary xrayDraftBoundary) ([]LinkConfigurationDraft, int, error) {
	outbounds, err := readXrayDraftOutbounds(raw)
	if err != nil {
		return nil, 0, err
	}
	drafts, rejected, err := collectXrayDrafts(outbounds, boundary)
	if err != nil {
		return nil, 0, err
	}
	if boundary == xrayStrictDrafts && len(drafts) == 0 {
		return nil, 0, fmt.Errorf("XRAY-JSON contains no supported outbound")
	}
	return drafts, rejected, nil
}

func collectXrayDrafts(outbounds []any, boundary xrayDraftBoundary) ([]LinkConfigurationDraft, int, error) {
	drafts := make([]LinkConfigurationDraft, 0, len(outbounds))
	rejected := 0
	for _, outbound := range interpretXrayOutboundList(outbounds) {
		parsed, err := outbound.boundaryDrafts(outbound.index, boundary)
		if err != nil {
			if boundary == xrayStrictDrafts {
				return nil, 0, err
			}
			rejected++
			continue
		}
		drafts = append(drafts, parsed...)
	}
	return drafts, rejected, nil
}

func (outbound xrayOutbound) boundaryDrafts(index int, boundary xrayDraftBoundary) ([]LinkConfigurationDraft, error) {
	if boundary == xrayDeliveryDrafts {
		return outbound.projectedDrafts()
	}
	return outbound.strictDrafts(index)
}

// xrayOutbound interprets the shared object shape. Admission remains with each
// boundary: target uses the first endpoint, strict parsing enumerates all users,
// and delivery projection rejects individual unsupported/incomplete units.
type xrayOutbound struct {
	raw, settings, stream map[string]any
	protocol              string
	index                 int
}

func interpretXrayOutboundList(values []any) []xrayOutbound {
	outbounds := make([]xrayOutbound, 0, len(values))
	for index, value := range values {
		outbound, ok := interpretXrayOutbound(value)
		if ok {
			outbound.index = index
			outbounds = append(outbounds, outbound)
		}
	}
	return outbounds
}

func interpretXrayOutbound(value any) (xrayOutbound, bool) {
	raw, ok := AsObject(value)
	if !ok {
		return xrayOutbound{}, false
	}
	settings, _ := AsObject(raw["settings"])
	stream, _ := AsObject(raw["streamSettings"])
	return xrayOutbound{raw: raw, settings: settings, stream: stream, protocol: strings.ToLower(AnyToString(raw["protocol"]))}, true
}

func firstXrayObject(value any) map[string]any {
	items := AsArray(value)
	if len(items) == 0 {
		return nil
	}
	object, _ := AsObject(items[0])
	return object
}

func (outbound xrayOutbound) target() (host, port string, usable bool) {
	var server map[string]any
	var identifier string
	switch outbound.protocol {
	case "vless", "vmess":
		server = firstXrayObject(outbound.settings["vnext"])
		user := firstXrayObject(server["users"])
		identifier = AnyToString(user["id"])
	case "trojan":
		server = firstXrayObject(outbound.settings["servers"])
		identifier = AnyToString(server["password"])
	default:
		return "", "", false
	}
	host = AnyToString(server["address"])
	return host, AnyToPort(server["port"]), host != "" && identifier != ""
}

func (outbound xrayOutbound) isNativeProxy() bool {
	switch outbound.protocol {
	case "vless", "vmess", "trojan":
		return true
	default:
		return false
	}
}

func (outbound xrayOutbound) strictDrafts(index int) ([]LinkConfigurationDraft, error) {
	if !outbound.isNativeProxy() {
		return nil, nil
	}
	base := xrayStreamDraft(outbound.protocol, outbound.raw, outbound.stream)
	if outbound.protocol == "trojan" {
		return xrayTrojanDrafts(base, outbound.settings, index)
	}
	return xrayVnextDrafts(base, outbound.settings, index, outbound.protocol)
}

func (outbound xrayOutbound) projectedDrafts() ([]LinkConfigurationDraft, error) {
	switch outbound.protocol {
	case "vless", "vmess", "trojan":
		// Retain projection's existing JSON normalization and local error index.
		encoded, _ := json.Marshal(map[string]any{"outbounds": []any{outbound.raw}})
		return ParseXrayJSONDrafts(string(encoded))
	case "shadowsocks":
		return projectXrayShadowsocks(outbound.raw)
	case "hysteria", "hysteria2":
		return projectXrayHysteria2(outbound.raw)
	case "tuic":
		return projectXrayTUIC(outbound.raw)
	default:
		return nil, nil
	}
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
		outbound, ok := interpretXrayOutbound(outboundRaw)
		if !ok {
			continue
		}
		host, port, usable := outbound.target()
		if usable {
			return host, port, nil
		}
	}

	return "", "", fmt.Errorf("XRAY-JSON must contain outbound with protocol vless/vmess/trojan and valid server settings")
}

// ParseXrayJSONDrafts parses all supported outbounds in XRAY-JSON into LinkConfigurationDraft slice.
func ParseXrayJSONDrafts(raw string) ([]LinkConfigurationDraft, error) {
	drafts, _, err := parseXrayDraftBoundary(xrayDocument(raw), xrayStrictDrafts)
	return drafts, err
}

// ProjectXrayJSONDrafts parses each supported proxy outbound independently for
// delivery projection. Invalid proxy outbounds are counted and skipped, while
// routing helpers such as freedom and blackhole are ignored.
func ProjectXrayJSONDrafts(raw string) ([]LinkConfigurationDraft, int, error) {
	return parseXrayDraftBoundary(xrayDocument(raw), xrayDeliveryDrafts)
}

// ValidateXrayJSONConfiguration validates XRAY-JSON string.
func ValidateXrayJSONConfiguration(raw string) error {
	if _, _, err := ParseXrayJSONTarget(raw); err != nil {
		return err
	}
	return nil
}
