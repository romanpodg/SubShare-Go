package profiles

import (
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type uriComponents struct {
	scheme      string
	authority   string
	rawQuery    string
	fragment    string
	hasQuery    bool
	hasFragment bool
}

type authorityParts struct {
	rawUserInfo string
	hasUserInfo bool
	host        string
	port        PortSpec
}

type canonicalParameter struct {
	key      string
	value    string
	hasValue bool
	primary  bool
}

func detectScheme(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	separator := strings.IndexByte(trimmed, ':')
	if separator <= 0 {
		return "", newError(ErrorUnsupportedScheme, "", "scheme")
	}
	scheme := trimmed[:separator]
	for index, char := range scheme {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(index > 0 && ((char >= '0' && char <= '9') || char == '+' || char == '-' || char == '.')) {
			continue
		}
		return "", newError(ErrorUnsupportedScheme, "", "scheme")
	}
	return strings.ToLower(scheme), nil
}

func splitStandardURI(raw string, protocol Protocol) (uriComponents, error) {
	trimmed := strings.TrimSpace(raw)
	scheme, err := detectScheme(trimmed)
	if err != nil {
		return uriComponents{}, err
	}
	prefixLength := len(scheme) + 1
	if len(trimmed) < prefixLength+2 || trimmed[prefixLength:prefixLength+2] != "//" {
		return uriComponents{}, newError(ErrorInvalidAuthority, protocol, "uri")
	}
	remainder := trimmed[prefixLength+2:]
	components := uriComponents{scheme: scheme}
	if before, after, found := strings.Cut(remainder, "#"); found {
		remainder = before
		components.hasFragment = true
		decoded, decodeErr := url.PathUnescape(after)
		if decodeErr != nil || !utf8.ValidString(decoded) {
			return uriComponents{}, newError(ErrorInvalidEncoding, protocol, "fragment")
		}
		components.fragment = decoded
	}
	if before, after, found := strings.Cut(remainder, "?"); found {
		remainder = before
		components.hasQuery = true
		components.rawQuery = after
	}
	if slash := strings.IndexByte(remainder, '/'); slash >= 0 {
		if strings.Trim(remainder[slash:], "/") != "" {
			return uriComponents{}, newError(ErrorInvalidAuthority, protocol, "path")
		}
		remainder = remainder[:slash]
	}
	if remainder == "" {
		return uriComponents{}, newError(ErrorInvalidAuthority, protocol, "authority")
	}
	components.authority = remainder
	return components, nil
}

func parseAuthority(raw string, protocol Protocol, defaultPort string, allowExpression bool) (authorityParts, error) {
	parts := authorityParts{}
	hostPort := raw
	if at := strings.LastIndexByte(raw, '@'); at >= 0 {
		parts.hasUserInfo = true
		parts.rawUserInfo = raw[:at]
		hostPort = raw[at+1:]
		if strings.Contains(parts.rawUserInfo, "@") {
			return authorityParts{}, newError(ErrorInvalidAuthority, protocol, "userinfo")
		}
	}
	if hostPort == "" {
		return authorityParts{}, newError(ErrorInvalidAuthority, protocol, "host")
	}

	rawHost := ""
	rawPort := ""
	portSpecified := false
	bracketedHost := false
	if strings.HasPrefix(hostPort, "[") {
		bracketedHost = true
		closing := strings.IndexByte(hostPort, ']')
		if closing <= 1 {
			return authorityParts{}, newError(ErrorInvalidAuthority, protocol, "host")
		}
		rawHost = hostPort[1:closing]
		tail := hostPort[closing+1:]
		if tail != "" {
			if !strings.HasPrefix(tail, ":") || len(tail) == 1 {
				return authorityParts{}, newError(ErrorInvalidAuthority, protocol, "port")
			}
			portSpecified = true
			rawPort = tail[1:]
		}
	} else {
		switch strings.Count(hostPort, ":") {
		case 0:
			rawHost = hostPort
		case 1:
			portSpecified = true
			rawHost, rawPort, _ = strings.Cut(hostPort, ":")
		default:
			return authorityParts{}, newError(ErrorInvalidAuthority, protocol, "host")
		}
	}

	host, err := url.PathUnescape(rawHost)
	if err != nil {
		return authorityParts{}, newError(ErrorInvalidEncoding, protocol, "host")
	}
	if bracketedHost {
		parsedIP := net.ParseIP(host)
		if parsedIP == nil || parsedIP.To4() != nil {
			return authorityParts{}, newError(ErrorInvalidAuthority, protocol, "host")
		}
	}
	parts.host, err = normalizeHost(host, protocol)
	if err != nil {
		return authorityParts{}, err
	}
	if portSpecified && rawPort == "" {
		return authorityParts{}, newError(ErrorInvalidPort, protocol, "port")
	}
	if !portSpecified {
		rawPort = defaultPort
	}
	if rawPort == "" {
		return authorityParts{}, newError(ErrorInvalidPort, protocol, "port")
	}
	parts.port, err = parsePortSpec(rawPort, protocol, allowExpression)
	if err != nil {
		return authorityParts{}, err
	}
	parts.port.Explicit = portSpecified
	return parts, nil
}

func normalizeHost(raw string, protocol Protocol) (string, error) {
	if !utf8.ValidString(raw) {
		return "", newError(ErrorInvalidEncoding, protocol, "host")
	}
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", newError(ErrorInvalidAuthority, protocol, "host")
	}
	for _, char := range host {
		if unicode.IsSpace(char) || unicode.IsControl(char) || strings.ContainsRune("\\/@?#[]", char) {
			return "", newError(ErrorInvalidAuthority, protocol, "host")
		}
	}
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		return strings.ToLower(parsedIP.String()), nil
	}
	return strings.ToLower(host), nil
}

