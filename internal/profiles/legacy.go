package profiles

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

type vlessAdapter struct{}
type trojanAdapter struct{}
type vmessAdapter struct{}

func (vlessAdapter) Protocol() Protocol  { return ProtocolVLESS }
func (vlessAdapter) Schemes() []string   { return []string{"vless"} }
func (trojanAdapter) Protocol() Protocol { return ProtocolTrojan }
func (trojanAdapter) Schemes() []string  { return []string{"trojan"} }
func (vmessAdapter) Protocol() Protocol  { return ProtocolVMess }
func (vmessAdapter) Schemes() []string   { return []string{"vmess"} }

func (vlessAdapter) Parse(raw string) (*Profile, error) {
	return parseUserInfoLink(raw, ProtocolVLESS)
}

func (trojanAdapter) Parse(raw string) (*Profile, error) {
	return parseUserInfoLink(raw, ProtocolTrojan)
}

func parseUserInfoLink(raw string, protocol Protocol) (*Profile, error) {
	components, err := splitStandardURI(raw, protocol)
	if err != nil {
		return nil, err
	}
	if components.scheme != string(protocol) {
		return nil, newError(ErrorUnsupportedScheme, protocol, "scheme")
	}
	authority, err := parseAuthority(components.authority, protocol, "443", false)
	if err != nil {
		return nil, err
	}
	if !authority.hasUserInfo || authority.rawUserInfo == "" {
		return nil, newError(ErrorMissingCredential, protocol, "credential")
	}
	credential, err := decodeUserInfo(authority.rawUserInfo, protocol, "credential")
	if err != nil {
		return nil, err
	}
	if credential == "" {
		return nil, newError(ErrorMissingCredential, protocol, "credential")
	}
	parameters, err := parseQuery(components.rawQuery, protocol)
	if err != nil {
		return nil, err
	}
	profile := &Profile{
		Protocol:               protocol,
		Server:                 authority.host,
		Port:                   authority.port,
		DisplayName:            components.fragment,
		OriginalURI:            NewSensitiveValue(raw),
		QueryParameters:        parameters,
		UnknownQueryParameters: append([]QueryParameter(nil), parameters...),
		Warnings:               duplicateWarnings(parameters, nil),
		Capabilities:           fullCapabilities("legacy-share-link"),
	}
	if protocol == ProtocolVLESS {
		profile.Data = VLESSData{UUID: NewSensitiveValue(credential)}
	} else {
		profile.Data = TrojanData{Password: NewSensitiveValue(credential)}
	}
	return profile, nil
}

func validateUserInfoLink(profile *Profile, protocol Protocol) error {
	if profile == nil || profile.Protocol != protocol {
		return newError(ErrorInvalidProfile, protocol, "profile")
	}
	credentialSet := false
	if protocol == ProtocolVLESS {
		data, ok := profile.Data.(VLESSData)
		credentialSet = ok && data.UUID.IsSet()
	} else {
		data, ok := profile.Data.(TrojanData)
		credentialSet = ok && data.Password.IsSet()
	}
	if !credentialSet {
		return newError(ErrorMissingCredential, protocol, "credential")
	}
	if _, err := normalizeHost(profile.Server, protocol); err != nil {
		return err
	}
	if _, err := parsePortSpec(profile.Port.Expression, protocol, false); err != nil {
		return err
	}
	return nil
}

func (vlessAdapter) Validate(profile *Profile) error {
	return validateUserInfoLink(profile, ProtocolVLESS)
}

func (trojanAdapter) Validate(profile *Profile) error {
	return validateUserInfoLink(profile, ProtocolTrojan)
}

func canonicalizeUserInfoLink(profile *Profile, protocol Protocol) (*Profile, error) {
	if err := validateUserInfoLink(profile, protocol); err != nil {
		return nil, err
	}
	canonical := cloneProfile(profile)
	canonical.Server, _ = normalizeHost(profile.Server, protocol)
	canonical.Port, _ = parsePortSpec(profile.Port.Expression, protocol, false)
	canonical.Port.Explicit = profile.Port.Explicit
	return canonical, nil
}

