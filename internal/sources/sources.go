// Package sources fetches, parses and synchronises external subscription
// feeds. Fetching goes through an injectable HTTP client behind an SSRF guard,
// parsing is pure, and Sync applies the reconciliation policy through the
// Profile store's SourceSync transaction.
package sources

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

const (
	MaxBodyBytes   = 10 << 20 // 10 MB
	MaxImportItems = 10000
)

type Metadata struct {
	Title           string
	RefreshHours    int
	SupportURL      string
	WebPageURL      string
	Announce        string
	ContentType     string
	ContentDisp     string
	SourceFinalURL  string
	HTTPStatusCode  int
	HTTPStatusLabel string
}

type ParsedKey struct {
	Label                 string
	URL                   string
	Scheme                string
	Protocol              string
	Host                  string
	Port                  string
	Ref                   string
	ItemRef               string
	LineIndex             int
	Compatibility         string
	ProfileSchemaVersion  int
	Fingerprint           string
	FingerprintCandidates []string
	WarningCodes          []string
	InitialStatus         string
}

func (key ParsedKey) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "external_profile{protocol="+key.Protocol+", item_ref="+key.ItemRef+", raw=[redacted]}")
}

func (key ParsedKey) String() string {
	return "external_profile{protocol=" + key.Protocol + ", item_ref=" + key.ItemRef + ", raw=[redacted]}"
}

func (key ParsedKey) GoString() string { return key.String() }

func (key ParsedKey) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("external_profile_internal_only")
}

type ParseResult struct {
	DetectedFormat string
	Metadata       Metadata
	Keys           []ParsedKey
	Items          []ImportItem
	Counts         Counts
	Warnings       []string
}

func (result ParseResult) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, fmt.Sprintf("external_subscription{format=%s, items=%d, raw=[redacted]}", result.DetectedFormat, len(result.Items)))
}

func (result ParseResult) String() string {
	return fmt.Sprintf("external_subscription{format=%s, items=%d, raw=[redacted]}", result.DetectedFormat, len(result.Items))
}

func (result ParseResult) GoString() string { return result.String() }

type ImportItem struct {
	ItemRef       string   `json:"item_ref"`
	LineIndex     int      `json:"line_index"`
	Protocol      string   `json:"protocol"`
	Scheme        string   `json:"scheme"`
	DisplayName   string   `json:"display_name"`
	Label         string   `json:"label"`
	Host          string   `json:"host"`
	Port          string   `json:"port"`
	Compatibility string   `json:"compatibility"`
	Status        string   `json:"status"`
	Warnings      []string `json:"warnings"`
	ErrorCode     string   `json:"error_code,omitempty"`
	// URLShort is a deprecated compatibility field. It contains only a safe
	// endpoint summary and never any userinfo, query string, or fragment.
	URLShort string `json:"url_short"`
}

type Counts struct {
	Accepted          int `json:"accepted"`
	Added             int `json:"added"`
	Rejected          int `json:"rejected"`
	Duplicate         int `json:"duplicate"`
	Updated           int `json:"updated"`
	Unchanged         int `json:"unchanged"`
	CompatibilityOnly int `json:"compatibility_only"`
	Ambiguous         int `json:"ambiguous"`
	Unsupported       int `json:"unsupported"`
	Removed           int `json:"removed"`
}

type SyncResult struct {
	Imported int
	Skipped  int
	Counts   Counts
	Items    []ImportItem
}

const ExternalProfileSchemaVersion = 1

const (
	StatusAccepted          = model.ExternalImportStatusAccepted
	StatusAdded             = model.ExternalImportStatusAdded
	StatusRejected          = model.ExternalImportStatusRejected
	StatusDuplicate         = model.ExternalImportStatusDuplicate
	StatusUpdated           = model.ExternalImportStatusUpdated
	StatusUnchanged         = model.ExternalImportStatusUnchanged
	StatusCompatibilityOnly = model.ExternalImportStatusCompatibilityOnly
	StatusAmbiguous         = model.ExternalImportStatusAmbiguous
	StatusUnsupported       = model.ExternalImportStatusUnsupported
)

type HWIDProfile struct {
	PassHWID  bool
	Version   string
	ModelName string
	HWID      string
}

func NormalizeKeyInsertMode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "top", "bottom":
		return value
	default:
		return "bottom"
	}
}

func NonNilWarnings(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func DecodeHeaderValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), "base64:") {
		decoded := profileconfig.DecodeBase64String(value)
		if decoded != "" {
			return strings.TrimSpace(decoded)
		}
		value = strings.TrimSpace(value[7:])
	}
	return strings.TrimSpace(value)
}

func parsePositiveInt(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func trimUTF8BOM(raw string) string {
	return strings.TrimPrefix(raw, "\uFEFF")
}

// maybeDecodeBase64SubscriptionBody accepts standard or URL-safe Base64 with
// either canonical padding or no padding. An optional case-insensitive
// "base64:" marker and ASCII whitespace between encoded characters are
// accepted. A Base64-looking body is an explicit format attempt: malformed
// encoding or decoded data without a recognized subscription format returns a
// stable error instead of being reinterpreted as an unknown URI.

// maybeDecodeBase64SubscriptionBody accepts standard or URL-safe Base64 with
// either canonical padding or no padding. An optional case-insensitive
// "base64:" marker and ASCII whitespace between encoded characters are
// accepted. A Base64-looking body is an explicit format attempt: malformed
// encoding or decoded data without a recognized subscription format returns a
// stable error instead of being reinterpreted as an unknown URI.
func maybeDecodeBase64SubscriptionBody(raw string) (string, bool, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", false, nil
	}
	if bodyLooksLikeSupportedSubscription(candidate) {
		return "", false, nil
	}
	explicit := strings.HasPrefix(strings.ToLower(candidate), "base64:")
	if explicit {
		candidate = strings.TrimSpace(candidate[len("base64:"):])
		if candidate == "" {
			return "", true, fmt.Errorf("invalid_base64_subscription")
		}
	}

	encoded, base64Like := compactBase64Body(candidate)
	if !base64Like || len(encoded) < 24 {
		if explicit {
			return "", true, fmt.Errorf("invalid_base64_subscription")
		}
		return "", false, nil
	}
	if strings.ContainsAny(encoded, "+/") && strings.ContainsAny(encoded, "-_") {
		return "", true, fmt.Errorf("invalid_base64_subscription")
	}

	encoding := base64EncodingFor(encoded)
	if encoding.DecodedLen(len(encoded)) > MaxBodyBytes {
		return "", true, fmt.Errorf("subscription_body_too_large")
	}
	decodedBytes, err := encoding.DecodeString(encoded)
	if err != nil {
		return "", true, fmt.Errorf("invalid_base64_subscription")
	}
	decoded := string(decodedBytes)
	decoded = strings.TrimSpace(trimUTF8BOM(decoded))
	if decoded == "" {
		return "", true, fmt.Errorf("invalid_base64_subscription")
	}

	if bodyLooksLikeSupportedSubscription(decoded) {
		return decoded, true, nil
	}
	return "", true, fmt.Errorf("invalid_base64_subscription")
}

// compactBase64Body strips ASCII whitespace and reports whether every
// remaining character belongs to the standard or URL-safe Base64 alphabet.
func compactBase64Body(candidate string) (string, bool) {
	var compact strings.Builder
	compact.Grow(len(candidate))
	base64Like := true
	for _, char := range candidate {
		switch {
		case char == '\r' || char == '\n' || char == '\t' || char == ' ':
			continue
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9', char == '+', char == '/', char == '-', char == '_', char == '=':
			compact.WriteRune(char)
		default:
			base64Like = false
		}
	}
	return compact.String(), base64Like
}