func parsePortSpec(raw string, protocol Protocol, allowExpression bool) (PortSpec, error) {
	for _, char := range raw {
		if unicode.IsSpace(char) {
			return PortSpec{}, newError(ErrorInvalidPort, protocol, "port")
		}
	}
	expression := strings.TrimSpace(raw)
	if expression == "" {
		return PortSpec{}, newError(ErrorInvalidPort, protocol, "port")
	}
	items := strings.Split(expression, ",")
	if !allowExpression && len(items) != 1 {
		return PortSpec{}, newError(ErrorInvalidPort, protocol, "port")
	}
	ranges := make([]PortRange, 0, len(items))
	canonical := make([]string, 0, len(items))
	containsRange := false
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			return PortSpec{}, newError(ErrorInvalidPort, protocol, "port")
		}
		bounds := strings.Split(item, "-")
		if len(bounds) > 2 || (!allowExpression && len(bounds) != 1) {
			return PortSpec{}, newError(ErrorInvalidPort, protocol, "port")
		}
		start, err := parsePortNumber(bounds[0])
		if err != nil {
			return PortSpec{}, newError(ErrorInvalidPort, protocol, "port")
		}
		end := start
		if len(bounds) == 2 {
			containsRange = true
			end, err = parsePortNumber(bounds[1])
			if err != nil || end < start {
				return PortSpec{}, newError(ErrorInvalidPort, protocol, "port")
			}
		}
		ranges = append(ranges, PortRange{Start: uint16(start), End: uint16(end)})
		if start == end {
			canonical = append(canonical, strconv.Itoa(start))
		} else {
			canonical = append(canonical, strconv.Itoa(start)+"-"+strconv.Itoa(end))
		}
	}
	kind := PortSingle
	if containsRange || len(ranges) > 1 {
		kind = PortExpression
	}
	return PortSpec{Expression: strings.Join(canonical, ","), Kind: kind, Ranges: ranges}, nil
}

func parsePortNumber(raw string) (int, error) {
	if raw == "" {
		return 0, strconv.ErrSyntax
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, strconv.ErrRange
	}
	return port, nil
}

