package sources

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func parseExternalProfileItem(raw string, lineIndex int, scheme string, fingerprintKeys [][]byte) (*ParsedKey, ImportItem, error) {
	itemRef, err := buildExternalItemRef(raw, lineIndex, fingerprintKeys)
	if err != nil {
		return nil, ImportItem{}, err
	}
	profile, original, code := parseOriginalProfile(raw)
	if code != "" {
		item, itemErr := rejectedExternalItem(raw, lineIndex, scheme, StatusRejected, code, fingerprintKeys)
		return nil, item, itemErr
	}
	metadata := profile.SafeMetadata()
	warnings := profileWarningCodes(profile)
	fingerprints, code := profileFingerprints(profile, metadata.Capabilities.Fingerprint, fingerprintKeys)
	if code != "" {
		item, itemErr := rejectedExternalItem(raw, lineIndex, scheme, StatusRejected, code, fingerprintKeys)
		return nil, item, itemErr
	}
	status := initialProfileStatus(metadata.Capabilities.Status, warnings)
	key := &ParsedKey{
		Label: labelOrDefault(metadata.DisplayName, "Import %03d", lineIndex), URL: original.URI.Reveal(), Scheme: scheme, Protocol: string(metadata.Protocol),
		Host: metadata.Server, Port: metadata.Port, Ref: KeyRef(original.URI.Reveal()),
		ItemRef: itemRef, LineIndex: lineIndex, Compatibility: string(metadata.Capabilities.Status),
		ProfileSchemaVersion: ExternalProfileSchemaVersion, FingerprintCandidates: fingerprints,
		WarningCodes: warnings, InitialStatus: status,
	}
	if len(fingerprints) > 0 {
		key.Fingerprint = fingerprints[0]
	}
	return key, safeExternalItem(*key, status, ""), nil
}

// parseOriginalProfile parses raw and returns its exact original
// serialization, or the rejection code when either step fails.
func parseOriginalProfile(raw string) (*profiles.Profile, profiles.SerializationResult, string) {
	profile, err := profiles.Parse(raw)
	if err != nil {
		return nil, profiles.SerializationResult{}, profileErrorCode(err, "invalid_profile")
	}
	original, err := profiles.Serialize(profile, profiles.OriginalSerialization)
	if err != nil || !original.Exact {
		return nil, profiles.SerializationResult{}, "original_serialization_unavailable"
	}
	return profile, original, ""
}

func profileErrorCode(err error, fallback string) string {
	code := string(profiles.ErrorCodeOf(err))
	if code == "" {
		return fallback
	}
	return code
}

// profileFingerprints computes one fingerprint per key when the profile
// supports fingerprinting; the second value is the rejection code on failure.
func profileFingerprints(profile *profiles.Profile, enabled bool, fingerprintKeys [][]byte) ([]string, string) {
	fingerprints := make([]string, 0, len(fingerprintKeys))
	if !enabled {
		return fingerprints, ""
	}
	for _, key := range fingerprintKeys {
		fingerprint, err := profiles.Fingerprint(profile, key)
		if err != nil {
			return nil, profileErrorCode(err, "fingerprint_failed")
		}
		fingerprints = append(fingerprints, fingerprint)
	}
	return fingerprints, ""
}

func initialProfileStatus(capability profiles.CapabilityStatus, warnings []string) string {
	if capability == profiles.CapabilityReadOnly {
		return StatusCompatibilityOnly
	}
	if hasAmbiguityWarning(warnings) {
		return StatusAmbiguous
	}
	return StatusAccepted
}