func base64EncodingFor(encoded string) *base64.Encoding {
	urlSafe := strings.ContainsAny(encoded, "-_")
	padded := strings.Contains(encoded, "=")
	switch {
	case urlSafe && padded:
		return base64.URLEncoding.Strict()
	case urlSafe:
		return base64.RawURLEncoding.Strict()
	case padded:
		return base64.StdEncoding.Strict()
	default:
		return base64.RawStdEncoding.Strict()
	}
}

// externalURIScheme performs explicit ASCII URI-scheme recognition. Schemes
// are case-insensitive, but the remainder of the URI is left untouched.

// externalURIScheme performs explicit ASCII URI-scheme recognition. Schemes
// are case-insensitive, but the remainder of the URI is left untouched.
func externalURIScheme(raw string) string {
	value := strings.TrimSpace(raw)
	separator := strings.IndexByte(value, ':')
	if separator <= 0 || len(value) < separator+3 || value[separator+1:separator+3] != "//" {
		return ""
	}
	for index, char := range value[:separator] {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (index > 0 && ((char >= '0' && char <= '9') || char == '+' || char == '-' || char == '.')) {
			continue
		}
		return ""
	}
	return strings.ToLower(value[:separator])
}

func bodyLooksLikeSupportedSubscription(body string) bool {
	trimmed := strings.TrimSpace(trimUTF8BOM(body))
	if json.Valid([]byte(trimmed)) {
		return true
	}
	for _, line := range strings.Split(strings.ReplaceAll(trimmed, "\r\n", "\n"), "\n") {
		switch externalURIScheme(line) {
		case "vless", "vmess", "trojan", "ss", "hysteria2", "hy2", "tuic", "hysteria", "hysteria2+realm", "realm", "realm+http":
			return true
		}
	}
	return false
}

func KeyRef(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:16])
}

