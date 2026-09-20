package sources

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
)

func ExtractJSONLabel(root map[string]any, fallback string) string {
	candidates := [...]string{
		trimmedStringField(root, "remarks"),
		trimmedStringField(objectField(root, "meta"), "serverDescription"),
		nonGenericOutboundTag(root),
		nonGenericOutboundTag(firstObject(root["outbounds"])),
	}
	for _, candidate := range candidates {
		if candidate != "" {
			return candidate
		}
	}
	return fallback
}

// trimmedStringField returns the trimmed string at key, or "" when the field
// is absent or not a string.
func trimmedStringField(object map[string]any, key string) string {
	value, _ := object[key].(string)
	return strings.TrimSpace(value)
}

func objectField(object map[string]any, key string) map[string]any {
	value, _ := object[key].(map[string]any)
	return value
}

func firstObject(value any) map[string]any {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	first, _ := items[0].(map[string]any)
	return first
}

// nonGenericOutboundTag returns the object's tag unless it is a routing
// placeholder such as "proxy" that carries no display value.
func nonGenericOutboundTag(object map[string]any) string {
	raw, _ := object["tag"].(string)
	if profileconfig.IsGenericXrayOutboundTag(raw) {
		return ""
	}
	return strings.TrimSpace(raw)
}

// xrayJSONLabel trims label and falls back to the positional JSON name.
func xrayJSONLabel(label string, lineIndex int) string {
	return labelOrDefault(label, "JSON %03d", lineIndex)
}

func isSIP008ProfileObject(object map[string]any) bool {
	_, passwordPresent := object["password"]
	return profileconfig.AnyToString(object["server"]) != "" &&
		profileconfig.AnyToString(object["server_port"]) != "" &&
		profileconfig.AnyToString(object["method"]) != "" &&
		passwordPresent
}

type xrayJSONIdentity struct {
	Protocol string
	Label    string
	Host     string
	Port     string
}

// xrayOutboundIdentity describes one outbound for safe reporting: the
// document label (or the outbound tag) plus the first target of the protocol.
func xrayOutboundIdentity(object, outbound map[string]any, protocol string) xrayJSONIdentity {
	identity := xrayJSONIdentity{Protocol: protocol, Label: strings.TrimSpace(ExtractJSONLabel(object, ""))}
	if identity.Label == "" {
		identity.Label = profileconfig.AnyToString(outbound["tag"])
	}
	settings, _ := profileconfig.AsObject(outbound["settings"])
	identity.Host, identity.Port = xrayOutboundTarget(protocol, settings)
	return identity
}

func xrayOutboundTarget(protocol string, settings map[string]any) (string, string) {
	switch protocol {
	case "vless", "vmess":
		return firstXrayNodeTarget(settings["vnext"])
	case "trojan":
		return firstXrayNodeTarget(settings["servers"])
	case "hysteria":
		return profileconfig.AnyToString(settings["address"]), profileconfig.AnyToString(settings["port"])
	}
	return "", ""
}

func firstXrayNodeTarget(raw any) (string, string) {
	nodes := profileconfig.AsArray(raw)
	if len(nodes) == 0 {
		return "", ""
	}
	node, ok := profileconfig.AsObject(nodes[0])
	if !ok {
		return "", ""
	}
	return profileconfig.AnyToString(node["address"]), profileconfig.AnyToString(node["port"])
}

func firstXrayJSONIdentity(object map[string]any) (xrayJSONIdentity, bool) {
	for _, outboundRaw := range profileconfig.AsArray(object["outbounds"]) {
		outbound, ok := profileconfig.AsObject(outboundRaw)
		if !ok {
			continue
		}
		protocol := strings.ToLower(profileconfig.AnyToString(outbound["protocol"]))
		if protocol == "" {
			continue
		}
		return xrayOutboundIdentity(object, outbound, protocol), true
	}
	return xrayJSONIdentity{}, false
}

func safeRejectedXrayJSONItem(raw string, lineIndex int, identity xrayJSONIdentity, status, errorCode string, fingerprintKeys [][]byte) (ImportItem, error) {
	itemRef, err := buildExternalItemRef(raw, lineIndex, fingerprintKeys)
	if err != nil {
		return ImportItem{}, err
	}
	label := xrayJSONLabel(identity.Label, lineIndex)
	return ImportItem{
		ItemRef: itemRef, LineIndex: lineIndex, Protocol: identity.Protocol, Scheme: "xray-json",
		DisplayName: label, Label: label, Host: identity.Host, Port: identity.Port,
		Compatibility: "unsupported", Status: status, Warnings: []string{}, ErrorCode: errorCode,
		URLShort: safeEndpointSummary(identity.Protocol, identity.Host, identity.Port),
	}, nil
}

func parseExternalJSONBody(body string, firstItemIndex int, fingerprintKeys [][]byte) ([]ParsedKey, []ImportItem, error) {
	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, nil, fmt.Errorf("invalid_json_subscription")
	}
	collector := &externalJSONCollector{
		keys:            make([]ParsedKey, 0, 16),
		items:           make([]ImportItem, 0, 16),
		seen:            make(map[string]struct{}),
		fingerprintKeys: fingerprintKeys,
	}
	switch typed := parsed.(type) {
	case map[string]any:
		if err := collector.appendObject(typed, firstItemIndex); err != nil {
			return nil, nil, err
		}
	case []any:
		if len(typed) > MaxImportItems {
			return nil, nil, fmt.Errorf("too_many_source_items")
		}
		for index, value := range typed {
			if err := collector.appendValue(value, firstItemIndex+index); err != nil {
				return nil, nil, err
			}
		}
	default:
		return nil, nil, fmt.Errorf("invalid_json_subscription")
	}
	return collector.keys, collector.items, nil
}

