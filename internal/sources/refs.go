package sources

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func KeyRef(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:16])
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

// bracketHost wraps a bare IPv6 literal in brackets so it can carry a port.
func bracketHost(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
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
	return bracketHost(host) + ":" + port
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

// hasAmbiguityWarning reports whether the profile warnings make the import
// ambiguous rather than cleanly accepted.
func hasAmbiguityWarning(codes []string) bool {
	return ContainsWarningCode(codes, profiles.WarningAmbiguousParameter, profiles.WarningConflictingPreference)
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
