package sources

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func xrayVersionValue(value any) (int, bool) {
	raw := profileconfig.AnyToString(value)
	if raw == "" {
		return 0, false
	}
	version, err := strconv.Atoi(raw)
	return version, err == nil
}

func onlyObjectKeys(object map[string]any, allowed ...string) bool {
	allow := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allow[key] = struct{}{}
	}
	for key := range object {
		if _, ok := allow[key]; !ok {
			return false
		}
	}
	return true
}

func xrayStringArray(object map[string]any, key string) ([]string, bool) {
	raw, present := object[key]
	if !present {
		return nil, true
	}
	values := profileconfig.AsArray(raw)
	if values == nil {
		return nil, false
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		text = strings.TrimSpace(text)
		if !ok || text == "" {
			return nil, false
		}
		result = append(result, text)
	}
	return result, true
}

// xrayTrimmedString reads an optional string field: absent is fine, but a
// present value must be a non-blank string.
func xrayTrimmedString(object map[string]any, key string) (string, bool, bool) {
	raw, present := object[key]
	if !present {
		return "", false, true
	}
	value, ok := raw.(string)
	value = strings.TrimSpace(value)
	return value, true, ok && value != ""
}

// xrayRequiredString reads a mandatory string field; ok is false when the
// field is missing, not a string or blank.
func xrayRequiredString(object map[string]any, key string) (string, bool) {
	value, ok := object[key].(string)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

// xrayOptionalObject reads an optional object field: absent is fine, but a
// present value must be an object using only the allowed keys.
func xrayOptionalObject(parent map[string]any, key string, allowed ...string) (map[string]any, bool) {
	raw, present := parent[key]
	if !present {
		return nil, true
	}
	object, ok := profileconfig.AsObject(raw)
	if !ok {
		return nil, false
	}
	return object, onlyObjectKeys(object, allowed...)
}

func singleXrayHysteriaOutbound(object map[string]any) (map[string]any, bool) {
	var hysteriaOutbound map[string]any
	for _, outboundRaw := range profileconfig.AsArray(object["outbounds"]) {
		outbound, ok := profileconfig.AsObject(outboundRaw)
		if !ok {
			return nil, false
		}
		switch strings.ToLower(profileconfig.AnyToString(outbound["protocol"])) {
		case "hysteria":
			if hysteriaOutbound != nil {
				return nil, false
			}
			hysteriaOutbound = outbound
		case "freedom", "blackhole", "dns":
			// Routing helpers are not additional subscription profiles.
		case "":
			return nil, false
		default:
			// A multi-profile Xray document must stay in the existing lossless
			// XRAY-JSON path instead of being reduced to one Hysteria profile.
			return nil, false
		}
	}
	return hysteriaOutbound, hysteriaOutbound != nil
}

func parseXrayHysteria2Object(object map[string]any, raw string, lineIndex int, fingerprintKeys [][]byte) (*ParsedKey, *ImportItem, bool, error) {
	outbound, singleProfile := singleXrayHysteriaOutbound(object)
	if !singleProfile {
		return nil, nil, false, nil
	}
	identity := xrayOutboundIdentity(object, outbound, "hysteria")
	settings, _ := profileconfig.AsObject(outbound["settings"])
	stream, _ := profileconfig.AsObject(outbound["streamSettings"])
	hysteriaSettings, _ := profileconfig.AsObject(stream["hysteriaSettings"])

	if protocol, errorCode := xrayHysteriaVersionRejection(settings, hysteriaSettings); errorCode != "" {
		identity.Protocol = protocol
		item, err := safeRejectedXrayJSONItem(raw, lineIndex, identity, StatusUnsupported, errorCode, fingerprintKeys)
		return nil, &item, true, err
	}
	identity.Protocol = "hysteria2"
	label := xrayJSONLabel(identity.Label, lineIndex)
	profile, serialized, ok := xrayHysteria2Profile(outbound, label)
	if !ok {
		item, err := safeRejectedXrayJSONItem(raw, lineIndex, identity, StatusRejected, "invalid_hysteria2_json", fingerprintKeys)
		return nil, &item, true, err
	}
	key, err := xrayHysteria2ParsedKey(profile, serialized, label, raw, lineIndex, fingerprintKeys)
	if err != nil {
		return nil, nil, true, err
	}
	item := safeExternalItem(*key, StatusAccepted, "")
	return key, &item, true, nil
}

// xrayVersionMarker is one of the two Hysteria version fields an Xray
// document may carry.
type xrayVersionMarker struct {
	present bool
	valid   bool
	version int
}

func xrayVersionMarkerOf(object map[string]any, key string) xrayVersionMarker {
	raw, present := object[key]
	version, valid := xrayVersionValue(raw)
	return xrayVersionMarker{present: present, valid: valid, version: version}
}

// invalid reports a marker that is present but not numeric.
func (m xrayVersionMarker) invalid() bool { return m.present && !m.valid }

// allows reports whether the marker is absent or names the given version.
func (m xrayVersionMarker) allows(version int) bool { return !m.present || m.version == version }

// xrayHysteriaVersionRejection inspects both Hysteria version markers and
// returns the identity protocol plus rejection code for anything other than
// an unambiguous version 2. An empty code means version 2 was confirmed.
func xrayHysteriaVersionRejection(settings, hysteriaSettings map[string]any) (string, string) {
	settingsMarker := xrayVersionMarkerOf(settings, "version")
	streamMarker := xrayVersionMarkerOf(hysteriaSettings, "version")
	switch {
	case !settingsMarker.present && !streamMarker.present:
		return "hysteria-unknown", "ambiguous_hysteria_version"
	case settingsMarker.invalid() || streamMarker.invalid():
		return "hysteria-unknown", "unsupported_hysteria_version"
	case settingsMarker.allows(2) && streamMarker.allows(2):
		return "", ""
	case settingsMarker.allows(1) && streamMarker.allows(1):
		return "hysteria", "unsupported_hysteria_v1"
	default:
		return "hysteria-unknown", "unsupported_hysteria_version"
	}
}

// xrayHysteria2Profile converts a single-outbound Hysteria 2 document into a
// parsed hysteria2:// profile. ok is false for any shape violation.
func xrayHysteria2Profile(outbound map[string]any, label string) (*profiles.Profile, profiles.SerializationResult, bool) {
	profileURI, ok := xrayHysteria2URI(outbound, label)
	if !ok {
		return nil, profiles.SerializationResult{}, false
	}
	profile, err := profiles.Parse(profileURI)
	if err != nil {
		return nil, profiles.SerializationResult{}, false
	}
	serialized, err := profiles.Serialize(profile, profiles.CanonicalSerialization)
	if err != nil {
		return nil, profiles.SerializationResult{}, false
	}
	return profile, serialized, true
}

// xrayHysteria2URI builds the equivalent hysteria2:// URI for a validated
// outbound: credentials and target from settings, TLS and obfuscation from
// the stream block.
func xrayHysteria2URI(outbound map[string]any, label string) (string, bool) {
	settings, _ := profileconfig.AsObject(outbound["settings"])
	stream, _ := profileconfig.AsObject(outbound["streamSettings"])
	hysteriaSettings, _ := profileconfig.AsObject(stream["hysteriaSettings"])
	host, hostOK := xrayRequiredString(settings, "address")
	auth, authOK := xrayRequiredString(hysteriaSettings, "auth")
	port := strings.TrimSpace(profileconfig.AnyToString(settings["port"]))
	if !hostOK || !authOK {
		return "", false
	}
	if port == "" {
		return "", false
	}
	if !xrayHysteria2ShapeValid(outbound, settings, stream, hysteriaSettings) {
		return "", false
	}
	query, hopPorts, ok := xrayHysteria2Query(stream)
	if !ok {
		return "", false
	}
	if hopPorts != "" {
		port = hopPorts
	}
	profileURI := "hysteria2://" + url.User(auth).String() + "@" + bracketHost(host) + ":" + port
	if encodedQuery := query.Encode(); encodedQuery != "" {
		profileURI += "?" + encodedQuery
	}
	return profileURI + "#" + url.PathEscape(label), true
}

// xrayHysteria2Query collects the TLS and finalmask query parameters and
// returns the udpHop port expression when present.
func xrayHysteria2Query(stream map[string]any) (url.Values, string, bool) {
	query := url.Values{}
	if !xrayHysteria2TLSQuery(stream, query) {
		return nil, "", false
	}
	hopPorts, ok := xrayHysteria2FinalMaskQuery(stream, query)
	return query, hopPorts, ok
}

func xrayHysteria2ShapeValid(outbound, settings, stream, hysteriaSettings map[string]any) bool {
	if !xrayHysteriaTransportValid(stream) {
		return false
	}
	if !strings.EqualFold(profileconfig.AnyToString(stream["security"]), "tls") {
		return false
	}
	allowedKeys := []struct {
		object  map[string]any
		allowed []string
	}{
		{outbound, []string{"tag", "protocol", "settings", "streamSettings"}},
		{settings, []string{"version", "address", "port"}},
		{hysteriaSettings, []string{"version", "auth"}},
		{stream, []string{"method", "network", "security", "hysteriaSettings", "tlsSettings", "finalmask"}},
	}
	for _, check := range allowedKeys {
		if !onlyObjectKeys(check.object, check.allowed...) {
			return false
		}
	}
	return true
}

// xrayHysteriaTransportValid requires method and/or network to name the
// hysteria transport and nothing else.
func xrayHysteriaTransportValid(stream map[string]any) bool {
	method := strings.ToLower(profileconfig.AnyToString(stream["method"]))
	network := strings.ToLower(profileconfig.AnyToString(stream["network"]))
	if method == "" && network == "" {
		return false
	}
	return namesHysteriaOrEmpty(method) && namesHysteriaOrEmpty(network)
}

func namesHysteriaOrEmpty(value string) bool { return value == "" || value == "hysteria" }

func xrayHysteria2TLSQuery(stream map[string]any, query url.Values) bool {
	tlsSettings, ok := xrayOptionalObject(stream, "tlsSettings", "serverName", "pinnedPeerCertSha256", "allowInsecure", "alpn", "fingerprint")
	if !ok {
		return false
	}
	for _, field := range [...]struct{ key, param string }{{"serverName", "sni"}, {"pinnedPeerCertSha256", "pinSHA256"}, {"fingerprint", "fp"}} {
		if !setXrayQueryString(query, tlsSettings, field.key, field.param) {
			return false
		}
	}
	if !setXrayInsecureQuery(query, tlsSettings) {
		return false
	}
	alpnValues, alpnOK := xrayStringArray(tlsSettings, "alpn")
	if !alpnOK {
		return false
	}
	for _, alpn := range alpnValues {
		query.Add("alpn", alpn)
	}
	return true
}

// setXrayQueryString copies an optional string field into query under param.
func setXrayQueryString(query url.Values, object map[string]any, key, param string) bool {
	value, present, ok := xrayTrimmedString(object, key)
	if !ok {
		return false
	}
	if present {
		query.Set(param, value)
	}
	return true
}

func setXrayInsecureQuery(query url.Values, tlsSettings map[string]any) bool {
	insecureRaw, present := tlsSettings["allowInsecure"]
	if !present {
		return true
	}
	insecure, ok := insecureRaw.(bool)
	if !ok {
		return false
	}
	if insecure {
		query.Set("insecure", "1")
	} else {
		query.Set("insecure", "0")
	}
	return true
}

// xrayHysteria2FinalMaskQuery validates the finalmask block, adds salamander
// obfuscation to query and returns the udpHop port expression when present.
func xrayHysteria2FinalMaskQuery(stream map[string]any, query url.Values) (string, bool) {
	finalMask, ok := xrayOptionalObject(stream, "finalmask", "quicParams", "udp")
	if !ok {
		return "", false
	}
	hopPorts, ok := xrayHysteria2HopPorts(finalMask)
	if !ok {
		return "", false
	}
	return hopPorts, xrayHysteria2ObfsQuery(finalMask, query)
}

// xrayHysteria2ObfsQuery validates the optional udp mask list and adds
// salamander obfuscation to query. An empty list carries no obfuscation.
func xrayHysteria2ObfsQuery(finalMask map[string]any, query url.Values) bool {
	masksRaw, present := finalMask["udp"]
	if !present {
		return true
	}
	masks := profileconfig.AsArray(masksRaw)
	if masks == nil {
		return false
	}
	if len(masks) == 0 {
		return true
	}
	if len(masks) != 1 {
		return false
	}
	password, ok := salamanderPassword(masks[0])
	if !ok {
		return false
	}
	query.Set("obfs", "salamander")
	query.Set("obfs-password", password)
	return true
}

// salamanderPassword accepts exactly {"type":"salamander","settings":{"password":...}}.
func salamanderPassword(maskRaw any) (string, bool) {
	mask, ok := profileconfig.AsObject(maskRaw)
	if !ok || !onlyObjectKeys(mask, "type", "settings") {
		return "", false
	}
	maskSettings, ok := profileconfig.AsObject(mask["settings"])
	if !ok || !onlyObjectKeys(maskSettings, "password") {
		return "", false
	}
	if profileconfig.AnyToString(mask["type"]) != "salamander" {
		return "", false
	}
	return profileconfig.AnyToString(maskSettings["password"]), true
}

func xrayHysteria2HopPorts(finalMask map[string]any) (string, bool) {
	quicParams, ok := profileconfig.AsObject(finalMask["quicParams"])
	if !ok {
		_, present := finalMask["quicParams"]
		return "", !present
	}
	if !onlyObjectKeys(quicParams, "udpHop") {
		return "", false
	}
	udpHop, hopOK := profileconfig.AsObject(quicParams["udpHop"])
	if !hopOK {
		_, present := quicParams["udpHop"]
		return "", !present
	}
	ports := profileconfig.AnyToString(udpHop["ports"])
	if !onlyObjectKeys(udpHop, "ports") || ports == "" {
		return "", false
	}
	return ports, true
}

func xrayHysteria2ParsedKey(profile *profiles.Profile, serialized profiles.SerializationResult, label, raw string, lineIndex int, fingerprintKeys [][]byte) (*ParsedKey, error) {
	itemRef, err := buildExternalItemRef(raw, lineIndex, fingerprintKeys)
	if err != nil {
		return nil, err
	}
	fingerprints := make([]string, 0, len(fingerprintKeys))
	for _, fingerprintKey := range fingerprintKeys {
		fingerprint, fingerprintErr := profiles.Fingerprint(profile, fingerprintKey)
		if fingerprintErr != nil {
			return nil, fingerprintErr
		}
		fingerprints = append(fingerprints, fingerprint)
	}
	metadata := profile.SafeMetadata()
	key := &ParsedKey{
		Label: label, URL: serialized.URI.Reveal(), Scheme: "hysteria2", Protocol: "hysteria2",
		Host: metadata.Server, Port: metadata.Port, Ref: KeyRef(serialized.URI.Reveal()),
		ItemRef: itemRef, LineIndex: lineIndex, Compatibility: string(metadata.Capabilities.Status),
		ProfileSchemaVersion: ExternalProfileSchemaVersion, FingerprintCandidates: fingerprints,
		WarningCodes: profileWarningCodes(profile), InitialStatus: StatusAccepted,
	}
	if len(fingerprints) > 0 {
		key.Fingerprint = fingerprints[0]
	}
	return key, nil
}