func ExtractJSONLabel(root map[string]any, fallback string) string {
	if value, ok := root["remarks"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if meta, ok := root["meta"].(map[string]any); ok {
		if value, ok := meta["serverDescription"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if value, ok := root["tag"].(string); ok && strings.TrimSpace(value) != "" && !profileconfig.IsGenericXrayOutboundTag(value) {
		return strings.TrimSpace(value)
	}
	if outbounds, ok := root["outbounds"].([]any); ok && len(outbounds) > 0 {
		if outbound, ok := outbounds[0].(map[string]any); ok {
			if value, ok := outbound["tag"].(string); ok && strings.TrimSpace(value) != "" && !profileconfig.IsGenericXrayOutboundTag(value) {
				return strings.TrimSpace(value)
			}
		}
	}
	return fallback
}

func buildExternalItemRef(raw string, lineIndex int, fingerprintKeys [][]byte) (string, error) {
	if len(fingerprintKeys) == 0 || len(fingerprintKeys[0]) == 0 {
		return "", fmt.Errorf("profile_fingerprint_key_unavailable")
	}
	digest := hmac.New(sha256.New, fingerprintKeys[0])
	_, _ = digest.Write([]byte("subshare-preview-item-v1\x00"))
	_, _ = digest.Write([]byte(strconv.Itoa(lineIndex)))
	_, _ = digest.Write([]byte{'\x00'})
	_, _ = digest.Write([]byte(raw))
	return "ir1_" + hex.EncodeToString(digest.Sum(nil)[:16]), nil
}

func safeEndpointSummary(protocol, host, port string) string {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" {
		return protocol
	}
	if port == "" {
		return host
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + port
}

func safeExternalItem(key ParsedKey, status, errorCode string) ImportItem {
	warnings := append([]string(nil), key.WarningCodes...)
	if warnings == nil {
		warnings = []string{}
	}
	return ImportItem{
		ItemRef: key.ItemRef, LineIndex: key.LineIndex, Protocol: key.Protocol, Scheme: key.Scheme,
		DisplayName: key.Label, Host: key.Host, Port: key.Port, Compatibility: key.Compatibility,
		Label:  key.Label,
		Status: status, Warnings: warnings, ErrorCode: errorCode,
		URLShort: safeEndpointSummary(key.Protocol, key.Host, key.Port),
	}
}

func rejectedExternalItem(raw string, lineIndex int, scheme, status, errorCode string, fingerprintKeys [][]byte) (ImportItem, error) {
	itemRef, err := buildExternalItemRef(raw, lineIndex, fingerprintKeys)
	if err != nil {
		return ImportItem{}, err
	}
	protocol := scheme
	if protocol == "" {
		protocol = "unknown"
	}
	return ImportItem{
		ItemRef: itemRef, LineIndex: lineIndex, Protocol: protocol, Scheme: scheme,
		Compatibility: "unsupported", Status: status, Warnings: []string{}, ErrorCode: errorCode,
		URLShort: protocol,
	}, nil
}

func profileWarningCodes(profile *profiles.Profile) []string {
	seen := make(map[string]struct{})
	codes := make([]string, 0, len(profile.Warnings))
	for _, warning := range profile.Warnings {
		code := strings.TrimSpace(warning.Code)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	return codes
}

func ContainsWarningCode(codes []string, wanted ...string) bool {
	for _, code := range codes {
		for _, candidate := range wanted {
			if code == candidate {
				return true
			}
		}
	}
	return false
}

func parseExternalProfileItem(raw string, lineIndex int, scheme string, fingerprintKeys [][]byte) (*ParsedKey, ImportItem, error) {
	itemRef, err := buildExternalItemRef(raw, lineIndex, fingerprintKeys)
	if err != nil {
		return nil, ImportItem{}, err
	}
	profile, err := profiles.Parse(raw)
	if err != nil {
		code := string(profiles.ErrorCodeOf(err))
		if code == "" {
			code = "invalid_profile"
		}
		item, itemErr := rejectedExternalItem(raw, lineIndex, scheme, StatusRejected, code, fingerprintKeys)
		return nil, item, itemErr
	}
	original, err := profiles.Serialize(profile, profiles.OriginalSerialization)
	if err != nil || !original.Exact {
		item, itemErr := rejectedExternalItem(raw, lineIndex, scheme, StatusRejected, "original_serialization_unavailable", fingerprintKeys)
		return nil, item, itemErr
	}
	metadata := profile.SafeMetadata()
	warnings := profileWarningCodes(profile)
	compatibility := string(metadata.Capabilities.Status)
	status := StatusAccepted
	if metadata.Capabilities.Status == profiles.CapabilityReadOnly {
		status = StatusCompatibilityOnly
	} else if ContainsWarningCode(warnings, profiles.WarningAmbiguousParameter, profiles.WarningConflictingPreference) {
		status = StatusAmbiguous
	}
	fingerprints := make([]string, 0, len(fingerprintKeys))
	if metadata.Capabilities.Fingerprint {
		for _, key := range fingerprintKeys {
			fingerprint, fingerprintErr := profiles.Fingerprint(profile, key)
			if fingerprintErr != nil {
				code := string(profiles.ErrorCodeOf(fingerprintErr))
				if code == "" {
					code = "fingerprint_failed"
				}
				item, itemErr := rejectedExternalItem(raw, lineIndex, scheme, StatusRejected, code, fingerprintKeys)
				return nil, item, itemErr
			}
			fingerprints = append(fingerprints, fingerprint)
		}
	}
	label := strings.TrimSpace(metadata.DisplayName)
	if label == "" {
		label = fmt.Sprintf("Import %03d", lineIndex)
	}
	key := &ParsedKey{
		Label: label, URL: original.URI.Reveal(), Scheme: scheme, Protocol: string(metadata.Protocol),
		Host: metadata.Server, Port: metadata.Port, Ref: KeyRef(original.URI.Reveal()),
		ItemRef: itemRef, LineIndex: lineIndex, Compatibility: compatibility,
		ProfileSchemaVersion: ExternalProfileSchemaVersion, FingerprintCandidates: fingerprints,
		WarningCodes: warnings, InitialStatus: status,
	}
	if len(fingerprints) > 0 {
		key.Fingerprint = fingerprints[0]
	}
	item := safeExternalItem(*key, status, "")
	return key, item, nil
}

func externalDedupeIdentity(key ParsedKey) string {
	if key.Fingerprint != "" {
		return "profile:" + key.Fingerprint
	}
	return "raw:" + key.Ref
}

func appendExternalParsedItem(keys *[]ParsedKey, items *[]ImportItem, seen map[string]struct{}, key ParsedKey) {
	identity := externalDedupeIdentity(key)
	if _, exists := seen[identity]; exists {
		*items = append(*items, safeExternalItem(key, StatusDuplicate, ""))
		return
	}
	seen[identity] = struct{}{}
	*keys = append(*keys, key)
	*items = append(*items, safeExternalItem(key, key.InitialStatus, ""))
}

func parseExternalLinkBody(body string, fingerprintKeys [][]byte) ([]ParsedKey, []ImportItem, error) {
	if strings.Count(body, "\n")+1 > MaxImportItems {
		return nil, nil, fmt.Errorf("too_many_source_items")
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	keys := make([]ParsedKey, 0, len(lines))
	items := make([]ImportItem, 0, len(lines))
	seen := make(map[string]struct{})
	for index, line := range lines {
		raw := strings.TrimSpace(line)
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, ";") {
			continue
		}
		var err error
		if strings.HasPrefix(raw, "{") && json.Valid([]byte(raw)) {
			err = appendExternalJSONLine(raw, index+1, fingerprintKeys, &keys, &items, seen)
		} else {
			err = appendExternalURILine(raw, index+1, fingerprintKeys, &keys, &items, seen)
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return keys, items, nil
}

// appendExternalJSONLine folds one inline JSON document into the link-list
// result: its rejected items are reported, its keys go through the shared
// dedupe path, and every other item status is dropped.
func appendExternalJSONLine(raw string, lineIndex int, fingerprintKeys [][]byte, keys *[]ParsedKey, items *[]ImportItem, seen map[string]struct{}) error {
	jsonKeys, jsonItems, err := parseExternalJSONBody(raw, lineIndex, fingerprintKeys)
	if err != nil {
		return err
	}
	for _, item := range jsonItems {
		if item.Status == StatusRejected {
			*items = append(*items, item)
		}
	}
	for _, key := range jsonKeys {
		appendExternalParsedItem(keys, items, seen, key)
	}
	return nil
}

func appendExternalURILine(raw string, lineIndex int, fingerprintKeys [][]byte, keys *[]ParsedKey, items *[]ImportItem, seen map[string]struct{}) error {
	scheme := externalURIScheme(raw)
	switch scheme {
	case "vless", "vmess", "trojan", "ss", "hysteria2", "hy2", "tuic":
		key, item, err := parseExternalProfileItem(raw, lineIndex, scheme, fingerprintKeys)
		if err != nil {
			return err
		}
		if key == nil {
			*items = append(*items, item)
			return nil
		}
		appendExternalParsedItem(keys, items, seen, *key)
		return nil
	case "hysteria", "hysteria2+realm", "realm", "realm+http":
		item, err := rejectedExternalItem(raw, lineIndex, scheme, StatusUnsupported, "unsupported_protocol", fingerprintKeys)
		if err != nil {
			return err
		}
		*items = append(*items, item)
		return nil
	default:
		item, err := rejectedExternalItem(raw, lineIndex, scheme, StatusUnsupported, "unsupported_scheme", fingerprintKeys)
		if err != nil {
			return err
		}
		*items = append(*items, item)
		return nil
	}
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

func firstXrayJSONIdentity(object map[string]any) (xrayJSONIdentity, bool) {
	label := strings.TrimSpace(ExtractJSONLabel(object, ""))
	for _, outboundRaw := range profileconfig.AsArray(object["outbounds"]) {
		outbound, ok := profileconfig.AsObject(outboundRaw)
		if !ok {
			continue
		}
		protocol := strings.ToLower(profileconfig.AnyToString(outbound["protocol"]))
		if protocol == "" {
			continue
		}
		identity := xrayJSONIdentity{Protocol: protocol, Label: label}
		if identity.Label == "" {
			identity.Label = profileconfig.AnyToString(outbound["tag"])
		}
		settings, _ := profileconfig.AsObject(outbound["settings"])
		switch protocol {
		case "vless", "vmess":
			if nodes := profileconfig.AsArray(settings["vnext"]); len(nodes) > 0 {
				if node, nodeOK := profileconfig.AsObject(nodes[0]); nodeOK {
					identity.Host = profileconfig.AnyToString(node["address"])
					identity.Port = profileconfig.AnyToString(node["port"])
				}
			}
		case "trojan":
			if servers := profileconfig.AsArray(settings["servers"]); len(servers) > 0 {
				if server, serverOK := profileconfig.AsObject(servers[0]); serverOK {
					identity.Host = profileconfig.AnyToString(server["address"])
					identity.Port = profileconfig.AnyToString(server["port"])
				}
			}
		case "hysteria":
			identity.Host = profileconfig.AnyToString(settings["address"])
			identity.Port = profileconfig.AnyToString(settings["port"])
		}
		return identity, true
	}
	return xrayJSONIdentity{}, false
}

func safeRejectedXrayJSONItem(raw string, lineIndex int, identity xrayJSONIdentity, status, errorCode string, fingerprintKeys [][]byte) (ImportItem, error) {
	itemRef, err := buildExternalItemRef(raw, lineIndex, fingerprintKeys)
	if err != nil {
		return ImportItem{}, err
	}
	label := strings.TrimSpace(identity.Label)
	if label == "" {
		label = fmt.Sprintf("JSON %03d", lineIndex)
	}
	return ImportItem{
		ItemRef: itemRef, LineIndex: lineIndex, Protocol: identity.Protocol, Scheme: "xray-json",
		DisplayName: label, Label: label, Host: identity.Host, Port: identity.Port,
		Compatibility: "unsupported", Status: status, Warnings: []string{}, ErrorCode: errorCode,
		URLShort: safeEndpointSummary(identity.Protocol, identity.Host, identity.Port),
	}, nil
}

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
	settings, _ := profileconfig.AsObject(outbound["settings"])
	identity := xrayJSONIdentity{
		Protocol: "hysteria",
		Label:    strings.TrimSpace(ExtractJSONLabel(object, "")),
		Host:     profileconfig.AnyToString(settings["address"]),
		Port:     profileconfig.AnyToString(settings["port"]),
	}
	if identity.Label == "" {
		identity.Label = profileconfig.AnyToString(outbound["tag"])
	}
	stream, _ := profileconfig.AsObject(outbound["streamSettings"])
	hysteriaSettings, _ := profileconfig.AsObject(stream["hysteriaSettings"])

	if protocol, errorCode := xrayHysteriaVersionRejection(settings, hysteriaSettings); errorCode != "" {
		identity.Protocol = protocol
		item, err := safeRejectedXrayJSONItem(raw, lineIndex, identity, StatusUnsupported, errorCode, fingerprintKeys)
		return nil, &item, true, err
	}
	identity.Protocol = "hysteria2"
	reject := func() (*ParsedKey, *ImportItem, bool, error) {
		item, err := safeRejectedXrayJSONItem(raw, lineIndex, identity, StatusRejected, "invalid_hysteria2_json", fingerprintKeys)
		return nil, &item, true, err
	}

	host, hostOK := settings["address"].(string)
	host = strings.TrimSpace(host)
	port := strings.TrimSpace(identity.Port)
	auth, authOK := hysteriaSettings["auth"].(string)
	auth = strings.TrimSpace(auth)
	if !hostOK || !authOK || host == "" || port == "" || auth == "" || !xrayHysteria2ShapeValid(outbound, settings, stream, hysteriaSettings) {
		return reject()
	}

	query := url.Values{}
	if !xrayHysteria2TLSQuery(stream, query) {
		return reject()
	}
	hopPorts, ok := xrayHysteria2FinalMaskQuery(stream, query)
	if !ok {
		return reject()
	}
	if hopPorts != "" {
		port = hopPorts
	}

	authorityHost := host
	if strings.Contains(authorityHost, ":") && !strings.HasPrefix(authorityHost, "[") {
		authorityHost = "[" + authorityHost + "]"
	}
	profileURI := "hysteria2://" + url.User(auth).String() + "@" + authorityHost + ":" + port
	if encodedQuery := query.Encode(); encodedQuery != "" {
		profileURI += "?" + encodedQuery
	}
	label := strings.TrimSpace(identity.Label)
	if label == "" {
		label = fmt.Sprintf("JSON %03d", lineIndex)
	}
	profileURI += "#" + url.PathEscape(label)

	profile, err := profiles.Parse(profileURI)
	if err != nil {
		return reject()
	}
	serialized, err := profiles.Serialize(profile, profiles.CanonicalSerialization)
	if err != nil {
		return reject()
	}
	key, err := xrayHysteria2ParsedKey(profile, serialized, label, raw, lineIndex, fingerprintKeys)
	if err != nil {
		return nil, nil, true, err
	}
	item := safeExternalItem(*key, StatusAccepted, "")
	return key, &item, true, nil
}

// xrayHysteriaVersionRejection inspects both Hysteria version markers and
// returns the identity protocol plus rejection code for anything other than
// an unambiguous version 2. An empty code means version 2 was confirmed.
func xrayHysteriaVersionRejection(settings, hysteriaSettings map[string]any) (string, string) {
	settingsVersionRaw, settingsMarkerPresent := settings["version"]
	streamVersionRaw, streamMarkerPresent := hysteriaSettings["version"]
	settingsVersion, settingsVersionValid := xrayVersionValue(settingsVersionRaw)
	streamVersion, streamVersionValid := xrayVersionValue(streamVersionRaw)
	if !settingsMarkerPresent && !streamMarkerPresent {
		return "hysteria-unknown", "ambiguous_hysteria_version"
	}
	if (settingsMarkerPresent && !settingsVersionValid) || (streamMarkerPresent && !streamVersionValid) {
		return "hysteria-unknown", "unsupported_hysteria_version"
	}
	isV2 := (settingsMarkerPresent && settingsVersion == 2) || (streamMarkerPresent && streamVersion == 2)
	if !isV2 {
		if (!settingsMarkerPresent || settingsVersion == 1) && (!streamMarkerPresent || streamVersion == 1) {
			return "hysteria", "unsupported_hysteria_v1"
		}
		return "hysteria-unknown", "unsupported_hysteria_version"
	}
	if (settingsMarkerPresent && settingsVersion != 2) || (streamMarkerPresent && streamVersion != 2) {
		return "hysteria-unknown", "unsupported_hysteria_version"
	}
	return "", ""
}

func xrayHysteria2ShapeValid(outbound, settings, stream, hysteriaSettings map[string]any) bool {
	method := strings.ToLower(profileconfig.AnyToString(stream["method"]))
	network := strings.ToLower(profileconfig.AnyToString(stream["network"]))
	if (method == "" && network == "") || (method != "" && method != "hysteria") || (network != "" && network != "hysteria") {
		return false
	}
	return strings.EqualFold(profileconfig.AnyToString(stream["security"]), "tls") &&
		onlyObjectKeys(outbound, "tag", "protocol", "settings", "streamSettings") &&
		onlyObjectKeys(settings, "version", "address", "port") &&
		onlyObjectKeys(hysteriaSettings, "version", "auth") &&
		onlyObjectKeys(stream, "method", "network", "security", "hysteriaSettings", "tlsSettings", "finalmask")
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

func xrayHysteria2TLSQuery(stream map[string]any, query url.Values) bool {
	tlsSettings, tlsSettingsOK := profileconfig.AsObject(stream["tlsSettings"])
	if _, present := stream["tlsSettings"]; present && !tlsSettingsOK {
		return false
	}
	if tlsSettings != nil && !onlyObjectKeys(tlsSettings, "serverName", "pinnedPeerCertSha256", "allowInsecure", "alpn", "fingerprint") {
		return false
	}
	for _, field := range [...]struct{ key, param string }{{"serverName", "sni"}, {"pinnedPeerCertSha256", "pinSHA256"}} {
		value, present, ok := xrayTrimmedString(tlsSettings, field.key)
		if !ok {
			return false
		}
		if present {
			query.Set(field.param, value)
		}
	}
	if insecureRaw, present := tlsSettings["allowInsecure"]; present {
		insecure, ok := insecureRaw.(bool)
		if !ok {
			return false
		}
		if insecure {
			query.Set("insecure", "1")
		} else {
			query.Set("insecure", "0")
		}
	}
	alpnValues, alpnOK := xrayStringArray(tlsSettings, "alpn")
	if !alpnOK {
		return false
	}
	for _, alpn := range alpnValues {
		query.Add("alpn", alpn)
	}
	fingerprint, present, ok := xrayTrimmedString(tlsSettings, "fingerprint")
	if !ok {
		return false
	}
	if present {
		query.Set("fp", fingerprint)
	}
	return true
}

// xrayHysteria2FinalMaskQuery validates the finalmask block, adds salamander
// obfuscation to query and returns the udpHop port expression when present.
func xrayHysteria2FinalMaskQuery(stream map[string]any, query url.Values) (string, bool) {
	finalMask, finalMaskOK := profileconfig.AsObject(stream["finalmask"])
	if _, present := stream["finalmask"]; present && !finalMaskOK {
		return "", false
	}
	if finalMask != nil && !onlyObjectKeys(finalMask, "quicParams", "udp") {
		return "", false
	}
	hopPorts, ok := xrayHysteria2HopPorts(finalMask)
	if !ok {
		return "", false
	}
	masks := profileconfig.AsArray(finalMask["udp"])
	if _, present := finalMask["udp"]; present && masks == nil {
		return "", false
	}
	if len(masks) > 0 {
		if len(masks) != 1 {
			return "", false
		}
		mask, ok := profileconfig.AsObject(masks[0])
		maskSettings, settingsOK := profileconfig.AsObject(mask["settings"])
		if !ok || !settingsOK || profileconfig.AnyToString(mask["type"]) != "salamander" || !onlyObjectKeys(mask, "type", "settings") || !onlyObjectKeys(maskSettings, "password") {
			return "", false
		}
		query.Set("obfs", "salamander")
		query.Set("obfs-password", profileconfig.AnyToString(maskSettings["password"]))
	}
	return hopPorts, true
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
		c.items = append(c.items, ImportItem{
			ItemRef: itemRef, LineIndex: lineIndex, Protocol: "shadowsocks", Scheme: "sip008",
			Compatibility: "unsupported", Status: StatusUnsupported, Warnings: []string{},
			ErrorCode: "unsupported_sip008_format", URLShort: "shadowsocks",
		})
		return nil
	}
	if hysteriaKey, hysteriaItem, handled, hysteriaErr := parseXrayHysteria2Object(obj, raw, lineIndex, c.fingerprintKeys); handled {
		if hysteriaErr != nil {
			return hysteriaErr
		}
		if hysteriaKey == nil {
			c.items = append(c.items, *hysteriaItem)
			return nil
		}
		appendExternalParsedItem(&c.keys, &c.items, c.seen, *hysteriaKey)
		return nil
	}
	drafts, parseErr := profileconfig.ParseXrayJSONDrafts(raw)
	if parseErr != nil || len(drafts) == 0 {
		item, itemErr := c.undraftableXrayJSONItem(obj, raw, itemRef, lineIndex)
		if itemErr != nil {
			return itemErr
		}
		c.items = append(c.items, item)
		return nil
	}
	label := strings.TrimSpace(ExtractJSONLabel(obj, fmt.Sprintf("JSON %03d", lineIndex)))
	if label == "" {
		label = fmt.Sprintf("JSON %03d", lineIndex)
	}
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
	if identity.Protocol == "vless" || identity.Protocol == "vmess" || identity.Protocol == "trojan" {
		status = StatusRejected
		errorCode = "invalid_" + identity.Protocol + "_json"
	}
	return safeRejectedXrayJSONItem(raw, lineIndex, identity, status, errorCode, c.fingerprintKeys)
}

func CountItems(items []ImportItem) Counts {
	var counts Counts
	for _, item := range items {
		if item.Compatibility == string(profiles.CapabilityReadOnly) {
			counts.CompatibilityOnly++
		}
		if ContainsWarningCode(item.Warnings, profiles.WarningAmbiguousParameter, profiles.WarningConflictingPreference) {
			counts.Ambiguous++
		}
		switch item.Status {
		case StatusAccepted:
			counts.Accepted++
		case StatusAdded:
			counts.Added++
		case StatusRejected:
			counts.Rejected++
		case StatusDuplicate:
			counts.Duplicate++
		case StatusUpdated:
			counts.Updated++
		case StatusUnchanged:
			counts.Unchanged++
		case StatusCompatibilityOnly:
			if item.Compatibility != string(profiles.CapabilityReadOnly) {
				counts.CompatibilityOnly++
			}
		case StatusAmbiguous:
			if !ContainsWarningCode(item.Warnings, profiles.WarningAmbiguousParameter, profiles.WarningConflictingPreference) {
				counts.Ambiguous++
			}
		case StatusUnsupported:
			counts.Unsupported++
		}
	}
	return counts
}

func NoSupportedKeysError(parsed ParseResult) error {
	if len(parsed.Items) > 0 {
		allSIP008 := true
		for _, item := range parsed.Items {
			if item.ErrorCode != "unsupported_sip008_format" {
				allSIP008 = false
				break
			}
		}
		if allSIP008 {
			return fmt.Errorf("unsupported_sip008_format")
		}
	}
	return fmt.Errorf("no supported keys found in source response")
}

func FilterSelection(parsed ParseResult, selected []string) (ParseResult, error) {
	if len(selected) == 0 {
		return parsed, nil
	}
	byRef := make(map[string]ParsedKey, len(parsed.Keys))
	for _, key := range parsed.Keys {
		byRef[key.ItemRef] = key
	}
	wanted := make(map[string]struct{}, len(selected))
	filteredKeys := make([]ParsedKey, 0, len(selected))
	for _, itemRef := range selected {
		itemRef = strings.TrimSpace(itemRef)
		if itemRef == "" {
			return ParseResult{}, fmt.Errorf("invalid_item_reference")
		}
		if _, duplicate := wanted[itemRef]; duplicate {
			return ParseResult{}, fmt.Errorf("duplicate_item_reference")
		}
		key, exists := byRef[itemRef]
		if !exists {
			return ParseResult{}, fmt.Errorf("invalid_item_reference")
		}
		wanted[itemRef] = struct{}{}
		filteredKeys = append(filteredKeys, key)
	}
	filteredItems := make([]ImportItem, 0, len(selected))
	for _, item := range parsed.Items {
		if _, exists := wanted[item.ItemRef]; exists {
			filteredItems = append(filteredItems, item)
		}
	}
	parsed.Keys = filteredKeys
	parsed.Items = filteredItems
	parsed.Counts = CountItems(filteredItems)
	return parsed, nil
}

func ParseBody(raw string, fingerprintKeys [][]byte) (ParseResult, error) {
	if len(fingerprintKeys) == 0 || len(fingerprintKeys[0]) == 0 {
		return ParseResult{}, fmt.Errorf("profile_fingerprint_key_unavailable")
	}
	if len(raw) > MaxBodyBytes {
		return ParseResult{}, fmt.Errorf("subscription_body_too_large")
	}
	body := strings.TrimSpace(trimUTF8BOM(raw))
	if body == "" {
		return ParseResult{}, fmt.Errorf("subscription body is empty")
	}

	result := ParseResult{
		Warnings: make([]string, 0),
	}
	decoded, base64Encoded, err := maybeDecodeBase64SubscriptionBody(body)
	if err != nil {
		return ParseResult{}, err
	}
	if base64Encoded {
		body = decoded
		result.Warnings = append(result.Warnings, "base64_decoded")
	}

	if json.Valid([]byte(body)) {
		keys, items, err := parseExternalJSONBody(body, 1, fingerprintKeys)
		if err != nil {
			return ParseResult{}, err
		}
		result.DetectedFormat = "xray-json"
		result.Keys = keys
		result.Items = items
		result.Counts = CountItems(items)
		if result.Counts.Rejected > 0 || result.Counts.Unsupported > 0 {
			result.Warnings = append(result.Warnings, "partial_import")
		}
		return result, nil
	}

	keys, items, err := parseExternalLinkBody(body, fingerprintKeys)
	if err != nil {
		return ParseResult{}, err
	}
	result.DetectedFormat = "links"
	result.Keys = keys
	result.Items = items
	result.Counts = CountItems(items)
	if result.Counts.Rejected > 0 || result.Counts.Unsupported > 0 {
		result.Warnings = append(result.Warnings, "partial_import")
	}
	return result, nil
}

func ParseRawBody(sourceURL string, rawBody string, metadata Metadata, fingerprintKeys [][]byte) (ParseResult, error) {
	parsed, err := ParseBody(rawBody, fingerprintKeys)
	if err != nil {
		return ParseResult{}, err
	}
	if len(parsed.Keys) == 0 {
		return ParseResult{}, NoSupportedKeysError(parsed)
	}
	if len(parsed.Items) > MaxImportItems || len(parsed.Keys) > MaxImportItems {
		return ParseResult{}, fmt.Errorf("too many keys in source response (max %d)", MaxImportItems)
	}
	if strings.TrimSpace(metadata.SourceFinalURL) == "" {
		metadata.SourceFinalURL = strings.TrimSpace(sourceURL)
	}
	parsed.Metadata = metadata
	return parsed, nil
}

func ValidateURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("source_url is required")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid source_url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("source_url must use http:// or https://")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return "", fmt.Errorf("source_url must contain host")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("source_url must not contain credentials")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return "", fmt.Errorf("source_url points to a forbidden host")
	}
	if address, err := netip.ParseAddr(host); err == nil && IsForbiddenIP(address) {
		return "", fmt.Errorf("source_url points to a forbidden network")
	}
	return parsed.String(), nil
}

func IsForbiddenIP(address netip.Addr) bool {
	address = address.Unmap()
	return !address.IsValid() ||
		address.IsUnspecified() ||
		address.IsLoopback() ||
		address.IsPrivate() ||
		address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() ||
		address.IsMulticast()
}

func ResolveHost(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if host == "" {
		return nil, fmt.Errorf("empty destination host")
	}
	if address, err := netip.ParseAddr(host); err == nil {
		if IsForbiddenIP(address) {
			return nil, fmt.Errorf("destination resolves to a forbidden network")
		}
		return []netip.Addr{address.Unmap()}, nil
	}

	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve destination host: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("destination host has no addresses")
	}
	for _, address := range addresses {
		if IsForbiddenIP(address) {
			return nil, fmt.Errorf("destination resolves to a forbidden network")
		}
	}
	return addresses, nil
}

func NewHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid destination address: %w", err)
			}
			addresses, err := ResolveHost(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, resolved := range addresses {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			return nil, fmt.Errorf("connect to destination: %w", lastErr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			_, err := ValidateURL(req.URL.String())
			return err
		},
	}
}

func NormalizeCategory(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "Общее"
	}
	if len(value) > 24 {
		value = value[:24]
	}
	return value
}

func clampExternalHWIDField(raw string, max int) string {
	value := strings.TrimSpace(raw)
	if max <= 0 || value == "" {
		return value
	}
	if len(value) > max {
		value = value[:max]
	}
	return value
}

func NormalizeHWIDProfile(pass bool, version, modelName, hwid string) HWIDProfile {
	if !pass {
		return HWIDProfile{}
	}
	return HWIDProfile{
		PassHWID:  true,
		Version:   clampExternalHWIDField(version, 64),
		ModelName: clampExternalHWIDField(modelName, 128),
		HWID:      clampExternalHWIDField(hwid, 128),
	}
}

func applyExternalHWIDHeaders(req *http.Request, profile HWIDProfile) {
	if !profile.PassHWID {
		req.Header.Set("User-Agent", "subshare/1.0 (+external-import)")
		return
	}

	userAgent := "subshare/1.0 (+external-import)"
	if profile.Version != "" {
		if profile.ModelName != "" {
			userAgent = fmt.Sprintf("Happ/%s (%s)", profile.Version, profile.ModelName)
		} else {
			userAgent = fmt.Sprintf("Happ/%s", profile.Version)
		}
	}
	req.Header.Set("User-Agent", userAgent)

	if profile.HWID != "" {
		req.Header.Set("X-HWID", profile.HWID)
		req.Header.Set("X-Device-ID", profile.HWID)
	}
	if profile.ModelName != "" {
		req.Header.Set("X-Device-Model", profile.ModelName)
	}
	if profile.Version != "" {
		req.Header.Set("X-App-Version", profile.Version)
		req.Header.Set("X-Client-Version", profile.Version)
	}
	if profile.ModelName != "" || profile.Version != "" {
		req.Header.Set("X-Device-Info", strings.TrimSpace(fmt.Sprintf("model=%s;version=%s", profile.ModelName, profile.Version)))
	}
}

func SuggestName(sourceURL string, meta Metadata) string {
	if strings.TrimSpace(meta.Title) != "" {
		return strings.TrimSpace(meta.Title)
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "Сторонняя подписка"
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "Сторонняя подписка"
	}
	return host
}

// Fetch downloads and parses a subscription. client may be nil, in which case
// the SSRF-guarded default is used; tests inject their own.
func Fetch(ctx context.Context, client *http.Client, sourceURL string, hwidProfile HWIDProfile, fingerprintKeys [][]byte) (ParseResult, error) {
	finalURL, err := ValidateURL(sourceURL)
	if err != nil {
		return ParseResult{}, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finalURL, nil)
	if err != nil {
		return ParseResult{}, fmt.Errorf("failed to prepare source request")
	}
	applyExternalHWIDHeaders(req, hwidProfile)
	req.Header.Set("Accept", "application/json,text/plain,*/*")

	if client == nil {
		client = NewHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		// net/http errors commonly embed the complete URL. Subscription URLs
		// frequently contain access tokens, so never persist or return them.
		return ParseResult{}, fmt.Errorf("failed to fetch source")
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, MaxBodyBytes+1)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return ParseResult{}, fmt.Errorf("failed to read source response: %w", err)
	}
	if len(bodyBytes) > MaxBodyBytes {
		return ParseResult{}, fmt.Errorf("source response is too large (max 10 MB)")
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return ParseResult{}, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}

	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		if resp.Header.Get("profile-title") != "" || resp.Header.Get("announce") != "" {
			return ParseResult{}, fmt.Errorf("source returned subscription headers but empty body")
		}
		return ParseResult{}, fmt.Errorf("subscription body is empty")
	}

	parsed, err := ParseBody(string(bodyBytes), fingerprintKeys)
	if err != nil {
		return ParseResult{}, err
	}
	if len(parsed.Keys) == 0 {
		return ParseResult{}, NoSupportedKeysError(parsed)
	}
	if len(parsed.Keys) > MaxImportItems {
		return ParseResult{}, fmt.Errorf("too many keys in source response (max %d)", MaxImportItems)
	}

	remoteTitle, titleWarnings := NormalizeRemoteProfileTitle(DecodeHeaderValue(resp.Header.Get("profile-title")))
	parsed.Metadata = Metadata{
		Title:           remoteTitle,
		RefreshHours:    parsePositiveInt(resp.Header.Get("profile-update-interval")),
		SupportURL:      strings.TrimSpace(resp.Header.Get("support-url")),
		WebPageURL:      strings.TrimSpace(resp.Header.Get("profile-web-page-url")),
		Announce:        DecodeHeaderValue(resp.Header.Get("announce")),
		ContentType:     strings.TrimSpace(resp.Header.Get("content-type")),
		ContentDisp:     strings.TrimSpace(resp.Header.Get("content-disposition")),
		SourceFinalURL:  strings.TrimSpace(resp.Request.URL.String()),
		HTTPStatusCode:  resp.StatusCode,
		HTTPStatusLabel: strings.TrimSpace(resp.Status),
	}
	parsed.Warnings = append(parsed.Warnings, titleWarnings...)
	return parsed, nil
}