func (vlessAdapter) Canonicalize(profile *Profile) (*Profile, error) {
	return canonicalizeUserInfoLink(profile, ProtocolVLESS)
}

func (trojanAdapter) Canonicalize(profile *Profile) (*Profile, error) {
	return canonicalizeUserInfoLink(profile, ProtocolTrojan)
}

func serializeUserInfoLink(profile *Profile, protocol Protocol) (SerializationResult, error) {
	if err := validateUserInfoLink(profile, protocol); err != nil {
		return SerializationResult{}, err
	}
	credential := ""
	if protocol == ProtocolVLESS {
		credential = profile.Data.(VLESSData).UUID.Reveal()
	} else {
		credential = profile.Data.(TrojanData).Password.Reveal()
	}
	query := canonicalQuery(profile.QueryParameters, nil, nil)
	uri := canonicalURI(string(protocol), percentEncode(credential), formatHostPort(profile.Server, profile.Port), query, profile.DisplayName)
	return SerializationResult{
		URI:                NewSensitiveValue(uri),
		Mode:               CanonicalSerialization,
		Exact:              false,
		SemanticallyStable: true,
		Warnings:           append([]Warning(nil), profile.Warnings...),
	}, nil
}

func (vlessAdapter) SerializeCanonical(profile *Profile) (SerializationResult, error) {
	return serializeUserInfoLink(profile, ProtocolVLESS)
}

func (trojanAdapter) SerializeCanonical(profile *Profile) (SerializationResult, error) {
	return serializeUserInfoLink(profile, ProtocolTrojan)
}

func fingerprintUserInfoLink(profile *Profile, protocol Protocol) (SensitiveValue, error) {
	if err := validateUserInfoLink(profile, protocol); err != nil {
		return SensitiveValue{}, err
	}
	credential := ""
	if protocol == ProtocolVLESS {
		credential = profile.Data.(VLESSData).UUID.Reveal()
	} else {
		credential = profile.Data.(TrojanData).Password.Reveal()
	}
	fields := []canonicalParameter{{key: "credential", value: credential, hasValue: true}}
	return semanticFingerprintInput(profile, fields, fingerprintExtraParameterLines(profile.QueryParameters, nil)), nil
}

func (vlessAdapter) FingerprintInput(profile *Profile) (SensitiveValue, error) {
	return fingerprintUserInfoLink(profile, ProtocolVLESS)
}

func (trojanAdapter) FingerprintInput(profile *Profile) (SensitiveValue, error) {
	return fingerprintUserInfoLink(profile, ProtocolTrojan)
}

func (vmessAdapter) Parse(raw string) (*Profile, error) {
	trimmed := strings.TrimSpace(raw)
	scheme, err := detectScheme(trimmed)
	if err != nil || scheme != "vmess" {
		return nil, newError(ErrorUnsupportedScheme, ProtocolVMess, "scheme")
	}
	separator := strings.Index(trimmed, "://")
	if separator < 0 || separator+3 == len(trimmed) {
		return nil, newError(ErrorInvalidEncoding, ProtocolVMess, "payload")
	}
	encoded := trimmed[separator+3:]
	decoded, err := decodeBase64Compatibility(encoded)
	if err != nil {
		return nil, newError(ErrorInvalidEncoding, ProtocolVMess, "payload")
	}
	payload, err := decodeVMessObject(decoded)
	if err != nil {
		return nil, err
	}
	server := vmessText(payload["add"])
	server, err = normalizeHost(server, ProtocolVMess)
	if err != nil {
		return nil, err
	}
	portRaw := vmessText(payload["port"])
	portSpecified := portRaw != ""
	if portRaw == "" {
		portRaw = "443"
	}
	port, err := parsePortSpec(portRaw, ProtocolVMess, false)
	if err != nil {
		return nil, err
	}
	port.Explicit = portSpecified
	uuid := vmessText(payload["id"])
	if uuid == "" {
		return nil, newError(ErrorMissingCredential, ProtocolVMess, "id")
	}
	return &Profile{
		Protocol:     ProtocolVMess,
		Server:       server,
		Port:         port,
		DisplayName:  vmessText(payload["ps"]),
		OriginalURI:  NewSensitiveValue(raw),
		Data:         VMessData{UUID: NewSensitiveValue(uuid), RawPayload: NewSensitiveValue(decoded)},
		Capabilities: fullCapabilities("legacy-share-json-v2"),
	}, nil
}

