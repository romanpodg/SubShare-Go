package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"xary-sub/internal/vless"
)

type deviceMeta struct {
	DeviceName  string
	DeviceModel string
	Platform    string
	OSVersion   string
	AppName     string
	AppVersion  string
	UserAgent   string
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
		Platform:    clampString(platform, 64),
		OSVersion:   clampString(requestValue(r, []string{"os_version"}, []string{"X-OS-Version"}), 64),
		AppName:     clampString(requestValue(r, []string{"app_name"}, []string{"X-App-Name"}), 64),
		AppVersion:  clampString(requestValue(r, []string{"app_version"}, []string{"X-App-Version"}), 64),
		UserAgent:   clampString(strings.TrimSpace(r.UserAgent()), 255),
	}
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

func nullInt64Value(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func (a *App) checkAndPersistKey(keyID int64, rawURL string) error {
	status, checkErr, latency := vless.CheckVLESSAvailability(rawURL)
	_, err := a.db.Exec(
		`UPDATE vless_keys SET check_status = ?, check_error = ?, last_latency_ms = ?, last_checked_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status,
		nullStringValue(checkErr),
		nullInt64Value(latency),
		keyID,
	)
	return err
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
