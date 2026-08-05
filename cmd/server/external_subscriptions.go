package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

const (
	maxExternalSubscriptionBodyBytes = 10 << 20 // 10 MB
	maxExternalImportItems           = 10000
)

func (a *App) externalProfileFingerprintKeys() [][]byte {
	if a == nil || len(a.profileFingerprintKey) == 0 {
		return nil
	}
	keys := make([][]byte, 0, 1+len(a.profileFingerprintOldKeys))
	keys = append(keys, append([]byte(nil), a.profileFingerprintKey...))
	keys = append(keys, cloneByteSlices(a.profileFingerprintOldKeys)...)
	return keys
}

type externalSubscriptionMetadata struct {
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

type externalParsedKey struct {
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

func (key externalParsedKey) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "external_profile{protocol="+key.Protocol+", item_ref="+key.ItemRef+", raw=[redacted]}")
}

func (key externalParsedKey) String() string {
	return "external_profile{protocol=" + key.Protocol + ", item_ref=" + key.ItemRef + ", raw=[redacted]}"
}

func (key externalParsedKey) GoString() string { return key.String() }

func (key externalParsedKey) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("external_profile_internal_only")
}

type externalSubscriptionParseResult struct {
	DetectedFormat string
	Metadata       externalSubscriptionMetadata
	Keys           []externalParsedKey
	Items          []externalSafeImportItem
	Counts         externalImportCounts
	Warnings       []string
}

func (result externalSubscriptionParseResult) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, fmt.Sprintf("external_subscription{format=%s, items=%d, raw=[redacted]}", result.DetectedFormat, len(result.Items)))
}

func (result externalSubscriptionParseResult) String() string {
	return fmt.Sprintf("external_subscription{format=%s, items=%d, raw=[redacted]}", result.DetectedFormat, len(result.Items))
}

func (result externalSubscriptionParseResult) GoString() string { return result.String() }

type externalSafeImportItem struct {
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

type externalImportCounts struct {
	Accepted          int `json:"accepted"`
	Rejected          int `json:"rejected"`
	Duplicate         int `json:"duplicate"`
	Updated           int `json:"updated"`
	Unchanged         int `json:"unchanged"`
	CompatibilityOnly int `json:"compatibility_only"`
	Ambiguous         int `json:"ambiguous"`
	Unsupported       int `json:"unsupported"`
}

type externalSyncResult struct {
	Imported int
	Skipped  int
	Counts   externalImportCounts
	Items    []externalSafeImportItem
}

const externalProfileSchemaVersion = 1

const (
	externalStatusAccepted          = model.ExternalImportStatusAccepted
	externalStatusRejected          = model.ExternalImportStatusRejected
	externalStatusDuplicate         = model.ExternalImportStatusDuplicate
	externalStatusUpdated           = model.ExternalImportStatusUpdated
	externalStatusUnchanged         = model.ExternalImportStatusUnchanged
	externalStatusCompatibilityOnly = model.ExternalImportStatusCompatibilityOnly
	externalStatusAmbiguous         = model.ExternalImportStatusAmbiguous
	externalStatusUnsupported       = model.ExternalImportStatusUnsupported
)

type externalSourceRow struct {
	ID                  int64
	Name                string
	Category            string
	KeyCategory         string
	KeyInsertMode       string
	SourceURL           string
	Enabled             bool
	ApplyRemoteMetadata bool
	PassHWID            bool
	HWIDVersion         string
	HWIDModelName       string
	HWIDValue           string
	LastImportCount     int
	ImportStatus        string
	LastError           string
	LastSyncedAt        sql.NullTime
	MetaTitle           string
	MetaRefreshHours    int
	MetaSupportURL      string
	MetaWebPageURL      string
	MetaAnnounce        string
	CreatedAt           sql.NullTime
	UpdatedAt           sql.NullTime
}

type externalHWIDProfile struct {
	PassHWID  bool
	Version   string
	ModelName string
	HWID      string
}

func normalizeKeyInsertMode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "top", "bottom":
		return value
	default:
		return "bottom"
	}
}

func nonNilWarnings(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func decodeSubscriptionHeaderValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), "base64:") {
		decoded := decodeBase64String(value)
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
	if !base64Like || compact.Len() < 24 {
		if explicit {
			return "", true, fmt.Errorf("invalid_base64_subscription")
		}
		return "", false, nil
	}
	encoded := compact.String()
	if strings.ContainsAny(encoded, "+/") && strings.ContainsAny(encoded, "-_") {
		return "", true, fmt.Errorf("invalid_base64_subscription")
	}

	var encoding *base64.Encoding
	urlSafe := strings.ContainsAny(encoded, "-_")
	padded := strings.Contains(encoded, "=")
	switch {
	case urlSafe && padded:
		encoding = base64.URLEncoding.Strict()
	case urlSafe:
		encoding = base64.RawURLEncoding.Strict()
	case padded:
		encoding = base64.StdEncoding.Strict()
	default:
		encoding = base64.RawStdEncoding.Strict()
	}
	if encoding.DecodedLen(len(encoded)) > maxExternalSubscriptionBodyBytes {
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

func buildExternalKeyRef(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:16])
}

