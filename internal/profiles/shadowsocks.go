package profiles

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

var shadowsocksAliases = parameterAliases("plugin")

type shadowsocksAdapter struct{}

func (shadowsocksAdapter) Protocol() Protocol { return ProtocolShadowsocks }
func (shadowsocksAdapter) Schemes() []string  { return []string{"ss"} }

func (shadowsocksAdapter) Parse(raw string) (*Profile, error) {
	trimmed := strings.TrimSpace(raw)
	components, legacyCredential, legacy, err := splitShadowsocksURI(trimmed)
	if err != nil {
		return nil, err
	}
	parameters, err := parseQuery(components.rawQuery, ProtocolShadowsocks)
	if err != nil {
		return nil, err
	}
	if err := validateKnownParameterValues(parameters, shadowsocksAliases, ProtocolShadowsocks); err != nil {
		return nil, err
	}

	var credential shadowsocksCredential
	if legacy {
		credential, err = parseLegacyShadowsocksCredential(legacyCredential)
	} else {
		credential, err = parseSIP002Credential(components.authority)
	}
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.ToLower(credential.method), "2022-") {
		if credential.style != ShadowsocksUserInfoPlain {
			return nil, newError(ErrorInvalidEncoding, ProtocolShadowsocks, "userinfo")
		}
		if err := validateShadowsocks2022Credential(credential.method, credential.password); err != nil {
			return nil, err
		}
	}

	plugin, err := parseShadowsocksPlugin(parameters)
	if err != nil {
		return nil, err
	}

	warnings := duplicateWarnings(parameters, shadowsocksAliases)
	if legacy {
		warnings = append(warnings, Warning{Code: WarningLegacyInput, Message: "legacy fully encoded Shadowsocks input was accepted"})
	}
	authority := credential.authority
	profile := &Profile{
		Protocol:               ProtocolShadowsocks,
		Server:                 authority.host,
		Port:                   authority.port,
		DisplayName:            components.fragment,
		OriginalURI:            NewSensitiveValue(raw),
		Data:                   ShadowsocksData{Method: credential.method, Password: NewSensitiveValue(credential.password), Plugin: plugin, UserInfoStyle: credential.style, LegacyInput: legacy},
		QueryParameters:        parameters,
		UnknownQueryParameters: unknownParameters(parameters, shadowsocksAliases),
		Warnings:               warnings,
		Capabilities:           fullCapabilities("sip002"),
	}
	if err := (shadowsocksAdapter{}).Validate(profile); err != nil {
		return nil, err
	}
	return profile, nil
}

type shadowsocksCredential struct {
	authority authorityParts
	method    string
	password  string
	style     ShadowsocksUserInfoStyle
}

// parseLegacyShadowsocksCredential handles the fully base64-encoded
// method:password@host:port body.
func parseLegacyShadowsocksCredential(legacyCredential string) (shadowsocksCredential, error) {
	at := strings.LastIndexByte(legacyCredential, '@')
	if at <= 0 || at == len(legacyCredential)-1 {
		return shadowsocksCredential{}, newError(ErrorInvalidAuthority, ProtocolShadowsocks, "legacy_authority")
	}
	method, password, err := splitShadowsocksCredential(legacyCredential[:at], false)
	if err != nil {
		return shadowsocksCredential{}, err
	}
	authority, err := parseAuthority("x@"+legacyCredential[at+1:], ProtocolShadowsocks, "", false)
	if err != nil {
		return shadowsocksCredential{}, err
	}
	return shadowsocksCredential{authority: authority, method: method, password: password, style: ShadowsocksUserInfoLegacy}, nil
}

// parseSIP002Credential handles userinfo@host:port where userinfo is either
// plain percent-encoded method:password or base64url(method:password).
func parseSIP002Credential(rawAuthority string) (shadowsocksCredential, error) {
	authority, err := parseAuthority(rawAuthority, ProtocolShadowsocks, "", false)
	if err != nil {
		return shadowsocksCredential{}, err
	}
	if !authority.hasUserInfo || authority.rawUserInfo == "" {
		return shadowsocksCredential{}, newError(ErrorMissingCredential, ProtocolShadowsocks, "userinfo")
	}
	credential := shadowsocksCredential{authority: authority, style: ShadowsocksUserInfoBase64}
	if strings.Contains(authority.rawUserInfo, ":") {
		credential.style = ShadowsocksUserInfoPlain
		credential.method, credential.password, err = splitShadowsocksCredential(authority.rawUserInfo, true)
		if err != nil {
			return shadowsocksCredential{}, err
		}
		return credential, nil
	}
	decoded, err := decodeBase64URL(authority.rawUserInfo)
	if err != nil {
		return shadowsocksCredential{}, err
	}
	credential.method, credential.password, err = splitShadowsocksCredential(decoded, false)
	if err != nil {
		return shadowsocksCredential{}, err
	}
	return credential, nil
}