// externalJSONCollector accumulates the keys and items produced while walking
// one JSON subscription document.
type externalJSONCollector struct {
	keys            []ParsedKey
	items           []ImportItem
	seen            map[string]struct{}
	fingerprintKeys [][]byte
}

func invalidXrayJSONItem(itemRef string, lineIndex int) ImportItem {
	return ImportItem{ItemRef: itemRef, LineIndex: lineIndex, Protocol: "xray-json", Scheme: "xray-json", Compatibility: "legacy", Status: StatusRejected, Warnings: []string{}, ErrorCode: "invalid_xray_json", URLShort: "xray-json"}
}

func sip008Item(itemRef string, lineIndex int) ImportItem {
	return ImportItem{
		ItemRef: itemRef, LineIndex: lineIndex, Protocol: "shadowsocks", Scheme: "sip008",
		Compatibility: "unsupported", Status: StatusUnsupported, Warnings: []string{},
		ErrorCode: "unsupported_sip008_format", URLShort: "shadowsocks",
	}
}

func (c *externalJSONCollector) appendValue(value any, lineIndex int) error {
	object, ok := value.(map[string]any)
	if !ok {
		itemRef, err := buildExternalItemRef("invalid-json-item", lineIndex, c.fingerprintKeys)
		if err != nil {
			return err
		}
		c.items = append(c.items, invalidXrayJSONItem(itemRef, lineIndex))
		return nil
	}
	return c.appendObject(object, lineIndex)
}

func (c *externalJSONCollector) appendObject(obj map[string]any, lineIndex int) error {
	encoded, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("invalid_json_subscription")
	}
	raw := string(encoded)
	itemRef, err := buildExternalItemRef(raw, lineIndex, c.fingerprintKeys)
	if err != nil {
		return err
	}
	if isSIP008ProfileObject(obj) {
		c.items = append(c.items, sip008Item(itemRef, lineIndex))
		return nil
	}
	if handled, hysteriaErr := c.appendHysteria2(obj, raw, lineIndex); handled {
		return hysteriaErr
	}
	return c.appendXrayDrafts(obj, raw, itemRef, lineIndex)
}

// appendHysteria2 folds a single-outbound Hysteria document into the result
// and reports whether the object was one.
func (c *externalJSONCollector) appendHysteria2(obj map[string]any, raw string, lineIndex int) (bool, error) {
	key, item, handled, err := parseXrayHysteria2Object(obj, raw, lineIndex, c.fingerprintKeys)
	if !handled {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	if key == nil {
		c.items = append(c.items, *item)
		return true, nil
	}
	appendExternalParsedItem(&c.keys, &c.items, c.seen, *key)
	return true, nil
}

// appendXrayDrafts keeps a lossless Xray document as one legacy xray-json key
// when it yields drafts, and classifies it otherwise.
func (c *externalJSONCollector) appendXrayDrafts(obj map[string]any, raw, itemRef string, lineIndex int) error {
	drafts, parseErr := profileconfig.ParseXrayJSONDrafts(raw)
	if parseErr != nil || len(drafts) == 0 {
		item, itemErr := c.undraftableXrayJSONItem(obj, raw, itemRef, lineIndex)
		if itemErr != nil {
			return itemErr
		}
		c.items = append(c.items, item)
		return nil
	}
	label := xrayJSONLabel(ExtractJSONLabel(obj, ""), lineIndex)
	host, port, _ := profileconfig.ParseXrayJSONTarget(raw)
	key := ParsedKey{Label: label, URL: raw, Scheme: "xray-json", Protocol: "xray-json", Host: host, Port: port, Ref: KeyRef(raw), ItemRef: itemRef, LineIndex: lineIndex, Compatibility: "legacy", InitialStatus: StatusAccepted}
	appendExternalParsedItem(&c.keys, &c.items, c.seen, key)
	return nil
}

// undraftableXrayJSONItem classifies an Xray document that yielded no drafts:
// a recognisable vless/vmess/trojan outbound is a rejected malformed profile,
// any other identified protocol is unsupported, and the rest is invalid JSON.
func (c *externalJSONCollector) undraftableXrayJSONItem(obj map[string]any, raw, itemRef string, lineIndex int) (ImportItem, error) {
	identity, identified := firstXrayJSONIdentity(obj)
	if !identified {
		return invalidXrayJSONItem(itemRef, lineIndex), nil
	}
	status := StatusUnsupported
	errorCode := "unsupported_xray_protocol"
	if isXrayURIProtocol(identity.Protocol) {
		status = StatusRejected
		errorCode = "invalid_" + identity.Protocol + "_json"
	}
	return safeRejectedXrayJSONItem(raw, lineIndex, identity, status, errorCode, c.fingerprintKeys)
}

// isXrayURIProtocol reports whether an Xray outbound protocol also has a URI
// profile form, so a malformed document is a rejected profile rather than an
// unsupported one.
func isXrayURIProtocol(protocol string) bool {
	switch protocol {
	case "vless", "vmess", "trojan":
		return true
	}
	return false
}
