package main

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultDeviceLimitMessage = "You have reached the maximum number of allowed devices for your subscription"

func normalizeUserStatus(raw string) (string, bool) {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		status = userStatusActive
	}
	switch status {
	case userStatusActive, userStatusPaused, userStatusBlocked:
		return status, true
	default:
		return "", false
	}
}

func normalizeStoredStatus(raw string) string {
	status, ok := normalizeUserStatus(raw)
	if !ok {
		return userStatusActive
	}
	return status
}

func normalizeKeyStatus(raw string) (string, bool) {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		status = keyStatusActive
	}
	if status == "blocked" {
		status = keyStatusNonActive
	}
	switch status {
	case keyStatusActive, keyStatusNonActive:
		return status, true
	default:
		return "", false
	}
}

func keyStatusLabel(status string) string {
	switch status {
	case keyStatusActive:
		return "active"
	case keyStatusNonActive:
		return "non-active"
	default:
		return "non-active"
	}
}

func truncateMiddle(value string, maxLen int) string {
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
	return value[:head] + "…" + value[len(value)-tail:]
}

func parseVLESSParts(raw string) (uuid, host, port, query, fragment string, err error) {
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

func buildVLESSURL(uuid, host, port, query, fragment string) (string, error) {
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

func parseOptionalDateTimeLocal(raw string) (sql.NullTime, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sql.NullTime{}, nil
	}
	t, err := time.ParseInLocation("2006-01-02T15:04", raw, time.Local)
	if err != nil {
		return sql.NullTime{}, err
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}, nil
}

func formatDateTimeInput(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Local().Format("2006-01-02T15:04")
}

func nullTimeValue(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.UTC()
}

func nullStringValue(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func normalizeCheckStatus(raw string) string {
	status := strings.ToLower(strings.TrimSpace(raw))
	switch status {
	case "up", "down", "unknown":
		return status
	default:
		return "unknown"
	}
}

func checkStatusLabel(status string) string {
	switch status {
	case "up":
		return "Доступен"
	case "down":
		return "Недоступен"
	default:
		return "Не проверен"
	}
}

func (a *App) checkAndPersistKey(keyID int64, rawURL string) error {
	status, checkErr, latency := checkVLESSAvailability(rawURL)
	_, err := a.db.Exec(
		`UPDATE vless_keys SET check_status = ?, check_error = ?, last_latency_ms = ?, last_checked_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status,
		nullStringValue(checkErr),
		nullInt64Value(latency),
		keyID,
	)
	return err
}

func isPrivateIP(ip net.IP) bool {
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

func checkVLESSAvailability(raw string) (string, string, int64) {
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
		if isPrivateIP(ip) {
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

func nullInt64Value(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func (a *App) resolveBaseURL(r *http.Request) string {
	if a.baseURL != "" {
		return a.baseURL
	}

	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		host = "localhost:8080"
	}

	if strings.HasSuffix(host, ":80") && scheme == "http" {
		host = strings.TrimSuffix(host, ":80")
	}
	if strings.HasSuffix(host, ":443") && scheme == "https" {
		host = strings.TrimSuffix(host, ":443")
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}
