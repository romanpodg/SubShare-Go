package profiles

import (
	"strconv"
	"strings"
	"time"
)

var tuicAliases = parameterAliases(
	"token",
	"uuid",
	"password",
	"sni=servername,server_name,sni",
	"alpn",
	"skip-cert-verify=skip-cert-verify,skip_cert_verify,insecure,allow_insecure",
	"disable-sni=disable-sni,disable_sni",
	"congestion-controller=congestion-controller,congestion_control",
	"udp-relay-mode=udp-relay-mode,udp_relay_mode",
	"udp-over-stream=udp-over-stream,udp_over_stream",
	"zero-rtt=zero-rtt,zero_rtt,zero-rtt-handshake,zero_rtt_handshake,reduce-rtt,reduce_rtt",
	"heartbeat=heartbeat,heartbeat-interval,heartbeat_interval",
	"request-timeout=request-timeout,request_timeout",
	"fast-open=fast-open,fast_open",
	"max-open-streams=max-open-streams,max_open_streams",
	"max-udp-relay-packet-size=max-udp-relay-packet-size,max_udp_relay_packet_size",
)

type tuicAdapter struct{}

func (tuicAdapter) Protocol() Protocol { return ProtocolTUIC }
func (tuicAdapter) Schemes() []string  { return []string{"tuic"} }