func decodeVMessObject(raw string) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil || payload == nil {
		return nil, newError(ErrorInvalidEncoding, ProtocolVMess, "payload")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, newError(ErrorInvalidEncoding, ProtocolVMess, "payload")
	}
	return payload, nil
}

func vmessText(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func (vmessAdapter) Validate(profile *Profile) error {
	if profile == nil || profile.Protocol != ProtocolVMess {
		return newError(ErrorInvalidProfile, ProtocolVMess, "profile")
	}
	data, ok := profile.Data.(VMessData)
	if !ok || !data.UUID.IsSet() || !data.RawPayload.IsSet() {
		return newError(ErrorMissingCredential, ProtocolVMess, "id")
	}
	if _, err := normalizeHost(profile.Server, ProtocolVMess); err != nil {
		return err
	}
	if _, err := parsePortSpec(profile.Port.Expression, ProtocolVMess, false); err != nil {
		return err
	}
	_, err := decodeVMessObject(data.RawPayload.Reveal())
	return err
}

func (adapter vmessAdapter) Canonicalize(profile *Profile) (*Profile, error) {
	if err := adapter.Validate(profile); err != nil {
		return nil, err
	}
	canonical := cloneProfile(profile)
	canonical.Server, _ = normalizeHost(profile.Server, ProtocolVMess)
	canonical.Port, _ = parsePortSpec(profile.Port.Expression, ProtocolVMess, false)
	canonical.Port.Explicit = profile.Port.Explicit
	payload, _ := decodeVMessObject(profile.Data.(VMessData).RawPayload.Reveal())
	payload["add"] = canonical.Server
	payload["port"] = canonical.Port.Expression
	payload["id"] = profile.Data.(VMessData).UUID.Reveal()
	if canonical.DisplayName != "" {
		payload["ps"] = canonical.DisplayName
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, newError(ErrorInvalidProfile, ProtocolVMess, "payload")
	}
	canonical.Data = VMessData{UUID: profile.Data.(VMessData).UUID, RawPayload: NewSensitiveValue(string(encoded))}
	return canonical, nil
}

func (adapter vmessAdapter) SerializeCanonical(profile *Profile) (SerializationResult, error) {
	canonical, err := adapter.Canonicalize(profile)
	if err != nil {
		return SerializationResult{}, err
	}
	payload := canonical.Data.(VMessData).RawPayload.Reveal()
	uri := "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload))
	return SerializationResult{
		URI:                NewSensitiveValue(uri),
		Mode:               CanonicalSerialization,
		Exact:              false,
		SemanticallyStable: true,
		Warnings:           append([]Warning(nil), profile.Warnings...),
	}, nil
}

func (adapter vmessAdapter) FingerprintInput(profile *Profile) (SensitiveValue, error) {
	canonical, err := adapter.Canonicalize(profile)
	if err != nil {
		return SensitiveValue{}, err
	}
	payload, _ := decodeVMessObject(canonical.Data.(VMessData).RawPayload.Reveal())
	for _, displayKey := range []string{"ps", "remark", "remarks", "tag", "serverDescription"} {
		delete(payload, displayKey)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return SensitiveValue{}, newError(ErrorInvalidProfile, ProtocolVMess, "payload")
	}
	return semanticFingerprintInput(canonical, []canonicalParameter{{key: "payload", value: string(encoded), hasValue: true}}, nil), nil
}
