package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/middleware"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
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

func formatDateTimeInput(value sql.NullTime) string {
	return formatDateTimeInputInLocation(value, time.Local)
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

func nullInt64Value(value int64) any {
	if value <= 0 {
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

func containsLegacySubscriptionBodyMarkers(body string) bool {
	legacyMarkers := []string{
		"#profile-desc:",
		"#profile-status:",
		"#description:",
		"#happ-provider-id:",
		"#happ-no-limit-mode:",
		"#happ-no-limit-mode-xhttp-only:",
		"#happ-mandatory-hwid:",
		"#happ-notify-expiration:",
		"#happ-hide-server-settings:",
	}
	for _, marker := range legacyMarkers {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
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

type subscriptionTemplateData struct {
	UserName       string
	Telegram       string
	SubscriptionID string
	ExpiryDate     string
	ExpiryDateTime string
	RealKeysCount  int
}

func (a *App) buildSubscriptionTemplateData(subscriptionID string, subscriptionFormat string) (subscriptionTemplateData, error) {
	out := subscriptionTemplateData{
		SubscriptionID: strings.TrimSpace(subscriptionID),
	}
	format, ok := model.NormalizeSubscriptionFormat(subscriptionFormat)
	if !ok {
		format = model.SubscriptionFormatLinks
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

	rows, err := a.db.Query(
		`SELECT k.id, s.encrypted_url
		 FROM users u
		 JOIN user_keys uk ON uk.user_id = u.id
		 JOIN vless_keys k ON k.id = uk.key_id
		 LEFT JOIN vless_key_secrets s ON k.id = s.vless_key_id
		 WHERE u.subscription_id = ?
		   AND k.status = 'active'
		   AND k.key_kind = 'real'
		   AND COALESCE(k.health_failure_count, 0) < 3`,
		subscriptionID,
	)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	realCount := 0
	for rows.Next() {
		var id int64
		var encURL sql.NullString
		if err := rows.Scan(&id, &encURL); err != nil {
			return out, err
		}
		if !encURL.Valid || encURL.String == "" {
			continue
		}
		sec, err := profilestorage.Decrypt(encURL.String, a.profileKeyring, id)
		if err != nil {
			continue
		}
		rawURL := sec.Reveal()
		if format == model.SubscriptionFormatLinks && profileconfig.SupportedConfigScheme(rawURL) == model.SubscriptionFormatXrayJSON {
			continue
		}
		realCount++
	}
	if err := rows.Err(); err != nil {
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

func buildInformationalXrayJSON(displayText string) string {
	displayText = strings.TrimSpace(displayText)
	if displayText == "" {
		displayText = "Info"
	}

	description := displayText
	if newline := strings.Index(description, "\n"); newline >= 0 {
		description = strings.TrimSpace(description[:newline])
	}
	if description == "" {
		description = "Informational key"
	}

	payload := map[string]any{
		"remarks": displayText,
		"meta": map[string]any{
			"serverDescription": description,
			"informational":     true,
		},
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []any{},
		"outbounds": []any{
			map[string]any{
				"tag":      "proxy",
				"protocol": "vless",
				"settings": map[string]any{
					"vnext": []any{
						map[string]any{
							"address": "info.invalid",
							"port":    443,
							"users": []any{
								map[string]any{
									"id":         "00000000-0000-0000-0000-000000000000",
									"encryption": "none",
								},
							},
						},
					},
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "none",
				},
			},
		},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}
