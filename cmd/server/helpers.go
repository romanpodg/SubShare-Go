package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/middleware"
)

type deviceMeta struct {
	NormalizedHWID string
	DeviceName     string
	DeviceModel    string
	DeviceBrand    string
	Platform       string
	OSVersion      string
	AppName        string
	AppVersion     string
	UserAgent      string
}

func requestValue(r *http.Request, queryKeys []string, headerKeys []string) string {
	for _, key := range queryKeys {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			return value
		}
	}
	for _, key := range headerKeys {
		if value := strings.TrimSpace(r.Header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func clampString(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if maxLen <= 0 {
		return ""
	}
	if len(value) > maxLen {
		return value[:maxLen]
	}
	return value
}

func extractDeviceMeta(r *http.Request) deviceMeta {
	platform := requestValue(r, []string{"platform"}, []string{"X-Device-Platform", "Sec-CH-UA-Platform"})
	platform = strings.Trim(platform, `"`)

	return deviceMeta{
		DeviceName:  clampString(requestValue(r, []string{"device_name", "device"}, []string{"X-Device-Name"}), 128),
		DeviceModel: clampString(requestValue(r, []string{"device_model", "model"}, []string{"X-Device-Model"}), 128),
		Platform:    clampString(firstNonEmpty(platform, requestValue(r, []string{"device_os", "os"}, []string{"X-Device-OS", "X-Ver-OS", "X-OS"})), 64),
		OSVersion:   clampString(requestValue(r, []string{"os_version", "ver_os"}, []string{"X-OS-Version", "X-Ver-OS-Version"}), 64),
		AppName:     clampString(requestValue(r, []string{"app_name"}, []string{"X-App-Name"}), 64),
		AppVersion:  clampString(requestValue(r, []string{"app_version"}, []string{"X-App-Version"}), 64),
		UserAgent:   clampString(strings.TrimSpace(r.UserAgent()), 255),
	}
}

func mergeDeviceMeta(meta deviceMeta, parsed ParsedDeviceInfo) deviceMeta {
	meta.NormalizedHWID = clampString(firstNonEmpty(meta.NormalizedHWID, parsed.NormalizedID), 128)
	meta.Platform = clampString(firstNonEmpty(meta.Platform, parsed.Platform), 64)
	meta.OSVersion = clampString(firstNonEmpty(meta.OSVersion, parsed.OSVersion), 64)
	meta.DeviceModel = clampString(firstNonEmpty(meta.DeviceModel, parsed.DeviceModel), 128)
	meta.DeviceBrand = clampString(firstNonEmpty(meta.DeviceBrand, parsed.DeviceBrand), 64)
	meta.AppName = clampString(firstNonEmpty(meta.AppName, parsed.ClientApp), 64)
	meta.AppVersion = clampString(firstNonEmpty(meta.AppVersion, parsed.ClientVersion), 64)

	if strings.TrimSpace(meta.DeviceName) == "" {
		meta.DeviceName = clampString(firstNonEmpty(meta.DeviceBrand+" "+meta.DeviceModel, meta.DeviceModel), 128)
	}

	return meta
}

func parseOptionalDateTimeLocal(raw string) (sql.NullTime, error) {
	return parseOptionalDateTimeInLocation(raw, time.Local)
}

func parseOptionalDateTimeInLocation(raw string, location *time.Location) (sql.NullTime, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return sql.NullTime{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return sql.NullTime{Time: parsed.UTC(), Valid: true}, nil
	}
	if location == nil {
		location = time.UTC
	}

	layouts := []string{
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"02/01/2006 15:04",
	}

	var lastErr error
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, raw, location)
		if err == nil {
			return sql.NullTime{Time: t.UTC(), Valid: true}, nil
		}
		lastErr = err
	}

	return sql.NullTime{}, lastErr
}

func formatDateTimeInputInLocation(value sql.NullTime, location *time.Location) string {
	if !value.Valid {
		return ""
	}
	if location == nil {
		location = time.UTC
	}
	local := value.Time.In(location)
	return local.Format("02/01/2006 15:04")
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

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (a *App) checkAndPersistKey(ctx context.Context, keyID int64, rawURL string) error {
	status, checkErr, latency := checkConfigurationAvailabilityContext(ctx, rawURL)
	return a.keyService().SaveHealthCheckResult(ctx, keyID, status, checkErr, latency)
}

func (a *App) resolveBaseURL(r *http.Request) string {
	if a.baseURL != "" {
		return a.baseURL
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Host)
	if middleware.TrustedProxy(r) {
		if forwardedScheme := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); forwardedScheme == "http" || forwardedScheme == "https" {
			scheme = forwardedScheme
		}
		if forwardedHost := firstForwardedValue(r.Header.Get("X-Forwarded-Host")); validOriginHost(forwardedHost) {
			host = forwardedHost
		}
	}
	if !validOriginHost(host) {
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

func firstForwardedValue(value string) string {
	value, _, _ = strings.Cut(value, ",")
	return strings.ToLower(strings.TrimSpace(value))
}

func validOriginHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "" || strings.ContainsAny(host, "/\\@?#\r\n\t ") {
		return false
	}
	parsed, err := url.Parse("http://" + host)
	return err == nil && parsed.Host == host && parsed.Hostname() != ""
}