func parseQuery(raw string, protocol Protocol) ([]QueryParameter, error) {
	if raw == "" {
		return []QueryParameter{}, nil
	}
	items := strings.Split(raw, "&")
	parameters := make([]QueryParameter, 0, len(items))
	for _, item := range items {
		if item == "" {
			return nil, newError(ErrorInvalidProfile, protocol, "query_parameter")
		}
		rawKey, rawValue, hasValue := strings.Cut(item, "=")
		if rawKey == "" {
			return nil, newError(ErrorInvalidProfile, protocol, "query_key")
		}
		key, err := url.QueryUnescape(rawKey)
		if err != nil || !utf8.ValidString(key) {
			return nil, newError(ErrorInvalidEncoding, protocol, "query_key")
		}
		value, err := url.QueryUnescape(rawValue)
		if err != nil || !utf8.ValidString(value) {
			return nil, newError(ErrorInvalidEncoding, protocol, "query_value")
		}
		parameters = append(parameters, QueryParameter{
			Key:      key,
			Value:    NewSensitiveValue(value),
			HasValue: hasValue,
			rawKey:   rawKey,
			rawValue: NewSensitiveValue(rawValue),
		})
	}
	return parameters, nil
}

func validateKnownParameterValues(parameters []QueryParameter, aliases map[string]string, protocol Protocol) error {
	for _, parameter := range parameters {
		canonical, known := aliases[strings.ToLower(parameter.Key)]
		if known && (!parameter.HasValue || parameter.Value.Reveal() == "") {
			return newError(ErrorInvalidProfile, protocol, canonical)
		}
	}
	return nil
}

func decodeUserInfo(raw string, protocol Protocol, field string) (string, error) {
	decoded, err := url.PathUnescape(raw)
	if err != nil || !utf8.ValidString(decoded) {
		return "", newError(ErrorInvalidEncoding, protocol, field)
	}
	return decoded, nil
}

func parameterAliases(entries ...string) map[string]string {
	aliases := make(map[string]string, len(entries))
	for _, entry := range entries {
		canonical, aliasText, _ := strings.Cut(entry, "=")
		if aliasText == "" {
			aliasText = canonical
		}
		for _, alias := range strings.Split(aliasText, ",") {
			aliases[strings.ToLower(strings.TrimSpace(alias))] = canonical
		}
	}
	return aliases
}

func firstParameter(parameters []QueryParameter, aliases map[string]string, canonical string) (string, bool) {
	for _, parameter := range parameters {
		if aliases[strings.ToLower(parameter.Key)] == canonical {
			return parameter.Value.Reveal(), true
		}
	}
	return "", false
}

func hasParameter(parameters []QueryParameter, aliases map[string]string, canonical string) bool {
	_, present := firstParameter(parameters, aliases, canonical)
	return present
}

func unknownParameters(parameters []QueryParameter, aliases map[string]string) []QueryParameter {
	unknown := make([]QueryParameter, 0)
	for _, parameter := range parameters {
		if _, known := aliases[strings.ToLower(parameter.Key)]; !known {
			unknown = append(unknown, parameter)
		}
	}
	return unknown
}

