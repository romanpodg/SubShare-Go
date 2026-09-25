package delivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func isUnsafeRune(char rune) bool {
	return char == 0x7f || char < 0x20
}

// isUnsafeStructuredRune is isUnsafeRune with the whitespace a JSON or YAML
// body legitimately contains carved out.
func isUnsafeStructuredRune(char rune) bool {
	switch char {
	case '\t', '\n', '\r':
		return false
	}
	return isUnsafeRune(char)
}

func HasUnsafeControl(raw string) bool {
	return strings.ContainsFunc(raw, isUnsafeRune)
}

func HasUnsafeStoredControl(raw string) bool {
	if profileconfig.SupportedConfigScheme(raw) == model.SubscriptionFormatXrayJSON {
		return hasUnsafeStructuredControl(raw)
	}
	return HasUnsafeControl(raw)
}

func hasUnsafeStructuredControl(body string) bool {
	return strings.ContainsFunc(body, isUnsafeStructuredRune)
}

func SafeProtocolName(raw, fallback string) string {
	scheme := profileconfig.SupportedConfigScheme(raw)
	if scheme == "" {
		scheme = strings.ToLower(strings.TrimSpace(fallback))
	}
	if scheme == "hy2" {
		return "hysteria2"
	}
	if scheme == "ss" {
		return "shadowsocks"
	}
	return scheme
}

// Identity is the delivery-time dedupe identity of a stored profile: a keyed
// fingerprint when the URI parses as a profile, otherwise a keyed digest of
// the exact bytes.
func Identity(raw string, key []byte) (string, error) {
	if len(key) == 0 {
		return "", fmt.Errorf("profile_fingerprint_key_unavailable")
	}
	if profile, err := profiles.Parse(raw); err == nil {
		fingerprint, fingerprintErr := profiles.Fingerprint(profile, key)
		if fingerprintErr != nil {
			return "", fmt.Errorf("profile_fingerprint_failed")
		}
		return "profile:" + fingerprint, nil
	}
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte("subshare-delivery-raw-v1\x00"))
	_, _ = digest.Write([]byte(raw))
	return "raw:" + hex.EncodeToString(digest.Sum(nil)), nil
}

func ValidateStoredEntry(raw string) error {
	switch profileconfig.SupportedConfigScheme(raw) {
	case "vless", "vmess", "trojan", "ss", "hysteria2", "hy2", "tuic":
		if _, err := profiles.Parse(raw); err == nil {
			return nil
		}
		if _, err := profileconfig.ParseLinkConfiguration(raw); err == nil {
			return nil
		}
		return errors.New(ReasonInvalidStored)
	case model.SubscriptionFormatXrayJSON:
		var root map[string]any
		if err := json.Unmarshal([]byte(raw), &root); err != nil || len(profileconfig.AsArray(root["outbounds"])) == 0 {
			return errors.New(ReasonInvalidStored)
		}
		return nil
	default:
		return errors.New(ReasonInvalidStored)
	}
}