// SyncTarget is what Sync needs to know about the source being synchronised.
type SyncTarget struct {
	ID            int64
	Enabled       bool
	KeyCategory   string
	KeyInsertMode string
}

// Sync reconciles the parsed feed with the source's stored keys inside the
// given store transaction: repairs historical duplicates, matches by ref,
// fingerprint or unique label, inserts/updates/removes rows and records the
// sync on the source. It never touches SQL or encryption directly.
func Sync(sync *storage.SourceSync, source SyncTarget, parsed ParseResult, fingerprintKeys [][]byte) (SyncResult, error) {
	result := SyncResult{Items: append([]ImportItem(nil), parsed.Items...)}
	result.Skipped = parsed.Counts.Rejected + parsed.Counts.Unsupported + parsed.Counts.Duplicate
	if len(parsed.Keys) == 0 {
		return result, fmt.Errorf("no_keys_to_import")
	}
	state := &syncState{sync: sync, source: source, parsed: parsed, fingerprintKeys: fingerprintKeys, result: result}
	state.itemIndexes = make(map[string]int, len(result.Items))
	for index := range result.Items {
		state.itemIndexes[result.Items[index].ItemRef] = index
	}

	state.statusValue = model.KeyStatusActive
	if !source.Enabled {
		state.statusValue = model.KeyStatusNonActive
	}
	state.targetCategory = keymanagement.NormalizeKeyCategory(source.KeyCategory)
	targetCategoryID, err := sync.EnsureCategory(state.targetCategory)
	if err != nil {
		return state.result, err
	}
	state.targetCategoryID = targetCategoryID
	insertMode := NormalizeKeyInsertMode(source.KeyInsertMode)

	state.existingKeys, err = sync.ListSourceKeys(source.ID)
	if err != nil {
		return state.result, err
	}
	if len(fingerprintKeys) == 0 {
		return state.result, fmt.Errorf("profile_fingerprint_key_unavailable")
	}
	if err := state.repairDuplicateGroups(); err != nil {
		return state.result, err
	}
	state.indexExistingKeys()
	state.countIncomingLabels()

	state.seenRefs = make(map[string]struct{}, len(parsed.Keys))
	state.nextSortOrder, err = sync.NextSortOrder(targetCategoryID, source.ID, insertMode == "top", len(parsed.Keys))
	if err != nil {
		return state.result, err
	}
	for index, item := range parsed.Keys {
		if err := state.upsertParsedKey(index, item); err != nil {
			return state.result, err
		}
	}
	if err := state.removeUnmatched(); err != nil {
		return state.result, err
	}
	removed := state.result.Counts.Removed
	state.result.Counts = CountItems(state.result.Items)
	state.result.Counts.Removed = removed

	if err := sync.MarkSourceSynced(source.ID, state.result.Imported, storage.SourceSyncMetadata{
		Title: parsed.Metadata.Title, RefreshHours: parsed.Metadata.RefreshHours, SupportURL: parsed.Metadata.SupportURL,
		WebPageURL: parsed.Metadata.WebPageURL, Announce: parsed.Metadata.Announce,
	}); err != nil {
		return state.result, err
	}

	return state.result, nil
}

