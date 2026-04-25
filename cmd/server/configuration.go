package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"xary-sub/internal/vless"
)

type vmessConfigPayload struct {
	Address string `json:"add"`
	Port    string `json:"port"`
	ID      string `json:"id"`
	Name    string `json:"ps"`
}

func supportedConfigScheme(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "vmess://") {
		return "vmess"
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parsed.Scheme))
}

func validateRealConfigURL(raw string) error {
	switch supportedConfigScheme(raw) {
	case "vless", "vmess", "trojan":
		return nil
	case "":
		return fmt.Errorf("configuration is empty or invalid")
	default:
		return fmt.Errorf("configuration must start with vless://, vmess:// or trojan://")
	}
}

func parseConfigTarget(raw string) (host, port string, err error) {
	trimmed := strings.TrimSpace(raw)
	switch supportedConfigScheme(trimmed) {
	case "vless", "trojan":
		parsed, parseErr := url.Parse(trimmed)
		if parseErr != nil {
			return "", "", parseErr
		}
		host = strings.TrimSpace(parsed.Hostname())
		port = strings.TrimSpace(parsed.Port())
	case "vmess":
		payloadRaw := strings.TrimSpace(strings.TrimPrefix(trimmed, "vmess://"))
		payload, parseErr := decodeVMESSPayload(payloadRaw)
		if parseErr != nil {
			return "", "", parseErr
		}
		host = strings.TrimSpace(payload.Address)
		port = strings.TrimSpace(payload.Port)
	default:
		return "", "", fmt.Errorf("unsupported configuration scheme")
	}

	if host == "" {
		return "", "", fmt.Errorf("missing host")
	}
	if port == "" {
		port = "443"
	}
	return host, port, nil
}

func decodeVMESSPayload(encoded string) (vmessConfigPayload, error) {
	normalized := strings.TrimSpace(encoded)
	normalized = strings.ReplaceAll(normalized, "-", "+")
	normalized = strings.ReplaceAll(normalized, "_", "/")
	if mod := len(normalized) % 4; mod != 0 {
		normalized += strings.Repeat("=", 4-mod)
	}

	decoded, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		return vmessConfigPayload{}, err
	}

	var payload vmessConfigPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return vmessConfigPayload{}, err
	}
	return payload, nil
}

func checkConfigurationAvailability(raw string) (string, string, int64) {
	host, port, err := parseConfigTarget(raw)
	if err != nil {
		return "down", "invalid configuration", 0
	}

	ips, err := net.LookupHost(host)
	if err != nil {
		return "down", fmt.Sprintf("DNS lookup failed: %s", err.Error()), 0
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if vless.IsPrivateIP(ip) {
			return "down", "health check to private addresses is not allowed", 0
		}
	}

	address := net.JoinHostPort(host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", address, 4*time.Second)
	if err != nil {
		return "down", err.Error(), 0
	}
	_ = conn.Close()

	latency := time.Since(start).Milliseconds()
	if latency < 0 {
		latency = 0
	}
	return "up", "", latency
}