// Parse accepts a documented compatibility URI convention because the TUIC
// wire specification does not define a universal share URI:
//
//   - v5 canonical: tuic://percent-encoded-uuid:password@host:port
//   - v5 compatibility: uuid/password query fields, or UUID userinfo plus a
//     password query field
//   - v4 compatibility: one token in userinfo or the token query field
//
// Unambiguous Mihomo and sing-box spellings are aliases for the same typed
// fields, but every spelling retains explicit provenance for later generator
// decisions. Mixed v4/v5 credentials are rejected as ambiguous.
func (tuicAdapter) Parse(raw string) (*Profile, error) {
	components, err := splitStandardURI(raw, ProtocolTUIC)
	if err != nil {
		return nil, err
	}
	if components.scheme != "tuic" {
		return nil, newError(ErrorUnsupportedScheme, ProtocolTUIC, "scheme")
	}
	authority, err := parseAuthority(components.authority, ProtocolTUIC, "", false)
	if err != nil {
		return nil, err
	}
	parameters, err := parseQuery(components.rawQuery, ProtocolTUIC)
	if err != nil {
		return nil, err
	}
	if err := validateKnownParameterValues(parameters, tuicAliases, ProtocolTUIC); err != nil {
		return nil, err
	}

	data, err := parseTUICCredentials(authority, parameters)
	if err != nil {
		return nil, err
	}
	data.FieldObservations = tuicFieldObservations(authority, parameters, data.Generation)
	data.SNI, _ = firstParameter(parameters, tuicAliases, "sni")
	data.SNI = strings.ToLower(strings.TrimSpace(data.SNI))
	if hasParameter(parameters, tuicAliases, "sni") && data.SNI == "" {
		return nil, newError(ErrorInvalidProfile, ProtocolTUIC, "sni")
	}
	if alpnValue, present := firstParameter(parameters, tuicAliases, "alpn"); present {
		for _, item := range strings.Split(alpnValue, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				return nil, newError(ErrorInvalidProfile, ProtocolTUIC, "alpn")
			}
			data.ALPN = append(data.ALPN, item)
		}
	}
	if value, present := firstParameter(parameters, tuicAliases, "skip-cert-verify"); present {
		data.SkipCertificateVerification, err = parseBoolean(value, ProtocolTUIC, "skip_cert_verify", false)
		if err != nil {
			return nil, err
		}
	}
	if value, present := firstParameter(parameters, tuicAliases, "disable-sni"); present {
		data.DisableSNI, err = parseBoolean(value, ProtocolTUIC, "disable_sni", false)
		if err != nil {
			return nil, err
		}
	}
	data.CongestionController, _ = firstParameter(parameters, tuicAliases, "congestion-controller")
	data.CongestionController = strings.ToLower(strings.TrimSpace(data.CongestionController))
	if hasParameter(parameters, tuicAliases, "congestion-controller") && data.CongestionController == "" {
		return nil, newError(ErrorInvalidProfile, ProtocolTUIC, "congestion_controller")
	}
	if data.CongestionController == "" {
		data.CongestionController = "cubic"
	}
	data.UDPRelayMode, _ = firstParameter(parameters, tuicAliases, "udp-relay-mode")
	data.UDPRelayMode = strings.ToLower(strings.TrimSpace(data.UDPRelayMode))
	if hasParameter(parameters, tuicAliases, "udp-relay-mode") && data.UDPRelayMode == "" {
		return nil, newError(ErrorInvalidProfile, ProtocolTUIC, "udp_relay_mode")
	}
	if data.UDPRelayMode == "" {
		data.UDPRelayMode = "native"
	}
	if value, present := firstParameter(parameters, tuicAliases, "udp-over-stream"); present {
		data.UDPOverStream, err = parseBoolean(value, ProtocolTUIC, "udp_over_stream", false)
		if err != nil {
			return nil, err
		}
	}
	if value, present := firstParameter(parameters, tuicAliases, "zero-rtt"); present {
		data.ZeroRTT, err = parseBoolean(value, ProtocolTUIC, "zero_rtt", false)
		if err != nil {
			return nil, err
		}
	}
	data.Heartbeat, err = durationParameter(parameters, "heartbeat")
	if err != nil {
		return nil, err
	}
	data.RequestTimeout, err = durationParameter(parameters, "request-timeout")
	if err != nil {
		return nil, err
	}
	if value, present := firstParameter(parameters, tuicAliases, "fast-open"); present {
		data.FastOpen, err = parseBoolean(value, ProtocolTUIC, "fast_open", false)
		if err != nil {
			return nil, err
		}
	}
	data.MaxOpenStreams, err = positiveIntegerParameter(parameters, "max-open-streams", 1<<31-1)
	if err != nil {
		return nil, err
	}
	data.MaxUDPRelayPacketSize, err = positiveIntegerParameter(parameters, "max-udp-relay-packet-size", 65535)
	if err != nil {
		return nil, err
	}

	warnings := duplicateWarnings(parameters, tuicAliases)
	capabilities := fullCapabilities("5")
	if data.Generation == 4 {
		capabilities = Capabilities{
			Status:              CapabilityReadOnly,
			Generation:          "4",
			Parse:               true,
			Validate:            true,
			CanonicalSerialize:  false,
			Generate:            false,
			Fingerprint:         true,
			ExactOriginalOutput: true,
		}
		warnings = append(warnings, Warning{Code: WarningCompatibilityOnly, Message: "TUIC v4 token input is read-only compatibility data"})
	}
	if data.DisableSNI && data.SNI != "" {
		warnings = append(warnings, Warning{Code: WarningConflictingPreference, Message: "SNI is present while SNI transmission is disabled"})
	}
	profile := &Profile{
		Protocol:               ProtocolTUIC,
		Server:                 authority.host,
		Port:                   authority.port,
		DisplayName:            components.fragment,
		OriginalURI:            NewSensitiveValue(raw),
		Data:                   data,
		QueryParameters:        parameters,
		UnknownQueryParameters: unknownParameters(parameters, tuicAliases),
		Warnings:               warnings,
		Capabilities:           capabilities,
	}
	if err := (tuicAdapter{}).Validate(profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func parseTUICCredentials(authority authorityParts, parameters []QueryParameter) (TUICData, error) {
	var data TUICData
	for _, credentialField := range []string{"token", "uuid", "password"} {
		if conflictingParameterValues(parameters, credentialField) {
			return TUICData{}, newError(ErrorAmbiguousTUICDialect, ProtocolTUIC, credentialField)
		}
	}
	queryToken, hasQueryToken := firstParameter(parameters, tuicAliases, "token")
	queryUUID, hasQueryUUID := firstParameter(parameters, tuicAliases, "uuid")
	queryPassword, hasQueryPassword := firstParameter(parameters, tuicAliases, "password")

	var userSingle string
	var userUUID string
	var userPassword string
	hasUserPair := false
	if authority.hasUserInfo {
		if before, after, found := strings.Cut(authority.rawUserInfo, ":"); found {
			hasUserPair = true
			var err error
			userUUID, err = decodeUserInfo(before, ProtocolTUIC, "uuid")
			if err != nil {
				return TUICData{}, err
			}
			userPassword, err = decodeUserInfo(after, ProtocolTUIC, "password")
			if err != nil {
				return TUICData{}, err
			}
		} else {
			var err error
			userSingle, err = decodeUserInfo(authority.rawUserInfo, ProtocolTUIC, "token")
			if err != nil {
				return TUICData{}, err
			}
		}
	}
	if userSingle != "" && !hasQueryToken && !hasQueryUUID && !hasQueryPassword {
		if isUUID(userSingle) {
			return TUICData{}, newError(ErrorMissingCredential, ProtocolTUIC, "password")
		}
		if looksLikeUUID(userSingle) {
			return TUICData{}, newError(ErrorInvalidProfile, ProtocolTUIC, "uuid")
		}
	}
	if userSingle != "" && hasQueryToken && userSingle != queryToken {
		return TUICData{}, newError(ErrorAmbiguousTUICDialect, ProtocolTUIC, "token")
	}
	if userSingle != "" && hasQueryPassword && !hasQueryToken && !hasQueryUUID {
		if !isUUID(userSingle) {
			return TUICData{}, newError(ErrorAmbiguousTUICDialect, ProtocolTUIC, "credential")
		}
		queryUUID = userSingle
		hasQueryUUID = true
		userSingle = ""
	}

	hasV5Signal := hasUserPair || hasQueryUUID || hasQueryPassword
	hasV4Signal := hasQueryToken || (userSingle != "" && !hasQueryPassword)
	if hasV4Signal && hasV5Signal {
		return TUICData{}, newError(ErrorAmbiguousTUICDialect, ProtocolTUIC, "credential")
	}
	if hasV4Signal {
		token := queryToken
		if token == "" {
			token = userSingle
		}
		if token == "" {
			return TUICData{}, newError(ErrorMissingCredential, ProtocolTUIC, "token")
		}
		data.Generation = 4
		data.Token = NewSensitiveValue(token)
		return data, nil
	}
	if !hasV5Signal {
		return TUICData{}, newError(ErrorMissingCredential, ProtocolTUIC, "credential")
	}
	uuid := userUUID
	password := userPassword
	if hasQueryUUID {
		if uuid != "" && uuid != queryUUID {
			return TUICData{}, newError(ErrorAmbiguousTUICDialect, ProtocolTUIC, "uuid")
		}
		uuid = queryUUID
	}
	if hasQueryPassword {
		if password != "" && password != queryPassword {
			return TUICData{}, newError(ErrorAmbiguousTUICDialect, ProtocolTUIC, "password")
		}
		password = queryPassword
	}
	if uuid == "" {
		return TUICData{}, newError(ErrorMissingCredential, ProtocolTUIC, "uuid")
	}
	if password == "" {
		return TUICData{}, newError(ErrorMissingCredential, ProtocolTUIC, "password")
	}
	if !isUUID(uuid) {
		return TUICData{}, newError(ErrorInvalidProfile, ProtocolTUIC, "uuid")
	}
	data.Generation = 5
	data.UUID = NewSensitiveValue(strings.ToLower(uuid))
	data.Password = NewSensitiveValue(password)
	return data, nil
}

func conflictingParameterValues(parameters []QueryParameter, canonical string) bool {
	var first string
	found := false
	for _, parameter := range parameters {
		if tuicAliases[strings.ToLower(parameter.Key)] != canonical {
			continue
		}
		value := parameter.Value.Reveal()
		if !found {
			first = value
			found = true
			continue
		}
		if value != first {
			return true
		}
	}
	return false
}

func durationParameter(parameters []QueryParameter, canonical string) (string, error) {
	for _, parameter := range parameters {
		if tuicAliases[strings.ToLower(parameter.Key)] != canonical {
			continue
		}
		value := strings.TrimSpace(parameter.Value.Reveal())
		if value == "" {
			return "", newError(ErrorInvalidProfile, ProtocolTUIC, canonical)
		}
		key := strings.ToLower(parameter.Key)
		if allDigits(value) && (strings.Contains(key, "interval") || strings.Contains(key, "timeout")) {
			value += "ms"
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return "", newError(ErrorInvalidProfile, ProtocolTUIC, canonical)
		}
		return duration.String(), nil
	}
	return "", nil
}

func positiveIntegerParameter(parameters []QueryParameter, canonical string, maximum int) (int, error) {
	value, present := firstParameter(parameters, tuicAliases, canonical)
	if !present {
		return 0, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 || parsed > maximum {
		return 0, newError(ErrorInvalidProfile, ProtocolTUIC, canonical)
	}
	return parsed, nil
}

func parseBoolean(raw string, protocol Protocol, field string, numericOnly bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1":
		return true, nil
	case "0":
		return false, nil
	case "true":
		if !numericOnly {
			return true, nil
		}
	case "false":
		if !numericOnly {
			return false, nil
		}
	}
	return false, newError(ErrorInvalidProfile, protocol, field)
}

func isUUID(raw string) bool {
	if len(raw) != 36 {
		return false
	}
	for index, char := range raw {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func looksLikeUUID(raw string) bool {
	return len(raw) == 36 || strings.Count(raw, "-") == 4
}

func tuicFieldObservations(authority authorityParts, parameters []QueryParameter, generation int) []TUICFieldObservation {
	observations := make([]TUICFieldObservation, 0, len(parameters)+2)
	if authority.hasUserInfo {
		if strings.Contains(authority.rawUserInfo, ":") {
			observations = append(observations,
				TUICFieldObservation{Field: "uuid", SourceName: NewSensitiveValue("userinfo.uuid"), FieldClass: TUICProvenanceProtocolNative, SourceProvenance: TUICProvenanceProtocolNative},
				TUICFieldObservation{Field: "password", SourceName: NewSensitiveValue("userinfo.password"), FieldClass: TUICProvenanceProtocolNative, SourceProvenance: TUICProvenanceProtocolNative},
			)
		} else if generation == 4 {
			observations = append(observations, TUICFieldObservation{Field: "token", SourceName: NewSensitiveValue("userinfo.token"), FieldClass: TUICProvenanceV4, SourceProvenance: TUICProvenanceV4})
		} else {
			observations = append(observations, TUICFieldObservation{Field: "uuid", SourceName: NewSensitiveValue("userinfo.uuid"), FieldClass: TUICProvenanceProtocolNative, SourceProvenance: TUICProvenanceProtocolNative})
		}
	}
	for _, parameter := range parameters {
		source := strings.ToLower(parameter.Key)
		field, known := tuicAliases[source]
		provenance := TUICProvenanceUnknown
		if known {
			provenance = tuicParameterProvenance(source, field, generation)
		} else {
			field = "unknown"
		}
		observations = append(observations, TUICFieldObservation{
			Field:            field,
			SourceName:       NewSensitiveValue(parameter.Key),
			FieldClass:       tuicFieldClass(field, generation),
			SourceProvenance: provenance,
		})
	}
	return observations
}

func tuicFieldClass(field string, generation int) TUICFieldProvenance {
	if generation == 4 || field == "token" {
		return TUICProvenanceV4
	}
	switch field {
	case "uuid", "password":
		return TUICProvenanceProtocolNative
	case "sni", "alpn", "skip-cert-verify", "congestion-controller", "udp-relay-mode", "zero-rtt", "heartbeat":
		return TUICProvenanceCommonClient
	case "disable-sni", "request-timeout", "fast-open", "max-open-streams", "max-udp-relay-packet-size":
		return TUICProvenanceMihomo
	case "udp-over-stream":
		return TUICProvenanceSingBox
	default:
		return TUICProvenanceUnknown
	}
}

func tuicParameterProvenance(source, field string, generation int) TUICFieldProvenance {
	if field == "token" || generation == 4 && (field == "uuid" || field == "password") {
		return TUICProvenanceV4
	}
	switch source {
	case "disable-sni", "disable_sni", "reduce-rtt", "reduce_rtt",
		"heartbeat-interval", "heartbeat_interval", "request-timeout", "request_timeout",
		"fast-open", "fast_open", "max-open-streams", "max_open_streams",
		"max-udp-relay-packet-size", "max_udp_relay_packet_size",
		"congestion-controller", "udp-relay-mode":
		return TUICProvenanceMihomo
	case "server_name", "allow_insecure", "congestion_control", "udp_relay_mode",
		"udp-over-stream", "udp_over_stream", "zero-rtt-handshake", "zero_rtt_handshake", "heartbeat":
		return TUICProvenanceSingBox
	default:
		return TUICProvenanceCommonClient
	}
}

func allDigits(raw string) bool {
	if raw == "" {
		return false
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (tuicAdapter) Validate(profile *Profile) error {
	if profile == nil || profile.Protocol != ProtocolTUIC {
		return newError(ErrorInvalidProfile, ProtocolTUIC, "profile")
	}
	data, ok := profile.Data.(TUICData)
	if !ok {
		return newError(ErrorInvalidProfile, ProtocolTUIC, "data")
	}
	if _, err := normalizeHost(profile.Server, ProtocolTUIC); err != nil {
		return err
	}
	if _, err := parsePortSpec(profile.Port.Expression, ProtocolTUIC, false); err != nil {
		return err
	}
	switch data.Generation {
	case 4:
		if !data.Token.IsSet() {
			return newError(ErrorMissingCredential, ProtocolTUIC, "token")
		}
	case 5:
		if !data.UUID.IsSet() {
			return newError(ErrorMissingCredential, ProtocolTUIC, "uuid")
		}
		if !isUUID(data.UUID.Reveal()) {
			return newError(ErrorInvalidProfile, ProtocolTUIC, "uuid")
		}
		if !data.Password.IsSet() {
			return newError(ErrorMissingCredential, ProtocolTUIC, "password")
		}
	default:
		return newError(ErrorUnsupportedGeneration, ProtocolTUIC, "generation")
	}
	switch data.CongestionController {
	case "cubic", "new_reno", "bbr":
	default:
		return newError(ErrorInvalidProfile, ProtocolTUIC, "congestion_controller")
	}
	switch data.UDPRelayMode {
	case "native", "quic":
	default:
		return newError(ErrorInvalidProfile, ProtocolTUIC, "udp_relay_mode")
	}
	if data.UDPOverStream {
		if _, present := firstParameter(profile.QueryParameters, tuicAliases, "udp-relay-mode"); present {
			return newError(ErrorInvalidProfile, ProtocolTUIC, "udp_relay_mode")
		}
	}
	if data.SNI != "" {
		if _, err := normalizeHost(data.SNI, ProtocolTUIC); err != nil {
			return newError(ErrorInvalidProfile, ProtocolTUIC, "sni")
		}
	}
	for _, alpn := range data.ALPN {
		if strings.TrimSpace(alpn) == "" || strings.ContainsAny(alpn, "\r\n\x00") {
			return newError(ErrorInvalidProfile, ProtocolTUIC, "alpn")
		}
	}
	return nil
}

func (adapter tuicAdapter) Canonicalize(profile *Profile) (*Profile, error) {
	if err := adapter.Validate(profile); err != nil {
		return nil, err
	}
	canonical := cloneProfile(profile)
	canonical.Server, _ = normalizeHost(profile.Server, ProtocolTUIC)
	canonical.Port, _ = parsePortSpec(profile.Port.Expression, ProtocolTUIC, false)
	canonical.Port.Explicit = profile.Port.Explicit
	data := canonical.Data.(TUICData)
	if data.Generation == 5 {
		data.UUID = NewSensitiveValue(strings.ToLower(data.UUID.Reveal()))
	}
	data.SNI = strings.ToLower(strings.TrimSpace(data.SNI))
	data.CongestionController = strings.ToLower(strings.TrimSpace(data.CongestionController))
	data.UDPRelayMode = strings.ToLower(strings.TrimSpace(data.UDPRelayMode))
	data.ALPN = append([]string(nil), data.ALPN...)
	data.FieldObservations = append([]TUICFieldObservation(nil), data.FieldObservations...)
	canonical.Data = data
	return canonical, nil
}

func (adapter tuicAdapter) SerializeCanonical(profile *Profile) (SerializationResult, error) {
	if err := adapter.Validate(profile); err != nil {
		return SerializationResult{}, err
	}
	data := profile.Data.(TUICData)
	if data.Generation != 5 {
		return SerializationResult{}, newError(ErrorCompatibilityOnlyInput, ProtocolTUIC, "generation")
	}
	primary := make([]canonicalParameter, 0, 12)
	if data.SNI != "" || hasParameter(profile.QueryParameters, tuicAliases, "sni") {
		primary = append(primary, canonicalParameter{key: "sni", value: data.SNI, hasValue: true})
	}
	if len(data.ALPN) > 0 || hasParameter(profile.QueryParameters, tuicAliases, "alpn") {
		primary = append(primary, canonicalParameter{key: "alpn", value: strings.Join(data.ALPN, ","), hasValue: true})
	}
	if data.SkipCertificateVerification || hasParameter(profile.QueryParameters, tuicAliases, "skip-cert-verify") {
		primary = append(primary, canonicalParameter{key: "skip-cert-verify", value: booleanQueryValue(data.SkipCertificateVerification), hasValue: true})
	}
	if data.DisableSNI || hasParameter(profile.QueryParameters, tuicAliases, "disable-sni") {
		primary = append(primary, canonicalParameter{key: "disable-sni", value: booleanQueryValue(data.DisableSNI), hasValue: true})
	}
	if data.CongestionController != "cubic" || hasParameter(profile.QueryParameters, tuicAliases, "congestion-controller") {
		primary = append(primary, canonicalParameter{key: "congestion-controller", value: data.CongestionController, hasValue: true})
	}
	if data.UDPRelayMode != "native" || hasParameter(profile.QueryParameters, tuicAliases, "udp-relay-mode") {
		primary = append(primary, canonicalParameter{key: "udp-relay-mode", value: data.UDPRelayMode, hasValue: true})
	}
	if data.UDPOverStream || hasParameter(profile.QueryParameters, tuicAliases, "udp-over-stream") {
		primary = append(primary, canonicalParameter{key: "udp-over-stream", value: booleanQueryValue(data.UDPOverStream), hasValue: true})
	}
	if data.ZeroRTT || hasParameter(profile.QueryParameters, tuicAliases, "zero-rtt") {
		primary = append(primary, canonicalParameter{key: "zero-rtt", value: booleanQueryValue(data.ZeroRTT), hasValue: true})
	}
	if data.Heartbeat != "" || hasParameter(profile.QueryParameters, tuicAliases, "heartbeat") {
		primary = append(primary, canonicalParameter{key: "heartbeat", value: data.Heartbeat, hasValue: true})
	}
	if data.RequestTimeout != "" || hasParameter(profile.QueryParameters, tuicAliases, "request-timeout") {
		primary = append(primary, canonicalParameter{key: "request-timeout", value: data.RequestTimeout, hasValue: true})
	}
	if data.FastOpen || hasParameter(profile.QueryParameters, tuicAliases, "fast-open") {
		primary = append(primary, canonicalParameter{key: "fast-open", value: booleanQueryValue(data.FastOpen), hasValue: true})
	}
	if data.MaxOpenStreams > 0 || hasParameter(profile.QueryParameters, tuicAliases, "max-open-streams") {
		primary = append(primary, canonicalParameter{key: "max-open-streams", value: strconv.Itoa(data.MaxOpenStreams), hasValue: true})
	}
	if data.MaxUDPRelayPacketSize > 0 || hasParameter(profile.QueryParameters, tuicAliases, "max-udp-relay-packet-size") {
		primary = append(primary, canonicalParameter{key: "max-udp-relay-packet-size", value: strconv.Itoa(data.MaxUDPRelayPacketSize), hasValue: true})
	}
	query := canonicalQuery(profile.QueryParameters, primary, tuicAliases, "token", "uuid", "password")
	userInfo := percentEncode(data.UUID.Reveal()) + ":" + percentEncode(data.Password.Reveal())
	uri := canonicalURI("tuic", userInfo, formatHostPort(profile.Server, profile.Port), query, profile.DisplayName)
	return SerializationResult{
		URI:                NewSensitiveValue(uri),
		Mode:               CanonicalSerialization,
		Exact:              false,
		SemanticallyStable: true,
		Warnings:           append([]Warning(nil), profile.Warnings...),
	}, nil
}

func booleanQueryValue(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func (adapter tuicAdapter) FingerprintInput(profile *Profile) (SensitiveValue, error) {
	if err := adapter.Validate(profile); err != nil {
		return SensitiveValue{}, err
	}
	data := profile.Data.(TUICData)
	fields := []canonicalParameter{
		{key: "generation", value: strconv.Itoa(data.Generation), hasValue: true},
		{key: "uuid", value: data.UUID.Reveal(), hasValue: true},
		{key: "password", value: data.Password.Reveal(), hasValue: true},
		{key: "token", value: data.Token.Reveal(), hasValue: true},
		{key: "sni", value: strings.ToLower(data.SNI), hasValue: true},
		{key: "alpn", value: strings.Join(data.ALPN, ","), hasValue: true},
		{key: "skip_certificate_verification", value: boolString(data.SkipCertificateVerification), hasValue: true},
		{key: "disable_sni", value: boolString(data.DisableSNI), hasValue: true},
		{key: "congestion_controller", value: data.CongestionController, hasValue: true},
		{key: "udp_relay_mode", value: data.UDPRelayMode, hasValue: true},
		{key: "udp_over_stream", value: boolString(data.UDPOverStream), hasValue: true},
		{key: "zero_rtt", value: boolString(data.ZeroRTT), hasValue: true},
		{key: "heartbeat", value: data.Heartbeat, hasValue: true},
		{key: "request_timeout", value: data.RequestTimeout, hasValue: true},
		{key: "fast_open", value: boolString(data.FastOpen), hasValue: true},
		{key: "max_open_streams", value: strconv.Itoa(data.MaxOpenStreams), hasValue: true},
		{key: "max_udp_relay_packet_size", value: strconv.Itoa(data.MaxUDPRelayPacketSize), hasValue: true},
	}
	return semanticFingerprintInput(profile, fields, fingerprintExtraParameterLines(profile.QueryParameters, tuicAliases)), nil
}