// syncState carries the maps and counters shared by the Sync phases.
type syncState struct {
	sync            *storage.SourceSync
	source          SyncTarget
	parsed          ParseResult
	fingerprintKeys [][]byte
	result          SyncResult
	itemIndexes     map[string]int

	statusValue      string
	targetCategory   string
	targetCategoryID any

	existingKeys          []storage.SourceKey
	removedDuplicateIDs   map[int64]struct{}
	existingByRef         map[string]storage.SourceKey
	existingByFingerprint map[string]storage.SourceKey
	existingByLabel       map[string]storage.SourceKey
	existingLabelCounts   map[string]int
	existingIDs           map[int64]struct{}
	incomingLabelCounts   map[string]int

	seenRefs      map[string]struct{}
	nextSortOrder int64
}

func (s *syncState) setItemStatus(itemRef, status string) {
	if index, exists := s.itemIndexes[itemRef]; exists {
		s.result.Items[index].Status = status
	}
}

// syncLabel is the stored label for an incoming key: trimmed, defaulted by
// position and truncated to the column width.
func syncLabel(item ParsedKey, index int) string {
	label := strings.TrimSpace(item.Label)
	if label == "" {
		label = fmt.Sprintf("Импорт %03d", index+1)
	}
	if len(label) > 255 {
		label = label[:255]
	}
	return label
}

