package delivery

import (
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

// shadowsocksMethods are the AEAD and SIP022 ciphers both mihomo and sing-box
// accept at their pinned versions.
var shadowsocksMethods = map[string]struct{}{
	"aes-128-gcm": {}, "aes-192-gcm": {}, "aes-256-gcm": {},
	"chacha20-ietf-poly1305": {}, "xchacha20-ietf-poly1305": {},
	"2022-blake3-aes-128-gcm": {}, "2022-blake3-aes-256-gcm": {},
	"2022-blake3-chacha20-poly1305": {},
}

func supportsShadowsocksMethod(methods map[string]struct{}, method string) bool {
	_, ok := methods[strings.ToLower(method)]
	return ok
}

// shadowsocksPluginOptions normalizes the plugin name and parses its
// key=value option string.
func shadowsocksPluginOptions(plugin *profiles.ShadowsocksPlugin) (string, map[string]string, bool) {
	options, ok := parsePluginOptions(plugin.Options.Reveal())
	return strings.ToLower(strings.TrimSpace(plugin.Name)), options, ok
}

func parsePluginOptions(raw string) (map[string]string, bool) {
	result := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return result, true
	}
	for _, part := range strings.Split(raw, ";") {
		if part == "" {
			continue
		}
		key, value, ok := pluginOption(part)
		if !ok {
			return nil, false
		}
		if _, duplicate := result[key]; duplicate {
			return nil, false
		}
		result[key] = value
	}
	return result, true
}

// pluginOption splits one "key=value" plugin option; both sides must be
// non-empty.
func pluginOption(part string) (string, string, bool) {
	key, value, hasValue := strings.Cut(part, "=")
	key = strings.ToLower(strings.TrimSpace(key))
	value = strings.TrimSpace(value)
	if !hasValue || key == "" {
		return "", "", false
	}
	if value == "" {
		return "", "", false
	}
	return key, value, true
}
