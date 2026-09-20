package delivery

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

var xrayShadowsocksMethods = map[string]struct{}{
	"aes-128-gcm": {}, "aes-256-gcm": {},
	"chacha20-poly1305": {}, "chacha20-ietf-poly1305": {},
	"xchacha20-poly1305": {}, "xchacha20-ietf-poly1305": {},
	"2022-blake3-aes-128-gcm": {}, "2022-blake3-aes-256-gcm": {},
	"2022-blake3-chacha20-poly1305": {},
}

func RenderXray(entries []Entry) (Generated, error) {
	result := Generated{Exclusions: []Exclusion{}}
	lines := make([]string, 0, len(entries))
	names := &UniqueNames{}
	for _, entry := range entries {
		if entry.Kind == model.KeyKindInformational {
			continue
		}
		converted, exclusion := xrayJSONForEntryWithNames(entry, names)
		result.Exclusions = append(result.Exclusions, exclusionList(exclusion)...)
		lines = append(lines, converted...)
	}
	body, err := xrayJSONArray(lines)
	if err != nil {
		return result, err
	}
	result.Body = body
	result.GeneratedCount = len(lines)
	return result, nil
}

// xrayJSONArray packs already-serialized documents into one JSON array.
func xrayJSONArray(documents []string) (string, error) {
	items := make([]json.RawMessage, 0, len(documents))
	for _, document := range documents {
		if !json.Valid([]byte(document)) {
			return "", errors.New(ReasonSerialization)
		}
		items = append(items, json.RawMessage(document))
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return "", errors.New(ReasonSerialization)
	}
	return string(payload), nil
}

func xrayJSONForEntryWithNames(entry Entry, names *UniqueNames) ([]string, *Exclusion) {
	switch scheme := profileconfig.SupportedConfigScheme(entry.Raw); scheme {
	case "ss":
		return xrayShadowsocksOutbound(entry, names)
	case "hysteria2", "hy2":
		return xrayHysteria2Outbound(entry, names)
	case "tuic":
		return nil, xrayTUICExclusion(entry)
	case model.SubscriptionFormatXrayJSON:
		return xrayNormalizedStored(entry)
	}
	converted, err := NormalizeForOutput(entry.Raw, model.SubscriptionFormatXrayJSON, firstNonEmpty(entry.ClientDisplayName, entry.Label))
	if err != nil {
		return nil, entry.exclude(entry.protocolName(), "xray-json", ReasonInvalidStored)
	}
	return []string{converted}, nil
}

func xrayShadowsocksOutbound(entry Entry, names *UniqueNames) ([]string, *Exclusion) {
	profile, exclusion := structuredProfile(entry, "xray-json")
	if exclusion != nil {
		return nil, exclusion
	}
	data := profile.Data.(profiles.ShadowsocksData)
	if data.Plugin != nil {
		return nil, entry.exclude("shadowsocks", "xray-json", ReasonPlugin)
	}
	if !supportsShadowsocksMethod(xrayShadowsocksMethods, data.Method) {
		return nil, entry.exclude("shadowsocks", "xray-json", ReasonClientVersion)
	}
	tag := profileName(entry, profile, names)
	payload, err := json.Marshal(map[string]any{"outbounds": []any{map[string]any{
		"tag":      tag,
		"protocol": "shadowsocks",
		"settings": map[string]any{
			"address":  profile.Server,
			"port":     portNumber(profile.Port),
			"method":   data.Method,
			"password": data.Password.Reveal(),
		},
	}}})
	if err != nil {
		return nil, entry.exclude("shadowsocks", "xray-json", ReasonSerialization)
	}
	return []string{string(payload)}, nil
}