func parseShadowsocksPlugin(parameters []QueryParameter) (*ShadowsocksPlugin, error) {
	pluginValue, present := firstParameter(parameters, shadowsocksAliases, "plugin")
	if !present {
		return nil, nil
	}
	parts := strings.Split(pluginValue, ";")
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return nil, newError(ErrorInvalidProfile, ProtocolShadowsocks, "plugin")
	}
	plugin := &ShadowsocksPlugin{Name: name}
	if len(parts) > 1 {
		plugin.Options = NewSensitiveValue(strings.Join(parts[1:], ";"))
	}
	return plugin, nil
}

func splitShadowsocksURI(raw string) (uriComponents, string, bool, error) {
	scheme, err := detectScheme(raw)
	if err != nil || scheme != "ss" {
		return uriComponents{}, "", false, newError(ErrorUnsupportedScheme, ProtocolShadowsocks, "scheme")
	}
	prefix := strings.Index(raw, "://")
	if prefix < 0 {
		return uriComponents{}, "", false, newError(ErrorInvalidAuthority, ProtocolShadowsocks, "uri")
	}
	remainder := raw[prefix+3:]
	components := uriComponents{scheme: scheme}
	if before, after, found := strings.Cut(remainder, "#"); found {
		remainder = before
		components.hasFragment = true
		components.fragment, err = decodeUserInfo(after, ProtocolShadowsocks, "fragment")
		if err != nil {
			return uriComponents{}, "", false, err
		}
	}
	if before, after, found := strings.Cut(remainder, "?"); found {
		remainder = before
		components.hasQuery = true
		components.rawQuery = after
	}
	if strings.Contains(remainder, "@") {
		components.authority = strings.TrimSuffix(remainder, "/")
		if strings.Contains(components.authority, "/") {
			return uriComponents{}, "", false, newError(ErrorInvalidAuthority, ProtocolShadowsocks, "path")
		}
		return components, "", false, nil
	}

	candidates := []string{remainder}
	if strings.HasSuffix(remainder, "/") {
		candidates = append(candidates, strings.TrimSuffix(remainder, "/"))
	}
	for _, candidate := range candidates {
		decoded, decodeErr := decodeBase64Compatibility(candidate)
		if decodeErr == nil && strings.Contains(decoded, "@") {
			return components, decoded, true, nil
		}
	}
	return uriComponents{}, "", false, newError(ErrorInvalidEncoding, ProtocolShadowsocks, "legacy_uri")
}

func splitShadowsocksCredential(raw string, percentEncoded bool) (string, string, error) {
	methodRaw, passwordRaw, found := strings.Cut(raw, ":")
	if !found {
		return "", "", newError(ErrorInvalidEncoding, ProtocolShadowsocks, "userinfo")
	}
	method := methodRaw
	password := passwordRaw
	var err error
	if percentEncoded {
		method, err = decodeUserInfo(methodRaw, ProtocolShadowsocks, "method")
		if err != nil {
			return "", "", err
		}
		password, err = decodeUserInfo(passwordRaw, ProtocolShadowsocks, "password")
		if err != nil {
			return "", "", err
		}
	}
	method = strings.TrimSpace(method)
	if method == "" {
		return "", "", newError(ErrorMissingCredential, ProtocolShadowsocks, "method")
	}
	if password == "" {
		return "", "", newError(ErrorMissingCredential, ProtocolShadowsocks, "password")
	}
	return method, password, nil
}

// decodeBase64URL is the modern SIP002 userinfo decoder. SIP002 uses the
// URL-safe alphabet; accepting the standard '+' and '/' alphabet here would
// blur modern userinfo with the separate legacy fully encoded dialect.
func decodeBase64URL(raw string) (string, error) {
	return decodeBase64(raw, base64.RawURLEncoding, base64.URLEncoding)
}

// decodeBase64Compatibility is intentionally limited to legacy SS bodies and
// pre-existing VMess JSON input, where both standard and URL-safe encodings are
// encountered in deployed clients.
func decodeBase64Compatibility(raw string) (string, error) {
	encodings := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	return decodeBase64(raw, encodings...)
}

func decodeBase64(raw string, encodings ...*base64.Encoding) (string, error) {
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(raw)
		if err == nil && utf8.Valid(decoded) {
			return string(decoded), nil
		}
	}
	return "", newError(ErrorInvalidEncoding, ProtocolShadowsocks, "base64")
}

