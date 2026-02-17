package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xary-sub/internal/vless"
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
	local := value.Time.Local()
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

type subscriptionTemplateData struct {
	UserName       string
	Telegram       string
	SubscriptionID string
	ExpiryDate     string
	ExpiryDateTime string
	RealKeysCount  int
}

func (a *App) buildSubscriptionTemplateData(subscriptionID string) (subscriptionTemplateData, error) {
	out := subscriptionTemplateData{
		SubscriptionID: strings.TrimSpace(subscriptionID),
	}

	var name sql.NullString
	var email sql.NullString
	var expiresAt sql.NullTime
	err := a.db.QueryRow(
		`SELECT name, email, expires_at FROM users WHERE subscription_id = ?`,
		subscriptionID,
	).Scan(&name, &email, &expiresAt)
	if err != nil {
		return out, err
	}

	out.UserName = strings.TrimSpace(name.String)
	telegram := strings.TrimPrefix(strings.TrimSpace(email.String), "@")
	out.Telegram = telegram
	if expiresAt.Valid {
		local := expiresAt.Time.Local()
		out.ExpiryDate = local.Format("02/01/2006")
		out.ExpiryDateTime = local.Format("02/01/2006 15:04")
	}

	var realCount int
	if err := a.db.QueryRow(
		`SELECT COUNT(1)
		 FROM users u
		 JOIN user_keys uk ON uk.user_id = u.id
		 JOIN vless_keys k ON k.id = uk.key_id
		 WHERE u.subscription_id = ?
		   AND k.status = 'active'
		   AND k.key_kind = 'real'
		   AND k.check_status != 'down'`,
		subscriptionID,
	).Scan(&realCount); err != nil {
		return out, err
	}
	out.RealKeysCount = realCount

	return out, nil
}

func renderInfoTemplate(template string, data subscriptionTemplateData) string {
	text := strings.TrimSpace(template)
	if text == "" {
		return ""
	}
	replacements := map[string]string{
		"{user_name}":       data.UserName,
		"{telegram}":        data.Telegram,
		"{subscription_id}": data.SubscriptionID,
		"{expires_date}":    data.ExpiryDate,
		"{expires_at}":      data.ExpiryDateTime,
		"{real_keys_count}": fmt.Sprintf("%d", data.RealKeysCount),
	}
	for key, value := range replacements {
		text = strings.ReplaceAll(text, key, strings.TrimSpace(value))
	}
	return strings.TrimSpace(text)
}

func buildInformationalVLESSURL(displayText string) string {
	displayText = strings.TrimSpace(displayText)
	if displayText == "" {
		displayText = "Info"
	}
	return "vless://00000000-0000-0000-0000-000000000000@info.invalid:443?type=tcp&security=none#" + url.QueryEscape(displayText)
}
