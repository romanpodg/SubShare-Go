package sources

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
)

const (
	asciiLetters   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	asciiDigits    = "0123456789"
	base64Alphabet = asciiLetters + asciiDigits + "+/-_="
)

var errInvalidBase64Subscription = errors.New("invalid_base64_subscription")

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
func maybeDecodeBase64SubscriptionBody(raw string) (string, bool, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" || bodyLooksLikeSupportedSubscription(candidate) {
		return "", false, nil
	}
	candidate, explicit := stripBase64Marker(candidate)
	encoded, base64Like := compactBase64Body(candidate)
	if !base64Like || len(encoded) < 24 {
		if explicit {
			return "", true, errInvalidBase64Subscription
		}
		return "", false, nil
	}
	decoded, err := decodeBase64Subscription(encoded)
	if err != nil {
		return "", true, err
	}
	return decoded, true, nil
}

// stripBase64Marker removes an explicit "base64:" prefix and reports whether
// one was present.
func stripBase64Marker(candidate string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(candidate), "base64:") {
		return candidate, false
	}
	return strings.TrimSpace(candidate[len("base64:"):]), true
}

// decodeBase64Subscription decodes a compacted Base64 body and insists that
// the result is a recognised subscription format.
func decodeBase64Subscription(encoded string) (string, error) {
	if strings.ContainsAny(encoded, "+/") && strings.ContainsAny(encoded, "-_") {
		return "", errInvalidBase64Subscription
	}
	encoding := base64EncodingFor(encoded)
	if encoding.DecodedLen(len(encoded)) > MaxBodyBytes {
		return "", fmt.Errorf("subscription_body_too_large")
	}
	decodedBytes, err := encoding.DecodeString(encoded)
	if err != nil {
		return "", errInvalidBase64Subscription
	}
	decoded := strings.TrimSpace(trimUTF8BOM(string(decodedBytes)))
	if decoded == "" || !bodyLooksLikeSupportedSubscription(decoded) {
		return "", errInvalidBase64Subscription
	}
	return decoded, nil
}

// compactBase64Body strips ASCII whitespace and reports whether every
// remaining character belongs to the standard or URL-safe Base64 alphabet.
func compactBase64Body(candidate string) (string, bool) {
	var compact strings.Builder
	compact.Grow(len(candidate))
	base64Like := true
	for _, char := range candidate {
		switch {
		case strings.ContainsRune(" \t\r\n", char):
			continue
		case strings.ContainsRune(base64Alphabet, char):
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
func externalURIScheme(raw string) string {
	value := strings.TrimSpace(raw)
	separator := strings.IndexByte(value, ':')
	if separator <= 0 || !strings.HasPrefix(value[separator+1:], "//") {
		return ""
	}
	scheme := value[:separator]
	for index, char := range scheme {
		if !isURISchemeChar(char, index) {
			return ""
		}
	}
	return strings.ToLower(scheme)
}

// isURISchemeChar accepts RFC 3986 scheme characters: a leading letter, then
// letters, digits, "+", "-" and ".".
func isURISchemeChar(char rune, index int) bool {
	if strings.ContainsRune(asciiLetters, char) {
		return true
	}
	return index > 0 && strings.ContainsRune(asciiDigits+"+-.", char)
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