func validateShadowsocks2022Credential(method, password string) error {
	wantLength := 0
	switch strings.ToLower(method) {
	case "2022-blake3-aes-128-gcm":
		wantLength = 16
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		wantLength = 32
	default:
		return newError(ErrorUnsupportedGeneration, ProtocolShadowsocks, "method")
	}
	decoded, err := base64.StdEncoding.DecodeString(password)
	if err != nil || len(decoded) != wantLength {
		return newError(ErrorInvalidEncoding, ProtocolShadowsocks, "password")
	}
	return nil
}

func (shadowsocksAdapter) Validate(profile *Profile) error {
	if profile == nil || profile.Protocol != ProtocolShadowsocks {
		return newError(ErrorInvalidProfile, ProtocolShadowsocks, "profile")
	}
	data, ok := profile.Data.(ShadowsocksData)
	if !ok {
		return newError(ErrorInvalidProfile, ProtocolShadowsocks, "data")
	}
	if data.Method == "" {
		return newError(ErrorMissingCredential, ProtocolShadowsocks, "method")
	}
	if !data.Password.IsSet() {
		return newError(ErrorMissingCredential, ProtocolShadowsocks, "password")
	}
	if _, err := normalizeHost(profile.Server, ProtocolShadowsocks); err != nil {
		return err
	}
	if _, err := parsePortSpec(profile.Port.Expression, ProtocolShadowsocks, false); err != nil {
		return err
	}
	if strings.HasPrefix(strings.ToLower(data.Method), "2022-") {
		return validateShadowsocks2022Credential(data.Method, data.Password.Reveal())
	}
	return nil
}

func (adapter shadowsocksAdapter) Canonicalize(profile *Profile) (*Profile, error) {
	if err := adapter.Validate(profile); err != nil {
		return nil, err
	}
	canonical := cloneProfile(profile)
	canonical.Server, _ = normalizeHost(profile.Server, ProtocolShadowsocks)
	canonical.Port, _ = parsePortSpec(profile.Port.Expression, ProtocolShadowsocks, false)
	canonical.Port.Explicit = profile.Port.Explicit
	data := canonical.Data.(ShadowsocksData)
	data.Method = strings.ToLower(strings.TrimSpace(data.Method))
	canonical.Data = data
	return canonical, nil
}

func (adapter shadowsocksAdapter) SerializeCanonical(profile *Profile) (SerializationResult, error) {
	if err := adapter.Validate(profile); err != nil {
		return SerializationResult{}, err
	}
	data := profile.Data.(ShadowsocksData)
	credential := data.Method + ":" + data.Password.Reveal()
	userInfo := base64.RawURLEncoding.EncodeToString([]byte(credential))
	if strings.HasPrefix(data.Method, "2022-") {
		userInfo = percentEncode(data.Method) + ":" + percentEncode(data.Password.Reveal())
	}
	primary := make([]canonicalParameter, 0, 1)
	if data.Plugin != nil {
		pluginValue := data.Plugin.Name
		if data.Plugin.Options.IsSet() {
			pluginValue += ";" + data.Plugin.Options.Reveal()
		}
		primary = append(primary, canonicalParameter{key: "plugin", value: pluginValue, hasValue: true})
	}
	query := canonicalQuery(profile.QueryParameters, primary, shadowsocksAliases)
	uri := canonicalURI("ss", userInfo, formatHostPort(profile.Server, profile.Port), query, profile.DisplayName)
	return SerializationResult{
		URI:                NewSensitiveValue(uri),
		Mode:               CanonicalSerialization,
		Exact:              false,
		SemanticallyStable: true,
		Warnings:           append([]Warning(nil), profile.Warnings...),
	}, nil
}

func (adapter shadowsocksAdapter) FingerprintInput(profile *Profile) (SensitiveValue, error) {
	if err := adapter.Validate(profile); err != nil {
		return SensitiveValue{}, err
	}
	data := profile.Data.(ShadowsocksData)
	fields := []canonicalParameter{
		{key: "method", value: strings.ToLower(data.Method), hasValue: true},
		{key: "password", value: data.Password.Reveal(), hasValue: true},
	}
	if data.Plugin != nil {
		fields = append(fields,
			canonicalParameter{key: "plugin", value: data.Plugin.Name, hasValue: true},
			canonicalParameter{key: "plugin_options", value: data.Plugin.Options.Reveal(), hasValue: true},
		)
	}
	return semanticFingerprintInput(profile, fields, fingerprintExtraParameterLines(profile.QueryParameters, shadowsocksAliases)), nil
}