// repairDuplicateGroups merges historical duplicate rows. Historical
// synchronizers could create multiple source-owned rows for the same URI
// profile. Repair those groups before normal matching so assigning the
// canonical fingerprint cannot temporarily conflict with a duplicate. A row
// already holding the current fingerprint wins; otherwise the lowest stable ID
// wins because existingKeys is ordered by ID. Raw XRAY-JSON does not parse as
// a URI profile and deliberately retains exact-raw identity.
func (s *syncState) repairDuplicateGroups() error {
	semanticGroups := make(map[string][]int)
	for index := range s.existingKeys {
		if s.existingKeys[index].URL == "" {
			continue
		}
		profile, parseErr := profiles.Parse(s.existingKeys[index].URL)
		if parseErr != nil {
			continue
		}
		fingerprint, fingerprintErr := profiles.Fingerprint(profile, s.fingerprintKeys[0])
		if fingerprintErr != nil {
			continue
		}
		semanticGroups[fingerprint] = append(semanticGroups[fingerprint], index)
	}
	s.removedDuplicateIDs = make(map[int64]struct{})
	for fingerprint, indexes := range semanticGroups {
		if len(indexes) < 2 {
			continue
		}
		if err := s.mergeDuplicateGroup(fingerprint, indexes); err != nil {
			return err
		}
	}
	return nil
}

