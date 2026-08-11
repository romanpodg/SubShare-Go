package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
)

type vmessConfigPayload = profileconfig.VMessConfigPayload
type linkConfigurationDraft = profileconfig.LinkConfigurationDraft

func supportedConfigScheme(raw string) string {
	return profileconfig.SupportedConfigScheme(raw)
}

func validateRealConfigURL(raw string) error {
	return profileconfig.ValidateRealConfigURL(raw)
}

func parseConfigTarget(raw string) (host, port string, err error) {
	return profileconfig.ParseConfigTarget(raw)
}

func asObject(value any) (map[string]any, bool) {
	return profileconfig.AsObject(value)
}

func asArray(value any) []any {
	return profileconfig.AsArray(value)
}

func anyToString(value any) string {
	return profileconfig.AnyToString(value)
}

func anyToPort(value any) string {
	return profileconfig.AnyToPort(value)
}

func parseXrayJSONTarget(raw string) (host, port string, err error) {
	return profileconfig.ParseXrayJSONTarget(raw)
}

func parseXrayJSONDrafts(raw string) ([]linkConfigurationDraft, error) {
	return profileconfig.ParseXrayJSONDrafts(raw)
}

func projectXrayJSONDrafts(raw string) ([]linkConfigurationDraft, int, error) {
	return profileconfig.ProjectXrayJSONDrafts(raw)
}

func validateXrayJSONConfiguration(raw string) error {
	return profileconfig.ValidateXrayJSONConfiguration(raw)
}

func decodeVMESSPayload(encoded string) (vmessConfigPayload, error) {
	return profileconfig.DecodeVMESSPayload(encoded)
}

func decodeVMESSPayloadMap(encoded string) (map[string]any, error) {
	return profileconfig.DecodeVMESSPayloadMap(encoded)
}

func normalizeXrayNetwork(raw string) string {
	return profileconfig.NormalizeXrayNetwork(raw)
}

func normalizeXraySecurity(raw string) string {
	return profileconfig.NormalizeXraySecurity(raw)
}

func parseCommaSeparatedValues(raw string) []string {
	return profileconfig.ParseCommaSeparatedValues(raw)
}

func parsePortNumber(raw string) int {
	return profileconfig.ParsePortNumber(raw)
}

func decodeBase64String(raw string) string {
	return profileconfig.DecodeBase64String(raw)
}

func decodeQueryComponent(raw string) string {
	return profileconfig.DecodeQueryComponent(raw)
}

func parseFragmentMetadata(rawFragment string) (string, string) {
	return profileconfig.ParseFragmentMetadata(rawFragment)
}

func parseBooleanFlag(raw string) bool {
	return profileconfig.ParseBooleanFlag(raw)
}

func normalizeHeaderType(raw string) string {
	return profileconfig.NormalizeHeaderType(raw)
}

func buildRawHeaderSettings(headerType string, host string, path string) map[string]any {
	return profileconfig.BuildRawHeaderSettings(headerType, host, path)
}

func recoverLegacyPlusValue(raw string) string {
	return profileconfig.RecoverLegacyPlusValue(raw)
}

func parseLinkConfiguration(raw string) (linkConfigurationDraft, error) {
	return profileconfig.ParseLinkConfiguration(raw)
}

func buildShareLinkFromDraft(draft linkConfigurationDraft) (string, error) {
	return profileconfig.BuildShareLinkFromDraft(draft)
}

func buildXrayJSONFromLink(raw string, fallbackRemark string) (string, error) {
	return profileconfig.BuildXrayJSONFromLink(raw, fallbackRemark)
}

func normalizeConfigurationForSubscriptionOutput(raw string, format string, fallbackRemark string) (string, error) {
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

func checkConfigurationAvailability(raw string) (string, string, int64) {
	return profileconfig.CheckConfigurationAvailabilityWithResolver(raw, resolveExternalHost)
}

func checkConfigurationAvailabilityContext(ctx context.Context, raw string) (string, string, int64) {
	return profileconfig.CheckConfigurationAvailabilityContext(ctx, raw, resolveExternalHost)
}
