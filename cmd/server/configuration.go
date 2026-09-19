package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
)

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