func (s *syncState) mergeDuplicateGroup(fingerprint string, indexes []int) error {
	survivorIndex := indexes[0]
	for _, index := range indexes {
		if s.existingKeys[index].Fingerprint == fingerprint {
			survivorIndex = index
			break
		}
	}
	survivorID := s.existingKeys[survivorIndex].ID
	if err := s.inheritClientDisplayName(survivorIndex, indexes); err != nil {
		return err
	}
	for _, index := range indexes {
		duplicateID := s.existingKeys[index].ID
		if duplicateID == survivorID {
			continue
		}
		if err := s.sync.MergeUserAssignments(survivorID, duplicateID); err != nil {
			return err
		}
		if err := s.sync.DeleteSourceKey(s.source.ID, duplicateID); err != nil {
			return err
		}
		s.removedDuplicateIDs[duplicateID] = struct{}{}
		s.result.Counts.Removed++
	}
	if s.existingKeys[survivorIndex].Fingerprint != fingerprint {
		if err := s.sync.SetFingerprint(s.source.ID, survivorID, fingerprint); err != nil {
			return err
		}
		s.existingKeys[survivorIndex].Fingerprint = fingerprint
	}
	return nil
}

// inheritClientDisplayName: client_display_name is local administrator
// metadata, not source data. Keep the survivor's override when present;
// otherwise inherit the first non-empty override in stable-ID order before
// duplicates are deleted.
func (s *syncState) inheritClientDisplayName(survivorIndex int, indexes []int) error {
	if strings.TrimSpace(s.existingKeys[survivorIndex].ClientDisplayName) != "" {
		return nil
	}
	for _, index := range indexes {
		override := strings.TrimSpace(s.existingKeys[index].ClientDisplayName)
		if override == "" {
			continue
		}
		if err := s.sync.SetClientDisplayName(s.source.ID, s.existingKeys[survivorIndex].ID, override); err != nil {
			return err
		}
		s.existingKeys[survivorIndex].ClientDisplayName = override
		return nil
	}
	return nil
}