// labelOrDefault trims label and falls back to format applied to lineIndex.
func labelOrDefault(label, format string, lineIndex int) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return fmt.Sprintf(format, lineIndex)
	}
	return label
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
		if isSkippableLine(raw) {
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

func CountItems(items []ImportItem) Counts {
	var counts Counts
	for _, item := range items {
		readOnly := item.Compatibility == string(profiles.CapabilityReadOnly)
		if readOnly || item.Status == StatusCompatibilityOnly {
			counts.CompatibilityOnly++
		}
		if hasAmbiguityWarning(item.Warnings) || item.Status == StatusAmbiguous {
			counts.Ambiguous++
		}
		counts.countStatus(item.Status)
	}
	return counts
}

// countStatus tallies the statuses that map one-to-one onto a counter.
// Compatibility-only and ambiguous are handled by CountItems because they
// also derive from the item's compatibility and warnings.
func (counts *Counts) countStatus(status string) {
	switch status {
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
	case StatusUnsupported:
		counts.Unsupported++
	}
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
	filteredKeys, wanted, err := selectedKeys(parsed.Keys, selected)
	if err != nil {
		return ParseResult{}, err
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

// selectedKeys resolves item references in selection order and returns the
// keys plus the set of accepted references.
func selectedKeys(keys []ParsedKey, selected []string) ([]ParsedKey, map[string]struct{}, error) {
	byRef := make(map[string]ParsedKey, len(keys))
	for _, key := range keys {
		byRef[key.ItemRef] = key
	}
	wanted := make(map[string]struct{}, len(selected))
	filteredKeys := make([]ParsedKey, 0, len(selected))
	for _, itemRef := range selected {
		itemRef = strings.TrimSpace(itemRef)
		if itemRef == "" {
			return nil, nil, fmt.Errorf("invalid_item_reference")
		}
		if _, duplicate := wanted[itemRef]; duplicate {
			return nil, nil, fmt.Errorf("duplicate_item_reference")
		}
		key, exists := byRef[itemRef]
		if !exists {
			return nil, nil, fmt.Errorf("invalid_item_reference")
		}
		wanted[itemRef] = struct{}{}
		filteredKeys = append(filteredKeys, key)
	}
	return filteredKeys, wanted, nil
}

func ParseBody(raw string, fingerprintKeys [][]byte) (ParseResult, error) {
	body, err := normalizedSubscriptionBody(raw, fingerprintKeys)
	if err != nil {
		return ParseResult{}, err
	}
	warnings := make([]string, 0)
	decoded, base64Encoded, err := maybeDecodeBase64SubscriptionBody(body)
	if err != nil {
		return ParseResult{}, err
	}
	if base64Encoded {
		body = decoded
		warnings = append(warnings, "base64_decoded")
	}
	result, err := parseDetectedBody(body, fingerprintKeys)
	if err != nil {
		return ParseResult{}, err
	}
	result.Warnings = append(warnings, result.Warnings...)
	return result, nil
}

// normalizedSubscriptionBody applies the size and key preconditions and
// strips the BOM and surrounding whitespace.
func normalizedSubscriptionBody(raw string, fingerprintKeys [][]byte) (string, error) {
	if len(fingerprintKeys) == 0 || len(fingerprintKeys[0]) == 0 {
		return "", fmt.Errorf("profile_fingerprint_key_unavailable")
	}
	if len(raw) > MaxBodyBytes {
		return "", fmt.Errorf("subscription_body_too_large")
	}
	body := strings.TrimSpace(trimUTF8BOM(raw))
	if body == "" {
		return "", fmt.Errorf("subscription body is empty")
	}
	return body, nil
}

// parseDetectedBody routes a decoded body to the JSON or link-list parser and
// fills the format, keys, items, counts and the partial_import warning.
func parseDetectedBody(body string, fingerprintKeys [][]byte) (ParseResult, error) {
	result := ParseResult{DetectedFormat: "links"}
	var err error
	if json.Valid([]byte(body)) {
		result.DetectedFormat = "xray-json"
		result.Keys, result.Items, err = parseExternalJSONBody(body, 1, fingerprintKeys)
	} else {
		result.Keys, result.Items, err = parseExternalLinkBody(body, fingerprintKeys)
	}
	if err != nil {
		return ParseResult{}, err
	}
	result.Counts = CountItems(result.Items)
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

// isSkippableLine reports blank lines and comment lines in a link list.
func isSkippableLine(raw string) bool {
	if raw == "" {
		return true
	}
	return strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, ";")
}
