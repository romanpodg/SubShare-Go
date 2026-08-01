package profiles

import (
	"strings"
)

var hysteria2Aliases = parameterAliases(
	"obfs",
	"obfs-password",
	"sni",
	"insecure",
	"pinSHA256=pinsha256",
)

type hysteria2Adapter struct{}

func (hysteria2Adapter) Protocol() Protocol { return ProtocolHysteria2 }
func (hysteria2Adapter) Schemes() []string  { return []string{"hysteria2", "hy2"} }

func (hysteria2Adapter) Parse(raw string) (*Profile, error) {
	components, err := splitStandardURI(raw, ProtocolHysteria2)
	if err != nil {
		return nil, err
	}
	if components.scheme != "hysteria2" && components.scheme != "hy2" {
		return nil, newError(ErrorUnsupportedScheme, ProtocolHysteria2, "scheme")
	}
	authority, err := parseAuthority(components.authority, ProtocolHysteria2, "443", true)
	if err != nil {
		return nil, err
	}
	if !authority.hasUserInfo || authority.rawUserInfo == "" {
		return nil, newError(ErrorMissingCredential, ProtocolHysteria2, "authentication")
	}
	authentication, err := decodeUserInfo(authority.rawUserInfo, ProtocolHysteria2, "authentication")
	if err != nil {
		return nil, err
	}
	if authentication == "" {
		return nil, newError(ErrorMissingCredential, ProtocolHysteria2, "authentication")
	}
	parameters, err := parseQuery(components.rawQuery, ProtocolHysteria2)
	if err != nil {
		return nil, err
	}
	if err := validateKnownParameterValues(parameters, hysteria2Aliases, ProtocolHysteria2); err != nil {
		return nil, err
	}
	data := Hysteria2Data{Authentication: NewSensitiveValue(authentication)}
	data.ObfuscationType, _ = firstParameter(parameters, hysteria2Aliases, "obfs")
	data.ObfuscationType = strings.ToLower(strings.TrimSpace(data.ObfuscationType))
	if hasParameter(parameters, hysteria2Aliases, "obfs") && data.ObfuscationType == "" {
		return nil, newError(ErrorInvalidProfile, ProtocolHysteria2, "obfs")
	}
	if value, present := firstParameter(parameters, hysteria2Aliases, "obfs-password"); present {
		data.ObfuscationPassword = NewSensitiveValue(value)
	}
	data.SNI, _ = firstParameter(parameters, hysteria2Aliases, "sni")
	data.SNI = strings.ToLower(strings.TrimSpace(data.SNI))
	if hasParameter(parameters, hysteria2Aliases, "sni") && data.SNI == "" {
		return nil, newError(ErrorInvalidProfile, ProtocolHysteria2, "sni")
	}
	data.CertificateSHA256, _ = firstParameter(parameters, hysteria2Aliases, "pinSHA256")
	if value, present := firstParameter(parameters, hysteria2Aliases, "insecure"); present {
		data.Insecure, err = parseBoolean(value, ProtocolHysteria2, "insecure", true)
		if err != nil {
			return nil, err
		}
	}
	warnings := duplicateWarnings(parameters, hysteria2Aliases)
	if components.scheme == "hy2" {
		warnings = append(warnings, Warning{Code: WarningNonCanonicalScheme, Message: "hy2 scheme alias was accepted"})
	}
	if data.ObfuscationType == "gecko" {
		warnings = append(warnings, Warning{Code: WarningClientVersionSensitive, Message: "gecko obfuscation requires a Hysteria client generation that supports it"})
	}
	profile := &Profile{
		Protocol:               ProtocolHysteria2,
		Server:                 authority.host,
		Port:                   authority.port,
		DisplayName:            components.fragment,
		OriginalURI:            NewSensitiveValue(raw),
		Data:                   data,
		QueryParameters:        parameters,
		UnknownQueryParameters: unknownParameters(parameters, hysteria2Aliases),
		Warnings:               warnings,
		Capabilities:           fullCapabilities("2"),
	}
	if err := (hysteria2Adapter{}).Validate(profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (hysteria2Adapter) Validate(profile *Profile) error {
	if profile == nil || profile.Protocol != ProtocolHysteria2 {
		return newError(ErrorInvalidProfile, ProtocolHysteria2, "profile")
	}
	data, ok := profile.Data.(Hysteria2Data)
	if !ok {
		return newError(ErrorInvalidProfile, ProtocolHysteria2, "data")
	}
	if !data.Authentication.IsSet() {
		return newError(ErrorMissingCredential, ProtocolHysteria2, "authentication")
	}
	if _, err := normalizeHost(profile.Server, ProtocolHysteria2); err != nil {
		return err
	}
	if _, err := parsePortSpec(profile.Port.Expression, ProtocolHysteria2, true); err != nil {
		return err
	}
	switch strings.ToLower(data.ObfuscationType) {
	case "":
	case "salamander", "gecko":
		if !data.ObfuscationPassword.IsSet() {
			return newError(ErrorMissingCredential, ProtocolHysteria2, "obfuscation_password")
		}
	default:
		return newError(ErrorInvalidProfile, ProtocolHysteria2, "obfuscation_type")
	}
	if data.SNI != "" {
		if _, err := normalizeHost(data.SNI, ProtocolHysteria2); err != nil {
			return newError(ErrorInvalidProfile, ProtocolHysteria2, "sni")
		}
	}
	if data.CertificateSHA256 != "" {
		if _, err := normalizeCertificatePin(data.CertificateSHA256); err != nil {
			return err
		}
	}
	return nil
}

func (adapter hysteria2Adapter) Canonicalize(profile *Profile) (*Profile, error) {
	if err := adapter.Validate(profile); err != nil {
		return nil, err
	}
	canonical := cloneProfile(profile)
	canonical.Server, _ = normalizeHost(profile.Server, ProtocolHysteria2)
	canonical.Port, _ = parsePortSpec(profile.Port.Expression, ProtocolHysteria2, true)
	canonical.Port.Explicit = profile.Port.Explicit
	data := canonical.Data.(Hysteria2Data)
	data.ObfuscationType = strings.ToLower(strings.TrimSpace(data.ObfuscationType))
	data.SNI = strings.ToLower(strings.TrimSpace(data.SNI))
	if data.CertificateSHA256 != "" {
		data.CertificateSHA256, _ = normalizeCertificatePin(data.CertificateSHA256)
	}
	canonical.Data = data
	return canonical, nil
}

func (adapter hysteria2Adapter) SerializeCanonical(profile *Profile) (SerializationResult, error) {
	if err := adapter.Validate(profile); err != nil {
		return SerializationResult{}, err
	}
	data := profile.Data.(Hysteria2Data)
	primary := make([]canonicalParameter, 0, 5)
	if data.ObfuscationType != "" || hasParameter(profile.QueryParameters, hysteria2Aliases, "obfs") {
		primary = append(primary, canonicalParameter{key: "obfs", value: data.ObfuscationType, hasValue: true})
	}
	if data.ObfuscationPassword.IsSet() || hasParameter(profile.QueryParameters, hysteria2Aliases, "obfs-password") {
		primary = append(primary, canonicalParameter{key: "obfs-password", value: data.ObfuscationPassword.Reveal(), hasValue: true})
	}
	if data.SNI != "" || hasParameter(profile.QueryParameters, hysteria2Aliases, "sni") {
		primary = append(primary, canonicalParameter{key: "sni", value: data.SNI, hasValue: true})
	}
	if data.Insecure || hasParameter(profile.QueryParameters, hysteria2Aliases, "insecure") {
		value := "0"
		if data.Insecure {
			value = "1"
		}
		primary = append(primary, canonicalParameter{key: "insecure", value: value, hasValue: true})
	}
	if data.CertificateSHA256 != "" || hasParameter(profile.QueryParameters, hysteria2Aliases, "pinSHA256") {
		primary = append(primary, canonicalParameter{key: "pinSHA256", value: data.CertificateSHA256, hasValue: true})
	}
	query := canonicalQuery(profile.QueryParameters, primary, hysteria2Aliases)
	uri := canonicalURI(
		"hysteria2",
		percentEncodeAuth(data.Authentication.Reveal(), true),
		formatHostPort(profile.Server, profile.Port),
		query,
		profile.DisplayName,
	)
	return SerializationResult{
		URI:                NewSensitiveValue(uri),
		Mode:               CanonicalSerialization,
		Exact:              false,
		SemanticallyStable: true,
		Warnings:           append([]Warning(nil), profile.Warnings...),
	}, nil
}

func (adapter hysteria2Adapter) FingerprintInput(profile *Profile) (SensitiveValue, error) {
	if err := adapter.Validate(profile); err != nil {
		return SensitiveValue{}, err
	}
	data := profile.Data.(Hysteria2Data)
	fields := []canonicalParameter{
		{key: "authentication", value: data.Authentication.Reveal(), hasValue: true},
		{key: "sni", value: strings.ToLower(data.SNI), hasValue: true},
		{key: "insecure", value: boolString(data.Insecure), hasValue: true},
		{key: "certificate_sha256", value: data.CertificateSHA256, hasValue: true},
		{key: "obfuscation_type", value: strings.ToLower(data.ObfuscationType), hasValue: true},
		{key: "obfuscation_password", value: data.ObfuscationPassword.Reveal(), hasValue: true},
	}
	return semanticFingerprintInput(profile, fields, fingerprintExtraParameterLines(profile.QueryParameters, hysteria2Aliases)), nil
}

func normalizeCertificatePin(raw string) (string, error) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(raw), ":", ""))
	if len(normalized) != 64 {
		return "", newError(ErrorInvalidProfile, ProtocolHysteria2, "certificate_sha256")
	}
	for _, char := range normalized {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return "", newError(ErrorInvalidProfile, ProtocolHysteria2, "certificate_sha256")
		}
	}
	return normalized, nil
}