func extractJSONSubscriptionLabel(root map[string]any, fallback string) string {
	if value, ok := root["remarks"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if value, ok := root["tag"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if meta, ok := root["meta"].(map[string]any); ok {
		if value, ok := meta["serverDescription"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if outbounds, ok := root["outbounds"].([]any); ok && len(outbounds) > 0 {
		if outbound, ok := outbounds[0].(map[string]any); ok {
			if value, ok := outbound["tag"].(string); ok && strings.TrimSpace(value) != "" {
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

func safeExternalItem(key externalParsedKey, status, errorCode string) externalSafeImportItem {
	warnings := append([]string(nil), key.WarningCodes...)
	if warnings == nil {
		warnings = []string{}
	}
	return externalSafeImportItem{
		ItemRef: key.ItemRef, LineIndex: key.LineIndex, Protocol: key.Protocol, Scheme: key.Scheme,
		DisplayName: key.Label, Host: key.Host, Port: key.Port, Compatibility: key.Compatibility,
		Label:  key.Label,
		Status: status, Warnings: warnings, ErrorCode: errorCode,
		URLShort: safeEndpointSummary(key.Protocol, key.Host, key.Port),
	}
}

func rejectedExternalItem(raw string, lineIndex int, scheme, status, errorCode string, fingerprintKeys [][]byte) (externalSafeImportItem, error) {
	itemRef, err := buildExternalItemRef(raw, lineIndex, fingerprintKeys)
	if err != nil {
		return externalSafeImportItem{}, err
	}
	protocol := scheme
	if protocol == "" {
		protocol = "unknown"
	}
	return externalSafeImportItem{
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

func containsWarningCode(codes []string, wanted ...string) bool {
	for _, code := range codes {
		for _, candidate := range wanted {
			if code == candidate {
				return true
			}
		}
	}
	return false
}

func parseExternalProfileItem(raw string, lineIndex int, scheme string, fingerprintKeys [][]byte) (*externalParsedKey, externalSafeImportItem, error) {
	itemRef, err := buildExternalItemRef(raw, lineIndex, fingerprintKeys)
	if err != nil {
		return nil, externalSafeImportItem{}, err
	}
	profile, err := profiles.Parse(raw)
	if err != nil {
		code := string(profiles.ErrorCodeOf(err))
		if code == "" {
			code = "invalid_profile"
		}
		item, itemErr := rejectedExternalItem(raw, lineIndex, scheme, externalStatusRejected, code, fingerprintKeys)
		return nil, item, itemErr
	}
	original, err := profiles.Serialize(profile, profiles.OriginalSerialization)
	if err != nil || !original.Exact {
		item, itemErr := rejectedExternalItem(raw, lineIndex, scheme, externalStatusRejected, "original_serialization_unavailable", fingerprintKeys)
		return nil, item, itemErr
	}
	metadata := profile.SafeMetadata()
	warnings := profileWarningCodes(profile)
	compatibility := string(metadata.Capabilities.Status)
	status := externalStatusAccepted
	if metadata.Capabilities.Status == profiles.CapabilityReadOnly {
		status = externalStatusCompatibilityOnly
	} else if containsWarningCode(warnings, profiles.WarningAmbiguousParameter, profiles.WarningConflictingPreference) {
		status = externalStatusAmbiguous
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
				item, itemErr := rejectedExternalItem(raw, lineIndex, scheme, externalStatusRejected, code, fingerprintKeys)
				return nil, item, itemErr
			}
			fingerprints = append(fingerprints, fingerprint)
		}
	}
	label := strings.TrimSpace(metadata.DisplayName)
	if label == "" {
		label = fmt.Sprintf("Import %03d", lineIndex)
	}
	key := &externalParsedKey{
		Label: label, URL: original.URI.Reveal(), Scheme: scheme, Protocol: string(metadata.Protocol),
		Host: metadata.Server, Port: metadata.Port, Ref: buildExternalKeyRef(original.URI.Reveal()),
		ItemRef: itemRef, LineIndex: lineIndex, Compatibility: compatibility,
		ProfileSchemaVersion: externalProfileSchemaVersion, FingerprintCandidates: fingerprints,
		WarningCodes: warnings, InitialStatus: status,
	}
	if len(fingerprints) > 0 {
		key.Fingerprint = fingerprints[0]
	}
	item := safeExternalItem(*key, status, "")
	return key, item, nil
}

func externalDedupeIdentity(key externalParsedKey) string {
	if key.Fingerprint != "" {
		return "profile:" + key.Fingerprint
	}
	return "raw:" + key.Ref
}

func appendExternalParsedItem(keys *[]externalParsedKey, items *[]externalSafeImportItem, seen map[string]struct{}, key externalParsedKey) {
	identity := externalDedupeIdentity(key)
	if _, exists := seen[identity]; exists {
		*items = append(*items, safeExternalItem(key, externalStatusDuplicate, ""))
		return
	}
	seen[identity] = struct{}{}
	*keys = append(*keys, key)
	*items = append(*items, safeExternalItem(key, key.InitialStatus, ""))
}

func parseExternalLinkBody(body string, fingerprintKeys [][]byte) ([]externalParsedKey, []externalSafeImportItem, error) {
	if strings.Count(body, "\n")+1 > maxExternalImportItems {
		return nil, nil, fmt.Errorf("too_many_source_items")
	}
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	keys := make([]externalParsedKey, 0, len(lines))
	items := make([]externalSafeImportItem, 0, len(lines))
	seen := make(map[string]struct{})
	for index, line := range lines {
		raw := strings.TrimSpace(line)
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, ";") {
			continue
		}
		if strings.HasPrefix(raw, "{") && json.Valid([]byte(raw)) {
			jsonKeys, jsonItems, err := parseExternalJSONBody(raw, index+1, fingerprintKeys)
			if err != nil {
				return nil, nil, err
			}
			for _, item := range jsonItems {
				if item.Status == externalStatusRejected {
					items = append(items, item)
				}
			}
			for _, key := range jsonKeys {
				appendExternalParsedItem(&keys, &items, seen, key)
			}
			continue
		}
		scheme := externalURIScheme(raw)
		switch scheme {
		case "vless", "vmess", "trojan":
			draft, err := parseLinkConfiguration(raw)
			if err != nil {
				item, refErr := rejectedExternalItem(raw, index+1, scheme, externalStatusRejected, "invalid_"+scheme, fingerprintKeys)
				if refErr != nil {
					return nil, nil, refErr
				}
				items = append(items, item)
				continue
			}
			itemRef, err := buildExternalItemRef(raw, index+1, fingerprintKeys)
			if err != nil {
				return nil, nil, err
			}
			label := strings.TrimSpace(draft.Remark)
			if label == "" {
				label = fmt.Sprintf("Import %03d", index+1)
			}
			key := externalParsedKey{
				Label: label, URL: raw, Scheme: scheme, Protocol: scheme, Host: draft.Server,
				Port: strconv.Itoa(draft.Port), Ref: buildExternalKeyRef(raw), ItemRef: itemRef,
				LineIndex: index + 1, Compatibility: "legacy", InitialStatus: externalStatusAccepted,
			}
			appendExternalParsedItem(&keys, &items, seen, key)
		case "ss", "hysteria2", "hy2", "tuic":
			key, item, err := parseExternalProfileItem(raw, index+1, scheme, fingerprintKeys)
			if err != nil {
				return nil, nil, err
			}
			if key == nil {
				items = append(items, item)
				continue
			}
			appendExternalParsedItem(&keys, &items, seen, *key)
		case "hysteria", "hysteria2+realm", "realm", "realm+http":
			item, err := rejectedExternalItem(raw, index+1, scheme, externalStatusUnsupported, "unsupported_protocol", fingerprintKeys)
			if err != nil {
				return nil, nil, err
			}
			items = append(items, item)
		default:
			item, err := rejectedExternalItem(raw, index+1, scheme, externalStatusUnsupported, "unsupported_scheme", fingerprintKeys)
			if err != nil {
				return nil, nil, err
			}
			items = append(items, item)
		}
	}
	return keys, items, nil
}

func isSIP008ProfileObject(object map[string]any) bool {
	_, passwordPresent := object["password"]
	return anyToString(object["server"]) != "" &&
		anyToString(object["server_port"]) != "" &&
		anyToString(object["method"]) != "" &&
		passwordPresent
}

func parseExternalJSONBody(body string, firstItemIndex int, fingerprintKeys [][]byte) ([]externalParsedKey, []externalSafeImportItem, error) {
	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, nil, fmt.Errorf("invalid_json_subscription")
	}
	keys := make([]externalParsedKey, 0, 16)
	items := make([]externalSafeImportItem, 0, 16)
	seen := make(map[string]struct{})
	appendObject := func(obj map[string]any, lineIndex int) error {
		encoded, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("invalid_json_subscription")
		}
		raw := string(encoded)
		itemRef, err := buildExternalItemRef(raw, lineIndex, fingerprintKeys)
		if err != nil {
			return err
		}
		if isSIP008ProfileObject(obj) {
			items = append(items, externalSafeImportItem{
				ItemRef: itemRef, LineIndex: lineIndex, Protocol: "shadowsocks", Scheme: "sip008",
				Compatibility: "unsupported", Status: externalStatusUnsupported, Warnings: []string{},
				ErrorCode: "unsupported_sip008_format", URLShort: "shadowsocks",
			})
			return nil
		}
		drafts, parseErr := parseXrayJSONDrafts(raw)
		if parseErr != nil || len(drafts) == 0 {
			items = append(items, externalSafeImportItem{ItemRef: itemRef, LineIndex: lineIndex, Protocol: "xray-json", Scheme: "xray-json", Compatibility: "legacy", Status: externalStatusRejected, Warnings: []string{}, ErrorCode: "invalid_xray_json", URLShort: "xray-json"})
			return nil
		}
		label := strings.TrimSpace(extractJSONSubscriptionLabel(obj, fmt.Sprintf("JSON %03d", lineIndex)))
		if label == "" {
			label = fmt.Sprintf("JSON %03d", lineIndex)
		}
		host, port, _ := parseXrayJSONTarget(raw)
		key := externalParsedKey{Label: label, URL: raw, Scheme: "xray-json", Protocol: "xray-json", Host: host, Port: port, Ref: buildExternalKeyRef(raw), ItemRef: itemRef, LineIndex: lineIndex, Compatibility: "legacy", InitialStatus: externalStatusAccepted}
		appendExternalParsedItem(&keys, &items, seen, key)
		return nil
	}
	switch typed := parsed.(type) {
	case map[string]any:
		if err := appendObject(typed, firstItemIndex); err != nil {
			return nil, nil, err
		}
	case []any:
		if len(typed) > maxExternalImportItems {
			return nil, nil, fmt.Errorf("too_many_source_items")
		}
		for index, value := range typed {
			lineIndex := firstItemIndex + index
			object, ok := value.(map[string]any)
			if !ok {
				itemRef, err := buildExternalItemRef("invalid-json-item", lineIndex, fingerprintKeys)
				if err != nil {
					return nil, nil, err
				}
				items = append(items, externalSafeImportItem{ItemRef: itemRef, LineIndex: lineIndex, Protocol: "xray-json", Scheme: "xray-json", Compatibility: "legacy", Status: externalStatusRejected, Warnings: []string{}, ErrorCode: "invalid_xray_json", URLShort: "xray-json"})
				continue
			}
			if err := appendObject(object, lineIndex); err != nil {
				return nil, nil, err
			}
		}
	default:
		return nil, nil, fmt.Errorf("invalid_json_subscription")
	}
	return keys, items, nil
}

func countExternalItems(items []externalSafeImportItem) externalImportCounts {
	var counts externalImportCounts
	for _, item := range items {
		if item.Compatibility == string(profiles.CapabilityReadOnly) {
			counts.CompatibilityOnly++
		}
		if containsWarningCode(item.Warnings, profiles.WarningAmbiguousParameter, profiles.WarningConflictingPreference) {
			counts.Ambiguous++
		}
		switch item.Status {
		case externalStatusAccepted:
			counts.Accepted++
		case externalStatusRejected:
			counts.Rejected++
		case externalStatusDuplicate:
			counts.Duplicate++
		case externalStatusUpdated:
			counts.Updated++
		case externalStatusUnchanged:
			counts.Unchanged++
		case externalStatusCompatibilityOnly:
			if item.Compatibility != string(profiles.CapabilityReadOnly) {
				counts.CompatibilityOnly++
			}
		case externalStatusAmbiguous:
			if !containsWarningCode(item.Warnings, profiles.WarningAmbiguousParameter, profiles.WarningConflictingPreference) {
				counts.Ambiguous++
			}
		case externalStatusUnsupported:
			counts.Unsupported++
		}
	}
	return counts
}

func noSupportedExternalKeysError(parsed externalSubscriptionParseResult) error {
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

func filterExternalSelection(parsed externalSubscriptionParseResult, selected []string) (externalSubscriptionParseResult, error) {
	if len(selected) == 0 {
		return parsed, nil
	}
	byRef := make(map[string]externalParsedKey, len(parsed.Keys))
	for _, key := range parsed.Keys {
		byRef[key.ItemRef] = key
	}
	wanted := make(map[string]struct{}, len(selected))
	filteredKeys := make([]externalParsedKey, 0, len(selected))
	for _, itemRef := range selected {
		itemRef = strings.TrimSpace(itemRef)
		if itemRef == "" {
			return externalSubscriptionParseResult{}, fmt.Errorf("invalid_item_reference")
		}
		if _, duplicate := wanted[itemRef]; duplicate {
			return externalSubscriptionParseResult{}, fmt.Errorf("duplicate_item_reference")
		}
		key, exists := byRef[itemRef]
		if !exists {
			return externalSubscriptionParseResult{}, fmt.Errorf("invalid_item_reference")
		}
		wanted[itemRef] = struct{}{}
		filteredKeys = append(filteredKeys, key)
	}
	filteredItems := make([]externalSafeImportItem, 0, len(selected))
	for _, item := range parsed.Items {
		if _, exists := wanted[item.ItemRef]; exists {
			filteredItems = append(filteredItems, item)
		}
	}
	parsed.Keys = filteredKeys
	parsed.Items = filteredItems
	parsed.Counts = countExternalItems(filteredItems)
	return parsed, nil
}

func parseExternalSubscriptionBody(raw string, fingerprintKeys [][]byte) (externalSubscriptionParseResult, error) {
	if len(fingerprintKeys) == 0 || len(fingerprintKeys[0]) == 0 {
		return externalSubscriptionParseResult{}, fmt.Errorf("profile_fingerprint_key_unavailable")
	}
	if len(raw) > maxExternalSubscriptionBodyBytes {
		return externalSubscriptionParseResult{}, fmt.Errorf("subscription_body_too_large")
	}
	body := strings.TrimSpace(trimUTF8BOM(raw))
	if body == "" {
		return externalSubscriptionParseResult{}, fmt.Errorf("subscription body is empty")
	}

	result := externalSubscriptionParseResult{
		Warnings: make([]string, 0),
	}
	decoded, base64Encoded, err := maybeDecodeBase64SubscriptionBody(body)
	if err != nil {
		return externalSubscriptionParseResult{}, err
	}
	if base64Encoded {
		body = decoded
		result.Warnings = append(result.Warnings, "base64_decoded")
	}

	if json.Valid([]byte(body)) {
		keys, items, err := parseExternalJSONBody(body, 1, fingerprintKeys)
		if err != nil {
			return externalSubscriptionParseResult{}, err
		}
		result.DetectedFormat = "xray-json"
		result.Keys = keys
		result.Items = items
		result.Counts = countExternalItems(items)
		if result.Counts.Rejected > 0 || result.Counts.Unsupported > 0 {
			result.Warnings = append(result.Warnings, "partial_import")
		}
		return result, nil
	}

	keys, items, err := parseExternalLinkBody(body, fingerprintKeys)
	if err != nil {
		return externalSubscriptionParseResult{}, err
	}
	result.DetectedFormat = "links"
	result.Keys = keys
	result.Items = items
	result.Counts = countExternalItems(items)
	if result.Counts.Rejected > 0 || result.Counts.Unsupported > 0 {
		result.Warnings = append(result.Warnings, "partial_import")
	}
	return result, nil
}

func parseExternalSubscriptionFromRawBody(sourceURL string, rawBody string, metadata externalSubscriptionMetadata, fingerprintKeys [][]byte) (externalSubscriptionParseResult, error) {
	parsed, err := parseExternalSubscriptionBody(rawBody, fingerprintKeys)
	if err != nil {
		return externalSubscriptionParseResult{}, err
	}
	if len(parsed.Keys) == 0 {
		return externalSubscriptionParseResult{}, noSupportedExternalKeysError(parsed)
	}
	if len(parsed.Items) > maxExternalImportItems || len(parsed.Keys) > maxExternalImportItems {
		return externalSubscriptionParseResult{}, fmt.Errorf("too many keys in source response (max %d)", maxExternalImportItems)
	}
	if strings.TrimSpace(metadata.SourceFinalURL) == "" {
		metadata.SourceFinalURL = strings.TrimSpace(sourceURL)
	}
	parsed.Metadata = metadata
	return parsed, nil
}

func validateExternalSourceURL(raw string) (string, error) {
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
	if address, err := netip.ParseAddr(host); err == nil && isForbiddenExternalIP(address) {
		return "", fmt.Errorf("source_url points to a forbidden network")
	}
	return parsed.String(), nil
}

func isForbiddenExternalIP(address netip.Addr) bool {
	address = address.Unmap()
	return !address.IsValid() ||
		address.IsUnspecified() ||
		address.IsLoopback() ||
		address.IsPrivate() ||
		address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() ||
		address.IsMulticast()
}

func resolveExternalHost(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if host == "" {
		return nil, fmt.Errorf("empty destination host")
	}
	if address, err := netip.ParseAddr(host); err == nil {
		if isForbiddenExternalIP(address) {
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
		if isForbiddenExternalIP(address) {
			return nil, fmt.Errorf("destination resolves to a forbidden network")
		}
	}
	return addresses, nil
}

func newExternalSubscriptionHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid destination address: %w", err)
			}
			addresses, err := resolveExternalHost(ctx, host)
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
			_, err := validateExternalSourceURL(req.URL.String())
			return err
		},
	}
}

func normalizeExternalSourceCategory(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "Общее"
	}
	if len(value) > 24 {
		value = value[:24]
	}
	return value
}

func (a *App) upsertExternalSourceCategory(category string) error {
	category = normalizeExternalSourceCategory(category)
	_, err := a.db.Exec(
		`INSERT INTO external_source_categories(name, updated_at)
		 VALUES(?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		category,
	)
	return err
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

func normalizeExternalHWIDProfile(pass bool, version, modelName, hwid string) externalHWIDProfile {
	if !pass {
		return externalHWIDProfile{}
	}
	return externalHWIDProfile{
		PassHWID:  true,
		Version:   clampExternalHWIDField(version, 64),
		ModelName: clampExternalHWIDField(modelName, 128),
		HWID:      clampExternalHWIDField(hwid, 128),
	}
}

func applyExternalHWIDHeaders(req *http.Request, profile externalHWIDProfile) {
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

func suggestExternalSourceName(sourceURL string, meta externalSubscriptionMetadata) string {
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

func fetchExternalSubscription(sourceURL string, hwidProfile externalHWIDProfile, fingerprintKeys [][]byte) (externalSubscriptionParseResult, error) {
	finalURL, err := validateExternalSourceURL(sourceURL)
	if err != nil {
		return externalSubscriptionParseResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finalURL, nil)
	if err != nil {
		return externalSubscriptionParseResult{}, fmt.Errorf("failed to prepare source request")
	}
	applyExternalHWIDHeaders(req, hwidProfile)
	req.Header.Set("Accept", "application/json,text/plain,*/*")

	client := newExternalSubscriptionHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		// net/http errors commonly embed the complete URL. Subscription URLs
		// frequently contain access tokens, so never persist or return them.
		return externalSubscriptionParseResult{}, fmt.Errorf("failed to fetch source")
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxExternalSubscriptionBodyBytes+1)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return externalSubscriptionParseResult{}, fmt.Errorf("failed to read source response: %w", err)
	}
	if len(bodyBytes) > maxExternalSubscriptionBodyBytes {
		return externalSubscriptionParseResult{}, fmt.Errorf("source response is too large (max 10 MB)")
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return externalSubscriptionParseResult{}, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}

	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		if resp.Header.Get("profile-title") != "" || resp.Header.Get("announce") != "" {
			return externalSubscriptionParseResult{}, fmt.Errorf("source returned subscription headers but empty body")
		}
		return externalSubscriptionParseResult{}, fmt.Errorf("subscription body is empty")
	}

	parsed, err := parseExternalSubscriptionBody(string(bodyBytes), fingerprintKeys)
	if err != nil {
		return externalSubscriptionParseResult{}, err
	}
	if len(parsed.Keys) == 0 {
		return externalSubscriptionParseResult{}, noSupportedExternalKeysError(parsed)
	}
	if len(parsed.Keys) > maxExternalImportItems {
		return externalSubscriptionParseResult{}, fmt.Errorf("too many keys in source response (max %d)", maxExternalImportItems)
	}

	remoteTitle, titleWarnings := normalizeRemoteProfileTitle(decodeSubscriptionHeaderValue(resp.Header.Get("profile-title")))
	parsed.Metadata = externalSubscriptionMetadata{
		Title:           remoteTitle,
		RefreshHours:    parsePositiveInt(resp.Header.Get("profile-update-interval")),
		SupportURL:      strings.TrimSpace(resp.Header.Get("support-url")),
		WebPageURL:      strings.TrimSpace(resp.Header.Get("profile-web-page-url")),
		Announce:        decodeSubscriptionHeaderValue(resp.Header.Get("announce")),
		ContentType:     strings.TrimSpace(resp.Header.Get("content-type")),
		ContentDisp:     strings.TrimSpace(resp.Header.Get("content-disposition")),
		SourceFinalURL:  strings.TrimSpace(resp.Request.URL.String()),
		HTTPStatusCode:  resp.StatusCode,
		HTTPStatusLabel: strings.TrimSpace(resp.Status),
	}
	parsed.Warnings = append(parsed.Warnings, titleWarnings...)
	return parsed, nil
}

func formatNullableTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Local().Format("2006-01-02 15:04:05")
}

func sourceRowToModel(row externalSourceRow) model.ExternalSubscriptionSource {
	return model.ExternalSubscriptionSource{
		ID:                  row.ID,
		Name:                row.Name,
		Category:            row.Category,
		KeyCategory:         row.KeyCategory,
		KeyInsertMode:       row.KeyInsertMode,
		SourceURL:           row.SourceURL,
		Enabled:             row.Enabled,
		ApplyRemoteMetadata: row.ApplyRemoteMetadata,
		PassHWID:            row.PassHWID,
		HWIDVersion:         row.HWIDVersion,
		HWIDModelName:       row.HWIDModelName,
		HWIDValue:           row.HWIDValue,
		LastImportCount:     row.LastImportCount,
		ImportStatus:        row.ImportStatus,
		LastError:           row.LastError,
		LastSyncedAt:        formatNullableTime(row.LastSyncedAt),
		MetaTitle:           row.MetaTitle,
		MetaRefreshHours:    row.MetaRefreshHours,
		MetaSupportURL:      row.MetaSupportURL,
		MetaWebPageURL:      row.MetaWebPageURL,
		MetaAnnounce:        row.MetaAnnounce,
		CreatedAt:           formatNullableTime(row.CreatedAt),
		UpdatedAt:           formatNullableTime(row.UpdatedAt),
	}
}

func (a *App) listExternalSources() ([]model.ExternalSubscriptionSource, error) {
	rows, err := a.db.Query(`
		SELECT id, name, category, key_category, key_insert_mode, source_url, enabled, apply_remote_metadata, pass_hwid, hwid_version, hwid_model_name, hwid_value, last_import_count, import_status, last_error,
		       last_synced_at, meta_title, meta_refresh_hours, meta_support_url, meta_web_page_url, meta_announce, created_at, updated_at
		FROM external_subscription_sources
		ORDER BY LOWER(category), id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ExternalSubscriptionSource, 0, 16)
	for rows.Next() {
		var row externalSourceRow
		var enabledInt int64
		var applyMetaInt int64
		var passHWIDInt int64
		var hwidVersion sql.NullString
		var hwidModelName sql.NullString
		var hwidValue sql.NullString
		var lastImportCountInt int64
		var importStatus sql.NullString
		var lastError sql.NullString
		var lastSyncedAt sql.NullTime
		var metaTitle sql.NullString
		var metaRefresh sql.NullInt64
		var metaSupport sql.NullString
		var metaWeb sql.NullString
		var metaAnnounce sql.NullString
		var createdAt sql.NullTime
		var updatedAt sql.NullTime
		if err := rows.Scan(
			&row.ID,
			&row.Name,
			&row.Category,
			&row.KeyCategory,
			&row.KeyInsertMode,
			&row.SourceURL,
			&enabledInt,
			&applyMetaInt,
			&passHWIDInt,
			&hwidVersion,
			&hwidModelName,
			&hwidValue,
			&lastImportCountInt,
			&importStatus,
			&lastError,
			&lastSyncedAt,
			&metaTitle,
			&metaRefresh,
			&metaSupport,
			&metaWeb,
			&metaAnnounce,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		row.Category = normalizeExternalSourceCategory(row.Category)
		row.KeyCategory = normalizeKeyCategory(row.KeyCategory)
		row.KeyInsertMode = normalizeKeyInsertMode(row.KeyInsertMode)
		row.Enabled = enabledInt != 0
		row.ApplyRemoteMetadata = applyMetaInt != 0
		row.PassHWID = passHWIDInt != 0
		row.HWIDVersion = strings.TrimSpace(hwidVersion.String)
		row.HWIDModelName = strings.TrimSpace(hwidModelName.String)
		row.HWIDValue = strings.TrimSpace(hwidValue.String)
		row.LastImportCount = int(lastImportCountInt)
		row.ImportStatus = strings.TrimSpace(importStatus.String)
		if row.ImportStatus == "" {
			row.ImportStatus = "idle"
		}
		row.LastError = strings.TrimSpace(lastError.String)
		row.LastSyncedAt = lastSyncedAt
		row.MetaTitle = strings.TrimSpace(metaTitle.String)
		if metaRefresh.Valid && metaRefresh.Int64 > 0 {
			row.MetaRefreshHours = int(metaRefresh.Int64)
		}
		row.MetaSupportURL = strings.TrimSpace(metaSupport.String)
		row.MetaWebPageURL = strings.TrimSpace(metaWeb.String)
		row.MetaAnnounce = strings.TrimSpace(metaAnnounce.String)
		row.CreatedAt = createdAt
		row.UpdatedAt = updatedAt
		out = append(out, sourceRowToModel(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *App) listExternalSourceCategories() ([]model.ExternalSourceCategory, error) {
	rows, err := a.db.Query(`SELECT name FROM external_source_categories ORDER BY LOWER(name), name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	countByName := make(map[string]int)
	categories := make([]model.ExternalSourceCategory, 0, 16)
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		normalized := normalizeExternalSourceCategory(name.String)
		if _, exists := countByName[normalized]; exists {
			continue
		}
		countByName[normalized] = 0
		categories = append(categories, model.ExternalSourceCategory{Name: normalized, SourcesCount: 0})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	countRows, err := a.db.Query(`
		SELECT category, COUNT(*)
		FROM external_subscription_sources
		GROUP BY category
	`)
	if err != nil {
		return nil, err
	}
	defer countRows.Close()

	for countRows.Next() {
		var category sql.NullString
		var count int64
		if err := countRows.Scan(&category, &count); err != nil {
			return nil, err
		}
		normalized := normalizeExternalSourceCategory(category.String)
		if _, exists := countByName[normalized]; !exists {
			categories = append(categories, model.ExternalSourceCategory{Name: normalized, SourcesCount: 0})
		}
		countByName[normalized] += int(count)
	}
	if err := countRows.Err(); err != nil {
		return nil, err
	}

	if len(categories) == 0 {
		categories = append(categories, model.ExternalSourceCategory{Name: "Общее", SourcesCount: 0})
		countByName["Общее"] = 0
	}

	for index := range categories {
		categories[index].SourcesCount = countByName[categories[index].Name]
	}

	sort.Slice(categories, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(categories[i].Name))
		right := strings.ToLower(strings.TrimSpace(categories[j].Name))
		if left == right {
			return categories[i].Name < categories[j].Name
		}
		return left < right
	})
	return categories, nil
}

func (a *App) getExternalSourceByID(id int64) (externalSourceRow, error) {
	var row externalSourceRow
	var enabledInt int64
	var applyMetaInt int64
	var passHWIDInt int64
	var hwidVersion sql.NullString
	var hwidModelName sql.NullString
	var hwidValue sql.NullString
	var lastImportCountInt int64
	var importStatus sql.NullString
	var lastError sql.NullString
	var lastSyncedAt sql.NullTime
	var metaTitle sql.NullString
	var metaRefresh sql.NullInt64
	var metaSupport sql.NullString
	var metaWeb sql.NullString
	var metaAnnounce sql.NullString
	var createdAt sql.NullTime
	var updatedAt sql.NullTime

	err := a.db.QueryRow(`
		SELECT id, name, category, key_category, key_insert_mode, source_url, enabled, apply_remote_metadata, pass_hwid, hwid_version, hwid_model_name, hwid_value, last_import_count, import_status, last_error,
		       last_synced_at, meta_title, meta_refresh_hours, meta_support_url, meta_web_page_url, meta_announce, created_at, updated_at
		FROM external_subscription_sources
		WHERE id = ?
	`, id).Scan(
		&row.ID,
		&row.Name,
		&row.Category,
		&row.KeyCategory,
		&row.KeyInsertMode,
		&row.SourceURL,
		&enabledInt,
		&applyMetaInt,
		&passHWIDInt,
		&hwidVersion,
		&hwidModelName,
		&hwidValue,
		&lastImportCountInt,
		&importStatus,
		&lastError,
		&lastSyncedAt,
		&metaTitle,
		&metaRefresh,
		&metaSupport,
		&metaWeb,
		&metaAnnounce,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return externalSourceRow{}, err
	}

	row.Category = normalizeExternalSourceCategory(row.Category)
	row.KeyCategory = normalizeKeyCategory(row.KeyCategory)
	row.KeyInsertMode = normalizeKeyInsertMode(row.KeyInsertMode)
	row.Enabled = enabledInt != 0
	row.ApplyRemoteMetadata = applyMetaInt != 0
	row.PassHWID = passHWIDInt != 0
	row.HWIDVersion = strings.TrimSpace(hwidVersion.String)
	row.HWIDModelName = strings.TrimSpace(hwidModelName.String)
	row.HWIDValue = strings.TrimSpace(hwidValue.String)
	row.LastImportCount = int(lastImportCountInt)
	row.ImportStatus = strings.TrimSpace(importStatus.String)
	if row.ImportStatus == "" {
		row.ImportStatus = "idle"
	}
	row.LastError = strings.TrimSpace(lastError.String)
	row.LastSyncedAt = lastSyncedAt
	row.MetaTitle = strings.TrimSpace(metaTitle.String)
	if metaRefresh.Valid && metaRefresh.Int64 > 0 {
		row.MetaRefreshHours = int(metaRefresh.Int64)
	}
	row.MetaSupportURL = strings.TrimSpace(metaSupport.String)
	row.MetaWebPageURL = strings.TrimSpace(metaWeb.String)
	row.MetaAnnounce = strings.TrimSpace(metaAnnounce.String)
	row.CreatedAt = createdAt
	row.UpdatedAt = updatedAt

	return row, nil
}

func normalizeImportStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "idle", "syncing", "ok", "error":
		return value
	default:
		return "idle"
	}
}

func (a *App) markExternalSourceStatus(sourceID int64, status string, errMessage string) {
	_, err := a.db.Exec(
		`UPDATE external_subscription_sources
		 SET import_status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		normalizeImportStatus(status),
		nullStringValue(errMessage),
		sourceID,
	)
	if err != nil {
		log.Printf("markExternalSourceStatus: source_id=%d err=%v", sourceID, err)
	}
}

func (a *App) syncExternalSource(sourceID int64, parsed externalSubscriptionParseResult) (externalSyncResult, error) {
	source, err := a.getExternalSourceByID(sourceID)
	if err != nil {
		return externalSyncResult{}, err
	}

	tx, err := a.db.Begin()
	if err != nil {
		return externalSyncResult{}, err
	}
	defer tx.Rollback()

	result, err := a.syncExternalSourceTx(tx, source, parsed)
	if err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func (a *App) syncExternalSourceTx(tx *sql.Tx, source externalSourceRow, parsed externalSubscriptionParseResult) (externalSyncResult, error) {
	result := externalSyncResult{Items: append([]externalSafeImportItem(nil), parsed.Items...)}
	result.Skipped = parsed.Counts.Rejected + parsed.Counts.Unsupported + parsed.Counts.Duplicate
	if len(parsed.Keys) == 0 {
		return result, fmt.Errorf("no_keys_to_import")
	}
	itemIndexes := make(map[string]int, len(result.Items))
	for index := range result.Items {
		itemIndexes[result.Items[index].ItemRef] = index
	}
	setItemStatus := func(itemRef, status string) {
		if index, exists := itemIndexes[itemRef]; exists {
			result.Items[index].Status = status
		}
	}

	sourceID := source.ID
	statusValue := model.KeyStatusActive
	if !source.Enabled {
		statusValue = model.KeyStatusNonActive
	}
	targetCategory := normalizeKeyCategory(source.KeyCategory)
	var targetCategoryID any
	if targetCategory != "" {
		var nextCategoryOrder int64
		if err := tx.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextCategoryOrder); err != nil {
			return result, err
		}
		if _, err := tx.Exec(`
			INSERT INTO key_categories(name, color, sort_order, updated_at)
			VALUES(?, '#d8b33d', ?, CURRENT_TIMESTAMP)
			ON CONFLICT(name) DO NOTHING
		`, targetCategory, nextCategoryOrder); err != nil {
			return result, err
		}
		var categoryID int64
		if err := tx.QueryRow(`SELECT id FROM key_categories WHERE name = ?`, targetCategory).Scan(&categoryID); err != nil {
			return result, err
		}
		targetCategoryID = categoryID
	}
	insertMode := normalizeKeyInsertMode(source.KeyInsertMode)

	type existingKey struct {
		ID                   int64
		Ref                  string
		Fingerprint          string
		Label                string
		URL                  string
		Protocol             string
		ProfileSchemaVersion int
		Compatibility        string
		WarningsJSON         string
	}
	activeID, activeKey, err := a.profileKeyring.GetActiveEncryptionKey()
	if err != nil {
		return result, err
	}

	existingRows, err := tx.Query(`
		SELECT k.id, k.external_key_ref, COALESCE(k.profile_fingerprint, ''), k.label, s.encrypted_url,
		       k.protocol, k.profile_schema_version, k.profile_compatibility, k.profile_warnings_json
		FROM vless_keys k
		LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		WHERE k.external_source_id = ?
	`, sourceID)
	if err != nil {
		return result, err
	}
	existingByRef := make(map[string]existingKey)
	existingByFingerprint := make(map[string]existingKey)
	existingIDs := make(map[int64]struct{})
	for existingRows.Next() {
		var key existingKey
		var encURL sql.NullString
		if err := existingRows.Scan(&key.ID, &key.Ref, &key.Fingerprint, &key.Label, &encURL, &key.Protocol, &key.ProfileSchemaVersion, &key.Compatibility, &key.WarningsJSON); err != nil {
			_ = existingRows.Close()
			return result, err
		}
		if encURL.Valid && encURL.String != "" {
			if dec, err := profilestorage.Decrypt(encURL.String, a.profileKeyring, key.ID); err == nil {
				key.URL = dec.Reveal()
			}
		}
		key.Ref = strings.TrimSpace(key.Ref)
		if key.Ref != "" {
			existingByRef[key.Ref] = key
		}
		key.Fingerprint = strings.TrimSpace(key.Fingerprint)
		if key.Fingerprint != "" {
			existingByFingerprint[key.Fingerprint] = key
		}
		existingIDs[key.ID] = struct{}{}
	}
	if err := existingRows.Err(); err != nil {
		_ = existingRows.Close()
		return result, err
	}
	_ = existingRows.Close()

	seenRefs := make(map[string]struct{}, len(parsed.Keys))
	var nextSortOrder int64
	if insertMode == "top" {
		if err := tx.QueryRow(
			`SELECT COALESCE(MIN(sort_order), 1) - ? FROM vless_keys WHERE category_id IS ? AND external_source_id != ?`,
			len(parsed.Keys)+8,
			targetCategoryID,
			sourceID,
		).Scan(&nextSortOrder); err != nil {
			return result, err
		}
	} else {
		if err := tx.QueryRow(
			`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys WHERE category_id IS ? AND external_source_id != ?`,
			targetCategoryID,
			sourceID,
		).Scan(&nextSortOrder); err != nil {
			return result, err
		}
	}

	for index, item := range parsed.Keys {
		ref := strings.TrimSpace(item.Ref)
		if ref == "" {
			ref = buildExternalKeyRef(item.URL)
		}
		if _, exists := seenRefs[ref]; exists {
			result.Skipped++
			setItemStatus(item.ItemRef, externalStatusDuplicate)
			continue
		}
		seenRefs[ref] = struct{}{}

		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = fmt.Sprintf("Импорт %03d", index+1)
		}
		if len(label) > 255 {
			label = label[:255]
		}

		urlValue := item.URL
		if strings.TrimSpace(urlValue) == "" {
			result.Skipped++
			setItemStatus(item.ItemRef, externalStatusRejected)
			continue
		}
		if len(urlValue) > 65535 {
			result.Skipped++
			setItemStatus(item.ItemRef, externalStatusRejected)
			continue
		}
		warningPayload, err := json.Marshal(item.WarningCodes)
		if err != nil {
			return result, err
		}
		warningsJSON := string(warningPayload)

		existing, matched := existingByRef[ref]
		if !matched {
			for _, candidate := range item.FingerprintCandidates {
				if candidateMatch, exists := existingByFingerprint[candidate]; exists {
					existing = candidateMatch
					matched = true
					break
				}
			}
		}
		if matched {
			stableRef := existing.Ref
			if stableRef == "" {
				stableRef = ref
			}
			changed := existing.Label != label || existing.URL != urlValue || existing.Protocol != item.Protocol ||
				existing.Fingerprint != item.Fingerprint || existing.ProfileSchemaVersion != item.ProfileSchemaVersion ||
				existing.Compatibility != item.Compatibility || existing.WarningsJSON != warningsJSON
			if _, err := tx.Exec(
				`UPDATE vless_keys
				 SET label = ?, category_id = ?, category = ?, status = ?, key_kind = 'real', template_text = NULL,
				     external_source_id = ?, external_key_ref = ?, sort_order = ?, protocol = ?, profile_fingerprint = ?,
				     profile_schema_version = ?, profile_compatibility = ?, profile_warnings_json = ?
				 WHERE id = ? AND external_source_id = ?`,
				label, targetCategoryID, targetCategory, statusValue, sourceID, stableRef, nextSortOrder,
				item.Protocol, nullStringValue(item.Fingerprint), item.ProfileSchemaVersion, item.Compatibility, warningsJSON, existing.ID, sourceID,
			); err != nil {
				return result, err
			}
			env, err := profilestorage.Encrypt([]byte(urlValue), activeID, activeKey, existing.ID)
			if err != nil {
				return result, err
			}
			if _, err := tx.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?) ON CONFLICT(vless_key_id) DO UPDATE SET encrypted_url = excluded.encrypted_url`, existing.ID, env); err != nil {
				return result, err
			}
			delete(existingIDs, existing.ID)
			nextSortOrder++
			result.Imported++
			if changed {
				setItemStatus(item.ItemRef, externalStatusUpdated)
			} else {
				setItemStatus(item.ItemRef, externalStatusUnchanged)
			}
			continue
		}

		insertResult, err := tx.Exec(
			`INSERT INTO vless_keys(
				label, category_id, category, status, check_status, key_kind, template_text, sort_order,
				external_source_id, external_key_ref, protocol, profile_fingerprint, profile_schema_version,
				profile_compatibility, profile_warnings_json
			) VALUES(?, ?, ?, ?, 'unknown', 'real', NULL, ?, ?, ?, ?, ?, ?, ?, ?)`,
			label, targetCategoryID, targetCategory, statusValue, nextSortOrder, sourceID, ref,
			item.Protocol, nullStringValue(item.Fingerprint), item.ProfileSchemaVersion, item.Compatibility, warningsJSON,
		)
		if err != nil {
			errText := strings.ToLower(err.Error())
			if strings.Contains(errText, "unique") {
				result.Skipped++
				setItemStatus(item.ItemRef, externalStatusDuplicate)
				continue
			}
			return result, err
		}
		keyID, err := insertResult.LastInsertId()
		if err != nil {
			return result, err
		}
		env, err := profilestorage.Encrypt([]byte(urlValue), activeID, activeKey, keyID)
		if err != nil {
			return result, err
		}
		if _, err := tx.Exec(`INSERT INTO vless_key_secrets(vless_key_id, encrypted_url) VALUES(?, ?)`, keyID, env); err != nil {
			return result, err
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO user_keys(user_id, key_id)
			 SELECT id, ? FROM users WHERE key_assignment_mode = 'all'`,
			keyID,
		); err != nil {
			return result, err
		}
		nextSortOrder++
		result.Imported++
	}

	// Any rejected/unsupported source item makes absence ambiguous. Preserve
	// unmatched rows until a fully parsed refresh confirms they are missing.
	if parsed.Counts.Rejected == 0 && parsed.Counts.Unsupported == 0 {
		for keyID := range existingIDs {
			if _, err := tx.Exec(`DELETE FROM vless_keys WHERE id = ? AND external_source_id = ?`, keyID, sourceID); err != nil {
				return result, err
			}
		}
	}
	result.Counts = countExternalItems(result.Items)

	if _, err := tx.Exec(
		`UPDATE external_subscription_sources
		 SET import_status = 'ok',
		     last_error = NULL,
		     last_synced_at = CURRENT_TIMESTAMP,
		     last_import_count = ?,
		     meta_title = ?,
		     meta_refresh_hours = ?,
		     meta_support_url = ?,
		     meta_web_page_url = ?,
		     meta_announce = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		result.Imported,
		nullStringValue(parsed.Metadata.Title),
		nullInt64Value(int64(parsed.Metadata.RefreshHours)),
		nullStringValue(parsed.Metadata.SupportURL),
		nullStringValue(parsed.Metadata.WebPageURL),
		nullStringValue(parsed.Metadata.Announce),
		sourceID,
	); err != nil {
		return result, err
	}

	return result, nil
}

func (a *App) apiListExternalSources(w http.ResponseWriter, r *http.Request) {
	sources, err := a.listExternalSources()
	if err != nil {
		log.Printf("apiListExternalSources: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list external sources")
		return
	}
	if sources == nil {
		sources = []model.ExternalSubscriptionSource{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sources": sources,
	})
}

func (a *App) apiListExternalSourceCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.listExternalSourceCategories()
	if err != nil {
		log.Printf("apiListExternalSourceCategories: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list source categories")
		return
	}
	if categories == nil {
		categories = []model.ExternalSourceCategory{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
	})
}

func (a *App) apiCreateExternalSourceCategory(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceCategoryCreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rawName := strings.TrimSpace(req.Name)
	if rawName == "" {
		writeError(w, http.StatusBadRequest, "category name is required")
		return
	}
	name := normalizeExternalSourceCategory(rawName)

	if err := a.upsertExternalSourceCategory(name); err != nil {
		log.Printf("apiCreateExternalSourceCategory: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create source category")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"category": model.ExternalSourceCategory{Name: name},
		"message":  "source category saved",
	})
}

func (a *App) apiRenameExternalSourceCategory(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceCategoryRenameRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	oldRaw := strings.TrimSpace(req.OldName)
	newRaw := strings.TrimSpace(req.NewName)
	if oldRaw == "" || newRaw == "" {
		writeError(w, http.StatusBadRequest, "both old_name and new_name are required")
		return
	}
	oldName := normalizeExternalSourceCategory(oldRaw)
	newName := normalizeExternalSourceCategory(newRaw)
	if oldName == newName {
		writeMessage(w, "category name unchanged")
		return
	}

	var sourceCount int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources WHERE category = ?`, oldName).Scan(&sourceCount); err != nil {
		log.Printf("apiRenameExternalSourceCategory count sources: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}
	var categoryCount int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM external_source_categories WHERE name = ?`, oldName).Scan(&categoryCount); err != nil {
		log.Printf("apiRenameExternalSourceCategory count categories: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}
	if sourceCount == 0 && categoryCount == 0 {
		writeError(w, http.StatusNotFound, "source category not found")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO external_source_categories(name, updated_at)
		 VALUES(?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		newName,
	); err != nil {
		log.Printf("apiRenameExternalSourceCategory upsert new category: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	if _, err := tx.Exec(
		`UPDATE external_subscription_sources
		 SET category = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE category = ?`,
		newName,
		oldName,
	); err != nil {
		log.Printf("apiRenameExternalSourceCategory update sources: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	if _, err := tx.Exec(`DELETE FROM external_source_categories WHERE name = ?`, oldName); err != nil {
		log.Printf("apiRenameExternalSourceCategory delete old category: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	writeMessage(w, "source category renamed")
}

func (a *App) apiPreviewExternalSource(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourcePreviewRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sourceURL, err := validateExternalSourceURL(req.SourceURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	hwidProfile := normalizeExternalHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)
	metadata := externalSubscriptionMetadata{
		ContentType:    strings.TrimSpace(req.RawContentType),
		SourceFinalURL: strings.TrimSpace(req.RawFinalURL),
	}
	parsed, err := func() (externalSubscriptionParseResult, error) {
		if strings.TrimSpace(req.RawBody) != "" {
			return parseExternalSubscriptionFromRawBody(sourceURL, req.RawBody, metadata, a.externalProfileFingerprintKeys())
		}
		return fetchExternalSubscription(sourceURL, hwidProfile, a.externalProfileFingerprintKeys())
	}()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source_url":      sourceURL,
		"suggested_name":  suggestExternalSourceName(sourceURL, parsed.Metadata),
		"detected_format": parsed.DetectedFormat,
		"key_count":       len(parsed.Keys),
		"metadata": map[string]any{
			"title":            parsed.Metadata.Title,
			"refresh_hours":    parsed.Metadata.RefreshHours,
			"support_url":      parsed.Metadata.SupportURL,
			"profile_web_page": parsed.Metadata.WebPageURL,
			"announce":         parsed.Metadata.Announce,
			"content_type":     parsed.Metadata.ContentType,
			"content_disp":     parsed.Metadata.ContentDisp,
			"http_status":      parsed.Metadata.HTTPStatusLabel,
			"final_url":        parsed.Metadata.SourceFinalURL,
		},
		"warnings":      nonNilWarnings(parsed.Warnings),
		"result_counts": parsed.Counts,
		"keys":          parsed.Items,
	})
}

func (a *App) apiImportExternalSource(w http.ResponseWriter, r *http.Request) {
	a.apiImportExternalSourceTransactional(w, r)
}

func (a *App) apiImportExternalSourceTransactional(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceImportRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sourceURL, err := validateExternalSourceURL(req.SourceURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var sourceExists bool
	if err := a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM external_subscription_sources WHERE source_url = ?)`, sourceURL).Scan(&sourceExists); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create external source")
		return
	}
	if sourceExists {
		writeError(w, http.StatusConflict, "source_url already exists")
		return
	}
	name, err := validateExternalSourceName(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	category := normalizeExternalSourceCategory(req.Category)
	keyCategory := normalizeKeyCategory(req.KeyCategory)
	hwidProfile := normalizeExternalHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)
	metadata := externalSubscriptionMetadata{ContentType: strings.TrimSpace(req.RawContentType), SourceFinalURL: strings.TrimSpace(req.RawFinalURL)}
	var parsed externalSubscriptionParseResult
	if strings.TrimSpace(req.RawBody) != "" {
		parsed, err = parseExternalSubscriptionFromRawBody(sourceURL, req.RawBody, metadata, a.externalProfileFingerprintKeys())
	} else {
		parsed, err = fetchExternalSubscription(sourceURL, hwidProfile, a.externalProfileFingerprintKeys())
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	parsed, err = filterExternalSelection(parsed, req.SelectedItemRefs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create external source")
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO external_source_categories(name) VALUES(?)`, category); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save source category")
		return
	}
	insertResult, err := tx.Exec(`
		INSERT INTO external_subscription_sources(
			name, category, key_category, key_insert_mode, source_url, enabled, apply_remote_metadata,
			pass_hwid, hwid_version, hwid_model_name, hwid_value, import_status, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, 'syncing', CURRENT_TIMESTAMP)
	`, name, category, keyCategory, normalizeKeyInsertMode(req.KeyInsertMode), sourceURL, boolToInt(req.Enabled),
		boolToInt(hwidProfile.PassHWID), nullStringValue(hwidProfile.Version), nullStringValue(hwidProfile.ModelName), nullStringValue(hwidProfile.HWID))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "source_url already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create external source")
		return
	}
	sourceID, err := insertResult.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create external source")
		return
	}
	source := externalSourceRow{ID: sourceID, Name: name, Category: category, KeyCategory: keyCategory, KeyInsertMode: normalizeKeyInsertMode(req.KeyInsertMode), SourceURL: sourceURL, Enabled: req.Enabled, PassHWID: hwidProfile.PassHWID, HWIDVersion: hwidProfile.Version, HWIDModelName: hwidProfile.ModelName, HWIDValue: hwidProfile.HWID, ImportStatus: "syncing"}
	syncResult, err := a.syncExternalSourceTx(tx, source, parsed)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create external source")
		return
	}
	created, err := a.getExternalSourceByID(sourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "source was imported but failed to load response")
		return
	}
	a.recordAuditEvent(r, "external_source.create", "external_source", strconv.FormatInt(sourceID, 10), map[string]any{"imported_count": syncResult.Imported, "skipped_count": syncResult.Skipped, "result_counts": syncResult.Counts})
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "external source imported", "imported_count": syncResult.Imported, "skipped_count": syncResult.Skipped,
		"result_counts": syncResult.Counts, "items": syncResult.Items, "warnings": nonNilWarnings(parsed.Warnings),
		"detected_format": parsed.DetectedFormat, "source": sourceRowToModel(created),
	})
}

func (a *App) apiUpdateExternalSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req model.ExternalSourceUpdateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sourceURL, err := validateExternalSourceURL(req.SourceURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name, err := validateExternalSourceName(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	category := normalizeExternalSourceCategory(req.Category)
	keyCategory := normalizeKeyCategory(req.KeyCategory)
	keyInsertMode := normalizeKeyInsertMode(req.KeyInsertMode)
	if err := a.upsertExternalSourceCategory(category); err != nil {
		log.Printf("apiUpdateExternalSource: upsert category: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save source category")
		return
	}
	hwidProfile := normalizeExternalHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)

	res, err := a.db.Exec(
		`UPDATE external_subscription_sources
		 SET name = ?, category = ?, key_category = ?, key_insert_mode = ?, source_url = ?, enabled = ?, apply_remote_metadata = ?, pass_hwid = ?,
		     hwid_version = ?, hwid_model_name = ?, hwid_value = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		name,
		category,
		keyCategory,
		keyInsertMode,
		sourceURL,
		boolToInt(req.Enabled),
		0,
		boolToInt(hwidProfile.PassHWID),
		nullStringValue(hwidProfile.Version),
		nullStringValue(hwidProfile.ModelName),
		nullStringValue(hwidProfile.HWID),
		id,
	)
	if err != nil {
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "source_url") || strings.Contains(errText, "unique") {
			writeError(w, http.StatusConflict, "source_url already exists")
			return
		}
		log.Printf("apiUpdateExternalSource: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update external source")
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "external source not found")
		return
	}

	keyStatus := model.KeyStatusActive
	if !req.Enabled {
		keyStatus = model.KeyStatusNonActive
	}
	if _, err := a.db.Exec(`UPDATE vless_keys SET status = ? WHERE external_source_id = ?`, keyStatus, id); err != nil {
		log.Printf("apiUpdateExternalSource: update key statuses: %v", err)
	}

	writeMessage(w, "external source updated")
}

func (a *App) apiDeleteExternalSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete external source")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM vless_keys WHERE external_source_id = ?`, id); err != nil {
		log.Printf("apiDeleteExternalSource: delete keys: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete source keys")
		return
	}

	res, err := tx.Exec(`DELETE FROM external_subscription_sources WHERE id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteExternalSource: delete source: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete external source")
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "external source not found")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete external source")
		return
	}

	writeMessage(w, "external source deleted")
}

func (a *App) apiSyncExternalSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	source, err := a.getExternalSourceByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "external source not found")
			return
		}
		log.Printf("apiSyncExternalSource: load source: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load external source")
		return
	}

	jobID := a.startTrackedJob("source_sync", "external_source", strconv.FormatInt(id, 10))
	runID := a.startSourceSyncRun(id)
	a.markExternalSourceStatus(id, "syncing", "")
	hwidProfile := normalizeExternalHWIDProfile(source.PassHWID, source.HWIDVersion, source.HWIDModelName, source.HWIDValue)
	parsed, err := fetchExternalSubscription(source.SourceURL, hwidProfile, a.externalProfileFingerprintKeys())
	if err != nil {
		a.markExternalSourceStatus(id, "error", err.Error())
		a.finishTrackedJob(jobID, err)
		a.finishSourceSyncRun(runID, externalSyncResult{}, err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	syncResult, syncErr := a.syncExternalSource(id, parsed)
	if syncErr != nil {
		a.markExternalSourceStatus(id, "error", syncErr.Error())
		a.finishTrackedJob(jobID, syncErr)
		a.finishSourceSyncRun(runID, syncResult, syncErr)
		writeError(w, http.StatusBadRequest, syncErr.Error())
		return
	}
	a.finishTrackedJob(jobID, nil)
	a.finishSourceSyncRun(runID, syncResult, nil)
	a.recordAuditEvent(r, "external_source.sync", "external_source", strconv.FormatInt(id, 10), map[string]any{
		"imported_count": syncResult.Imported,
		"skipped_count":  syncResult.Skipped,
		"result_counts":  syncResult.Counts,
	})

	updatedSource, err := a.getExternalSourceByID(id)
	if err != nil {
		log.Printf("apiSyncExternalSource: reload source: %v", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"message":        "external source synced",
			"imported_count": syncResult.Imported,
			"skipped_count":  syncResult.Skipped,
			"result_counts":  syncResult.Counts,
			"items":          syncResult.Items,
			"warnings":       nonNilWarnings(parsed.Warnings),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":        "external source synced",
		"imported_count": syncResult.Imported,
		"skipped_count":  syncResult.Skipped,
		"result_counts":  syncResult.Counts,
		"items":          syncResult.Items,
		"warnings":       nonNilWarnings(parsed.Warnings),
		"source":         sourceRowToModel(updatedSource),
	})
}