func xrayHysteria2Outbound(entry Entry, names *UniqueNames) ([]string, *Exclusion) {
	profile, exclusion := structuredProfile(entry, "xray-json")
	if exclusion != nil {
		return nil, exclusion
	}
	data := profile.Data.(profiles.Hysteria2Data)
	if data.ObfuscationType == "gecko" {
		// Xray enables Gecko through FinalMask's packetSize. The Hysteria URI
		// model does not carry that required value, so guessing one would
		// change wire behavior.
		return nil, entry.exclude("hysteria2", "xray-json", ReasonUnrepresentable)
	}
	if data.Insecure {
		// Xray v26.3.27 and v26.7.28 reject the removed allowInsecure
		// setting. A URI that disables certificate verification cannot be
		// safely translated into a certificate pin or peer-name constraint.
		return nil, entry.exclude("hysteria2", "xray-json", ReasonUnrepresentable)
	}
	payload, err := json.Marshal(map[string]any{"outbounds": []any{map[string]any{
		"tag":      profileName(entry, profile, names),
		"protocol": "hysteria",
		"settings": map[string]any{
			"version": 2,
			"address": profile.Server,
			"port":    portNumber(profile.Port),
		},
		"streamSettings": xrayHysteria2Stream(profile, data),
	}}})
	if err != nil {
		return nil, entry.exclude("hysteria2", "xray-json", ReasonSerialization)
	}
	return []string{string(payload)}, nil
}

func xrayHysteria2Stream(profile *profiles.Profile, data profiles.Hysteria2Data) map[string]any {
	stream := map[string]any{
		"method":   "hysteria",
		"security": "tls",
		"hysteriaSettings": map[string]any{
			"version": 2,
			"auth":    data.Authentication.Reveal(),
		},
	}
	tlsSettings := map[string]any{}
	setIfNotEmpty(tlsSettings, "serverName", data.SNI)
	setIfNotEmpty(tlsSettings, "pinnedPeerCertSha256", data.CertificateSHA256)
	if len(tlsSettings) > 0 {
		stream["tlsSettings"] = tlsSettings
	}
	finalMask := map[string]any{}
	if profile.Port.Kind == profiles.PortExpression {
		finalMask["quicParams"] = map[string]any{"udpHop": map[string]any{
			"ports": profile.Port.Expression,
		}}
	}
	if data.ObfuscationType == "salamander" {
		finalMask["udp"] = []any{map[string]any{
			"type": "salamander",
			"settings": map[string]any{
				"password": data.ObfuscationPassword.Reveal(),
			},
		}}
	}
	if len(finalMask) > 0 {
		stream["finalmask"] = finalMask
	}
	return stream
}

func xrayTUICExclusion(entry Entry) *Exclusion {
	profile, err := profiles.Parse(entry.Raw)
	protocol := entry.protocolName()
	if err == nil {
		protocol = string(profile.Protocol)
		if data, ok := profile.Data.(profiles.TUICData); ok && data.Generation == 4 {
			return entry.exclude(protocol, "xray-json", ReasonCompatibility)
		}
	}
	return entry.exclude(protocol, "xray-json", ReasonUnsupportedProtocol)
}

func xrayNormalizedStored(entry Entry) ([]string, *Exclusion) {
	if !json.Valid([]byte(entry.Raw)) {
		return nil, entry.exclude("xray-json", "xray-json", ReasonInvalidStored)
	}
	var value any
	if err := json.Unmarshal([]byte(entry.Raw), &value); err != nil {
		return nil, entry.exclude("xray-json", "xray-json", ReasonInvalidStored)
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return nil, entry.exclude("xray-json", "xray-json", ReasonSerialization)
	}
	return []string{string(normalized)}, nil
}

func NormalizeForOutput(raw string, format string, fallbackRemark string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("configuration is empty")
	}
	if strings.TrimSpace(format) != "xray-json" {
		return trimmed, nil
	}

	scheme := profileconfig.SupportedConfigScheme(trimmed)
	switch scheme {
	case "xray-json":
		var parsed any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return "", fmt.Errorf("invalid XRAY-JSON syntax")
		}
		normalized, err := json.Marshal(parsed)
		if err != nil {
			return "", err
		}
		return string(normalized), nil
	case "vless", "vmess", "trojan":
		return profileconfig.BuildXrayJSONFromLink(trimmed, fallbackRemark)
	default:
		return "", fmt.Errorf("unsupported configuration scheme")
	}
}
