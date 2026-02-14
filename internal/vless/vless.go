package vless

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// ParseVLESSParts extracts the UUID, host, port, query, and fragment from a VLESS URL.
func ParseVLESSParts(raw string) (uuid, host, port, query, fragment string, err error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", "", "", "", err
	}
	if strings.ToLower(parsed.Scheme) != "vless" {
		return "", "", "", "", "", fmt.Errorf("scheme is not vless")
	}

	if parsed.User != nil {
		uuid = strings.TrimSpace(parsed.User.Username())
	}
	host = strings.TrimSpace(parsed.Hostname())
	port = strings.TrimSpace(parsed.Port())
	if port == "" {
		port = "443"
	}
	query = strings.TrimSpace(parsed.RawQuery)
	fragment = strings.TrimSpace(parsed.Fragment)
	return uuid, host, port, query, fragment, nil
}

// BuildVLESSURL constructs a VLESS URL from individual parts.
func BuildVLESSURL(uuid, host, port, query, fragment string) (string, error) {
	uuid = strings.TrimSpace(uuid)
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	query = strings.TrimSpace(query)
	fragment = strings.TrimSpace(fragment)

	if uuid == "" {
		return "", fmt.Errorf("uuid is required")
	}
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	if port == "" {
		port = "443"
	}

	built := &url.URL{
		Scheme:   "vless",
		User:     url.User(uuid),
		Host:     net.JoinHostPort(host, port),
		RawQuery: query,
		Fragment: fragment,
	}
	return built.String(), nil
}

// IsPrivateIP returns true if the given IP is in a private or reserved range.
func IsPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		network string
	}{
		{"10.0.0.0/8"},
		{"172.16.0.0/12"},
		{"192.168.0.0/16"},
		{"127.0.0.0/8"},
		{"169.254.0.0/16"},
		{"::1/128"},
		{"fc00::/7"},
		{"fe80::/10"},
	}
	for _, r := range privateRanges {
		_, cidr, err := net.ParseCIDR(r.network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// CheckVLESSAvailability performs a health check on a VLESS URL by establishing a TCP connection.
// It returns the check status ("up" or "down"), an error message (empty on success), and latency in milliseconds.
func CheckVLESSAvailability(raw string) (string, string, int64) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "down", "invalid URL", 0
	}
	if strings.ToLower(parsed.Scheme) != "vless" {
		return "down", "scheme is not vless", 0
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "down", "missing host", 0
	}

	port := strings.TrimSpace(parsed.Port())
	if port == "" {
		port = "443"
	}

	// SSRF protection: resolve hostname and block private/reserved addresses
	ips, err := net.LookupHost(host)
	if err != nil {
		return "down", fmt.Sprintf("DNS lookup failed: %s", err.Error()), 0
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if IsPrivateIP(ip) {
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

// TruncateMiddle truncates a string in the middle if it exceeds maxLen,
// inserting an ellipsis character.
func TruncateMiddle(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	head := (maxLen - 1) / 2
	tail := maxLen - head - 1
	if tail < 1 {
		tail = 1
	}
	return value[:head] + "\u2026" + value[len(value)-tail:]
}