func (s *syncState) indexExistingKeys() {
	s.existingByRef = make(map[string]storage.SourceKey)
	s.existingByFingerprint = make(map[string]storage.SourceKey)
	s.existingByLabel = make(map[string]storage.SourceKey)
	s.existingLabelCounts = make(map[string]int)
	s.existingIDs = make(map[int64]struct{})
	for _, key := range s.existingKeys {
		if _, removed := s.removedDuplicateIDs[key.ID]; removed {
			continue
		}
		if key.Ref != "" {
			s.existingByRef[key.Ref] = key
		}
		if key.Fingerprint != "" {
			if _, exists := s.existingByFingerprint[key.Fingerprint]; !exists {
				s.existingByFingerprint[key.Fingerprint] = key
			}
		} else if key.URL != "" {
			s.indexLegacyFingerprints(key)
		}
		label := strings.TrimSpace(key.Label)
		if label != "" {
			s.existingLabelCounts[label]++
			s.existingByLabel[label] = key
		}
		s.existingIDs[key.ID] = struct{}{}
	}
}

// indexLegacyFingerprints: rows imported before semantic fingerprints existed
// still need to preserve their ID on the first post-upgrade
// rename/re-encoding. Compute only in memory from the decrypted profile; raw
// XRAY-JSON intentionally remains on its exact-raw reference identity.
func (s *syncState) indexLegacyFingerprints(key storage.SourceKey) {
	profile, parseErr := profiles.Parse(key.URL)
	if parseErr != nil {
		return
	}
	for _, fingerprintKey := range s.fingerprintKeys {
		fingerprint, fingerprintErr := profiles.Fingerprint(profile, fingerprintKey)
		if fingerprintErr != nil {
			continue
		}
		if _, exists := s.existingByFingerprint[fingerprint]; !exists {
			s.existingByFingerprint[fingerprint] = key
		}
	}
}

func (s *syncState) countIncomingLabels() {
	s.incomingLabelCounts = make(map[string]int)
	for index, item := range s.parsed.Keys {
		s.incomingLabelCounts[syncLabel(item, index)]++
	}
}

func (s *syncState) upsertParsedKey(index int, item ParsedKey) error {
	ref := strings.TrimSpace(item.Ref)
	if ref == "" {
		ref = KeyRef(item.URL)
	}
	if _, exists := s.seenRefs[ref]; exists {
		s.result.Skipped++
		s.setItemStatus(item.ItemRef, StatusDuplicate)
		return nil
	}
	s.seenRefs[ref] = struct{}{}

	label := syncLabel(item, index)
	urlValue := item.URL
	if strings.TrimSpace(urlValue) == "" || len(urlValue) > 65535 {
		s.result.Skipped++
		s.setItemStatus(item.ItemRef, StatusRejected)
		return nil
	}
	warningPayload, err := json.Marshal(item.WarningCodes)
	if err != nil {
		return err
	}
	write := storage.SourceKeyWrite{
		SourceID: s.source.ID, Ref: ref, Label: label, CategoryID: s.targetCategoryID, Category: s.targetCategory,
		SortOrder: s.nextSortOrder, Protocol: item.Protocol, Fingerprint: item.Fingerprint,
		ProfileSchemaVersion: item.ProfileSchemaVersion, Compatibility: item.Compatibility, WarningsJSON: string(warningPayload), URL: urlValue,
	}
	if existing, matched := s.matchExisting(item, ref, label); matched {
		return s.writeMatched(existing, item, write)
	}
	return s.insertNew(item, write)
}

// matchExisting finds the stored row for an incoming key by ref, then by any
// fingerprint candidate, then by unique label. A provider may rotate
// connection material while retaining the logical profile name. When both
// sides have exactly one such label, preserve the row (and its local
// metadata). Ambiguous labels deliberately remain on the conservative
// remove/add path.
func (s *syncState) matchExisting(item ParsedKey, ref, label string) (storage.SourceKey, bool) {
	if existing, matched := s.existingByRef[ref]; matched {
		return existing, true
	}
	for _, candidate := range item.FingerprintCandidates {
		if existing, exists := s.existingByFingerprint[candidate]; exists {
			return existing, true
		}
	}
	if s.incomingLabelCounts[label] == 1 && s.existingLabelCounts[label] == 1 {
		candidate := s.existingByLabel[label]
		if _, available := s.existingIDs[candidate.ID]; available {
			return candidate, true
		}
	}
	return storage.SourceKey{}, false
}

func (s *syncState) writeMatched(existing storage.SourceKey, item ParsedKey, write storage.SourceKeyWrite) error {
	if existing.Ref != "" {
		write.Ref = existing.Ref
	}
	changed := existing.Label != write.Label || existing.URL != write.URL || existing.Protocol != item.Protocol ||
		existing.Fingerprint != item.Fingerprint || existing.ProfileSchemaVersion != item.ProfileSchemaVersion ||
		existing.Compatibility != item.Compatibility || existing.WarningsJSON != write.WarningsJSON
	if err := s.sync.UpdateSourceKey(existing.ID, write); err != nil {
		return err
	}
	delete(s.existingIDs, existing.ID)
	s.nextSortOrder++
	s.result.Imported++
	if changed {
		s.setItemStatus(item.ItemRef, StatusUpdated)
	} else {
		s.setItemStatus(item.ItemRef, StatusUnchanged)
	}
	return nil
}

func (s *syncState) insertNew(item ParsedKey, write storage.SourceKeyWrite) error {
	write.Status = s.statusValue
	_, err := s.sync.InsertSourceKey(write)
	if errors.Is(err, storage.ErrDuplicateSourceKey) {
		s.result.Skipped++
		s.setItemStatus(item.ItemRef, StatusDuplicate)
		return nil
	}
	if err != nil {
		return err
	}
	s.setItemStatus(item.ItemRef, StatusAdded)
	s.nextSortOrder++
	s.result.Imported++
	return nil
}

// removeUnmatched deletes stored rows the feed no longer lists. Any
// rejected/unsupported source item makes absence ambiguous. Preserve unmatched
// rows until a fully parsed refresh confirms they are missing.
func (s *syncState) removeUnmatched() error {
	if s.parsed.Counts.Rejected != 0 || s.parsed.Counts.Unsupported != 0 {
		return nil
	}
	for keyID := range s.existingIDs {
		if err := s.sync.DeleteSourceKey(s.source.ID, keyID); err != nil {
			return err
		}
		s.result.Counts.Removed++
	}
	return nil
}

const maxExternalSourceNameCodePoints = 64

// ValidateName normalizes and validates names supplied through
// either source API. Database writes must use the returned value.
func ValidateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("name must be valid UTF-8")
	}
	if utf8.RuneCountInString(name) > maxExternalSourceNameCodePoints {
		return "", fmt.Errorf("name is too long (max %d characters)", maxExternalSourceNameCodePoints)
	}
	return name, nil
}

// NormalizeRemoteName is intentionally separate from user-input
// validation: remote metadata can be shortened, while user input is rejected.
func NormalizeRemoteName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) <= maxExternalSourceNameCodePoints {
		return name, false
	}
	runes := []rune(name)
	return string(runes[:maxExternalSourceNameCodePoints-1]) + "…", true
}

func NormalizeRemoteProfileTitle(raw string) (string, []string) {
	name, shortened := NormalizeRemoteName(raw)
	if !shortened {
		return name, nil
	}
	return name, []string{"Remote profile title exceeded 64 characters and was shortened."}
}