func duplicateWarnings(parameters []QueryParameter, aliases map[string]string) []Warning {
	type occurrence struct {
		value    string
		hasValue bool
		known    bool
	}
	groups := make(map[string][]occurrence)
	for _, parameter := range parameters {
		key := parameter.Key
		known := false
		if canonical, recognized := aliases[strings.ToLower(parameter.Key)]; recognized {
			key = canonical
			known = true
		}
		groups[key] = append(groups[key], occurrence{
			value:    parameter.Value.Reveal(),
			hasValue: parameter.HasValue,
			known:    known,
		})
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	warnings := make([]Warning, 0)
	for _, key := range keys {
		values := groups[key]
		if len(values) < 2 {
			continue
		}
		conflicting := false
		for _, value := range values[1:] {
			if value.value != values[0].value || value.hasValue != values[0].hasValue {
				conflicting = true
				break
			}
		}
		if conflicting {
			message := "conflicting duplicate unknown query values were preserved"
			if values[0].known {
				message = "conflicting duplicate known query values were preserved; the first value was selected"
			}
			warnings = append(warnings, Warning{Code: WarningAmbiguousParameter, Message: message})
			continue
		}
		warnings = append(warnings, Warning{Code: WarningDuplicateParameter, Message: "duplicate query values were preserved"})
	}
	return warnings
}

func canonicalQuery(original []QueryParameter, primary []canonicalParameter, aliases map[string]string, omitFirst ...string) string {
	parameters := append([]canonicalParameter(nil), primary...)
	replaced := make(map[string]bool, len(primary))
	for index := range parameters {
		parameters[index].primary = true
		replaced[parameters[index].key] = true
	}
	omitted := make(map[string]bool, len(omitFirst))
	for _, key := range omitFirst {
		omitted[key] = true
	}
	seen := make(map[string]int)
	for _, parameter := range original {
		canonical, known := aliases[strings.ToLower(parameter.Key)]
		if known {
			seen[canonical]++
			if seen[canonical] == 1 {
				if replaced[canonical] || omitted[canonical] {
					continue
				}
			}
			parameters = append(parameters, canonicalParameter{key: canonical, value: parameter.Value.Reveal(), hasValue: parameter.HasValue})
			continue
		}
		parameters = append(parameters, canonicalParameter{key: parameter.Key, value: parameter.Value.Reveal(), hasValue: parameter.HasValue})
	}
	sort.SliceStable(parameters, func(left, right int) bool {
		if parameters[left].key != parameters[right].key {
			return parameters[left].key < parameters[right].key
		}
		if parameters[left].primary != parameters[right].primary {
			return parameters[left].primary
		}
		// Values with the same semantic key retain source-relative order.
		// First/last duplicate policy can affect connectivity, so sorting them
		// by value would be a semantic change.
		return false
	})
	encoded := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		item := percentEncode(parameter.key)
		if parameter.hasValue {
			item += "=" + percentEncode(parameter.value)
		}
		encoded = append(encoded, item)
	}
	return strings.Join(encoded, "&")
}

func percentEncode(raw string) string {
	const hexadecimal = "0123456789ABCDEF"
	var builder strings.Builder
	for _, value := range []byte(raw) {
		if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
			(value >= '0' && value <= '9') || value == '-' || value == '.' || value == '_' || value == '~' {
			builder.WriteByte(value)
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(hexadecimal[value>>4])
		builder.WriteByte(hexadecimal[value&15])
	}
	return builder.String()
}

func percentEncodeAuth(raw string, allowColon bool) string {
	encoded := percentEncode(raw)
	if allowColon {
		encoded = strings.ReplaceAll(encoded, "%3A", ":")
	}
	return encoded
}

func formatHostPort(host string, port PortSpec) string {
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return host + ":" + port.Expression
}

func canonicalURI(scheme, userInfo, hostPort, query, fragment string) string {
	var builder strings.Builder
	builder.WriteString(scheme)
	builder.WriteString("://")
	if userInfo != "" {
		builder.WriteString(userInfo)
		builder.WriteByte('@')
	}
	builder.WriteString(hostPort)
	if query != "" {
		builder.WriteString("/?")
		builder.WriteString(query)
	}
	if fragment != "" {
		builder.WriteByte('#')
		builder.WriteString(percentEncode(fragment))
	}
	return builder.String()
}

func fullCapabilities(generation string) Capabilities {
	return Capabilities{
		Status:              CapabilityFull,
		Generation:          generation,
		Parse:               true,
		Validate:            true,
		CanonicalSerialize:  true,
		Generate:            true,
		Fingerprint:         true,
		ExactOriginalOutput: true,
	}
}

func cloneProfile(profile *Profile) *Profile {
	if profile == nil {
		return nil
	}
	cloned := *profile
	cloned.Port.Ranges = append([]PortRange(nil), profile.Port.Ranges...)
	cloned.QueryParameters = append([]QueryParameter(nil), profile.QueryParameters...)
	cloned.UnknownQueryParameters = append([]QueryParameter(nil), profile.UnknownQueryParameters...)
	cloned.Warnings = append([]Warning(nil), profile.Warnings...)
	return &cloned
}
