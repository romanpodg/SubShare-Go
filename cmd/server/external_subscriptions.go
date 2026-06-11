package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"subshare/internal/model"
	"subshare/internal/vless"
)

const (
	maxExternalSubscriptionBodyBytes = 10 << 20 // 10 MB
	maxExternalImportItems           = 10000
)

type externalSubscriptionMetadata struct {
	Title           string
	RefreshHours    int
	SupportURL      string
	WebPageURL      string
	Announce        string
	ContentType     string
	ContentDisp     string
	SourceFinalURL  string
	HTTPStatusCode  int
	HTTPStatusLabel string
}

type externalParsedKey struct {
	Label  string
	URL    string
	Scheme string
	Ref    string
}

type externalSubscriptionParseResult struct {
	DetectedFormat string
	Metadata       externalSubscriptionMetadata
	Keys           []externalParsedKey
	Warnings       []string
}

type externalSourceRow struct {
	ID                  int64
	Name                string
	Category            string
	KeyCategory         string
	KeyInsertMode       string
	SourceURL           string
	Enabled             bool
	ApplyRemoteMetadata bool
	PassHWID            bool
	HWIDVersion         string
	HWIDModelName       string
	HWIDValue           string
	LastImportCount     int
	ImportStatus        string
	LastError           string
	LastSyncedAt        sql.NullTime
	MetaTitle           string
	MetaRefreshHours    int
	MetaSupportURL      string
	MetaWebPageURL      string
	MetaAnnounce        string
	CreatedAt           sql.NullTime
	UpdatedAt           sql.NullTime
}

type externalHWIDProfile struct {
	PassHWID  bool
	Version   string
	ModelName string
	HWID      string
}

func normalizeKeyInsertMode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "top", "bottom":
		return value
	default:
		return "bottom"
	}
}

func nonNilWarnings(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func decodeSubscriptionHeaderValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(value), "base64:") {
		decoded := decodeBase64String(value)
		if decoded != "" {
			return strings.TrimSpace(decoded)
		}
		value = strings.TrimSpace(value[7:])
	}
	return strings.TrimSpace(value)
}

func parsePositiveInt(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func trimUTF8BOM(raw string) string {
	return strings.TrimPrefix(raw, "\uFEFF")
}

func maybeDecodeBase64SubscriptionBody(raw string) (string, bool) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return "", false
	}

	compact := strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '\t' || r == ' ' {
			return -1
		}
		return r
	}, candidate)
	if len(compact) < 24 {
		return "", false
	}

	decoded := decodeBase64String(compact)
	decoded = strings.TrimSpace(trimUTF8BOM(decoded))
	if decoded == "" {
		return "", false
	}

	if strings.HasPrefix(decoded, "{") || strings.HasPrefix(decoded, "[") {
		return decoded, true
	}
	if strings.Contains(decoded, "://") || strings.Contains(decoded, "\n") {
		return decoded, true
	}
	return "", false
}

func buildExternalKeyRef(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:16])
}

func extractJSONSubscriptionLabel(root map[string]any, fallback string) string {
	if value, ok := root["remarks"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if value, ok := root["tag"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if meta, ok := root["meta"].(map[string]any); ok {
		if value, ok := meta["serverDescription"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if outbounds, ok := root["outbounds"].([]any); ok && len(outbounds) > 0 {
		if outbound, ok := outbounds[0].(map[string]any); ok {
			if value, ok := outbound["tag"].(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return fallback
}

func parseExternalJSONBody(body string) ([]externalParsedKey, error) {
	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	appendItem := func(items *[]externalParsedKey, obj map[string]any, index int) error {
		encoded, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		raw := strings.TrimSpace(string(encoded))
		if raw == "" || !json.Valid([]byte(raw)) {
			return nil
		}
		label := strings.TrimSpace(extractJSONSubscriptionLabel(obj, fmt.Sprintf("JSON %03d", index+1)))
		if label == "" {
			label = fmt.Sprintf("JSON %03d", index+1)
		}
		*items = append(*items, externalParsedKey{
			Label:  label,
			URL:    raw,
			Scheme: "xray-json",
			Ref:    buildExternalKeyRef(raw),
		})
		return nil
	}

	out := make([]externalParsedKey, 0, 16)
	switch typed := parsed.(type) {
	case []any:
		for index, item := range typed {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if err := appendItem(&out, obj, index); err != nil {
				return nil, err
			}
		}
	case map[string]any:
		if err := appendItem(&out, typed, 0); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("JSON subscription must be an object or array")
	}

	return out, nil
}

func parseExternalLinkBody(body string) ([]externalParsedKey, []string, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	out := make([]externalParsedKey, 0, len(lines))
	warnings := make([]string, 0)

	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}

		scheme := supportedConfigScheme(trimmed)
		switch scheme {
		case "vless", "vmess", "trojan":
			draft, err := parseLinkConfiguration(trimmed)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Строка %d: пропущена (%v)", index+1, err))
				continue
			}
			label := strings.TrimSpace(draft.Remark)
			if label == "" {
				label = fmt.Sprintf("Импорт %03d", len(out)+1)
			}
			out = append(out, externalParsedKey{
				Label:  label,
				URL:    trimmed,
				Scheme: scheme,
				Ref:    buildExternalKeyRef(trimmed),
			})
		case "xray-json":
			var obj map[string]any
			if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
				warnings = append(warnings, fmt.Sprintf("Строка %d: пропущена (некорректный JSON)", index+1))
				continue
			}
			label := extractJSONSubscriptionLabel(obj, fmt.Sprintf("JSON %03d", len(out)+1))
			out = append(out, externalParsedKey{
				Label:  label,
				URL:    trimmed,
				Scheme: "xray-json",
				Ref:    buildExternalKeyRef(trimmed),
			})
		default:
			warnings = append(warnings, fmt.Sprintf("Строка %d: неизвестный формат", index+1))
		}
	}

	return out, warnings, nil
}

func parseExternalSubscriptionBody(raw string) (externalSubscriptionParseResult, error) {
	body := strings.TrimSpace(trimUTF8BOM(raw))
	if body == "" {
		return externalSubscriptionParseResult{}, fmt.Errorf("subscription body is empty")
	}

	result := externalSubscriptionParseResult{
		Warnings: make([]string, 0),
	}
	if decoded, ok := maybeDecodeBase64SubscriptionBody(body); ok {
		body = decoded
		result.Warnings = append(result.Warnings, "Получен base64-текст — выполнено автоматическое декодирование.")
	}

	if json.Valid([]byte(body)) {
		keys, err := parseExternalJSONBody(body)
		if err == nil && len(keys) > 0 {
			result.DetectedFormat = "xray-json"
			result.Keys = keys
			return result, nil
		}
	}

	keys, warnings, err := parseExternalLinkBody(body)
	if err != nil {
		return externalSubscriptionParseResult{}, err
	}
	result.DetectedFormat = "links"
	result.Keys = keys
	result.Warnings = append(result.Warnings, warnings...)
	return result, nil
}

func parseExternalSubscriptionFromRawBody(sourceURL string, rawBody string, metadata externalSubscriptionMetadata) (externalSubscriptionParseResult, error) {
	parsed, err := parseExternalSubscriptionBody(rawBody)
	if err != nil {
		return externalSubscriptionParseResult{}, err
	}
	if len(parsed.Keys) == 0 {
		return externalSubscriptionParseResult{}, fmt.Errorf("no supported keys found in source response")
	}
	if len(parsed.Keys) > maxExternalImportItems {
		return externalSubscriptionParseResult{}, fmt.Errorf("too many keys in source response (max %d)", maxExternalImportItems)
	}
	if strings.TrimSpace(metadata.SourceFinalURL) == "" {
		metadata.SourceFinalURL = strings.TrimSpace(sourceURL)
	}
	parsed.Metadata = metadata
	return parsed, nil
}

func validateExternalSourceURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("source_url is required")
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid source_url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("source_url must use http:// or https://")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return "", fmt.Errorf("source_url must contain host")
	}
	return parsed.String(), nil
}

func normalizeExternalSourceCategory(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "Общее"
	}
	if len(value) > 24 {
		value = value[:24]
	}
	return value
}

func (a *App) upsertExternalSourceCategory(category string) error {
	category = normalizeExternalSourceCategory(category)
	_, err := a.db.Exec(
		`INSERT INTO external_source_categories(name, updated_at)
		 VALUES(?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		category,
	)
	return err
}

func clampExternalHWIDField(raw string, max int) string {
	value := strings.TrimSpace(raw)
	if max <= 0 || value == "" {
		return value
	}
	if len(value) > max {
		value = value[:max]
	}
	return value
}

func normalizeExternalHWIDProfile(pass bool, version, modelName, hwid string) externalHWIDProfile {
	if !pass {
		return externalHWIDProfile{}
	}
	return externalHWIDProfile{
		PassHWID:  true,
		Version:   clampExternalHWIDField(version, 64),
		ModelName: clampExternalHWIDField(modelName, 128),
		HWID:      clampExternalHWIDField(hwid, 128),
	}
}

func applyExternalHWIDHeaders(req *http.Request, profile externalHWIDProfile) {
	if !profile.PassHWID {
		req.Header.Set("User-Agent", "subshare/1.0 (+external-import)")
		return
	}

	userAgent := "subshare/1.0 (+external-import)"
	if profile.Version != "" {
		if profile.ModelName != "" {
			userAgent = fmt.Sprintf("Happ/%s (%s)", profile.Version, profile.ModelName)
		} else {
			userAgent = fmt.Sprintf("Happ/%s", profile.Version)
		}
	}
	req.Header.Set("User-Agent", userAgent)

	if profile.HWID != "" {
		req.Header.Set("X-HWID", profile.HWID)
		req.Header.Set("X-Device-ID", profile.HWID)
	}
	if profile.ModelName != "" {
		req.Header.Set("X-Device-Model", profile.ModelName)
	}
	if profile.Version != "" {
		req.Header.Set("X-App-Version", profile.Version)
		req.Header.Set("X-Client-Version", profile.Version)
	}
	if profile.ModelName != "" || profile.Version != "" {
		req.Header.Set("X-Device-Info", strings.TrimSpace(fmt.Sprintf("model=%s;version=%s", profile.ModelName, profile.Version)))
	}
}

func suggestExternalSourceName(sourceURL string, meta externalSubscriptionMetadata) string {
	if strings.TrimSpace(meta.Title) != "" {
		return strings.TrimSpace(meta.Title)
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "Сторонняя подписка"
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "Сторонняя подписка"
	}
	return host
}

func fetchExternalSubscription(sourceURL string, hwidProfile externalHWIDProfile) (externalSubscriptionParseResult, error) {
	finalURL, err := validateExternalSourceURL(sourceURL)
	if err != nil {
		return externalSubscriptionParseResult{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finalURL, nil)
	if err != nil {
		return externalSubscriptionParseResult{}, fmt.Errorf("failed to prepare request: %w", err)
	}
	applyExternalHWIDHeaders(req, hwidProfile)
	req.Header.Set("Accept", "application/json,text/plain,*/*")

	client := &http.Client{
		Timeout: 20 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return externalSubscriptionParseResult{}, fmt.Errorf("failed to fetch source: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxExternalSubscriptionBodyBytes+1)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return externalSubscriptionParseResult{}, fmt.Errorf("failed to read source response: %w", err)
	}
	if len(bodyBytes) > maxExternalSubscriptionBodyBytes {
		return externalSubscriptionParseResult{}, fmt.Errorf("source response is too large (max 10 MB)")
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return externalSubscriptionParseResult{}, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}

	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		if resp.Header.Get("profile-title") != "" || resp.Header.Get("announce") != "" {
			return externalSubscriptionParseResult{}, fmt.Errorf("source returned subscription headers but empty body")
		}
		return externalSubscriptionParseResult{}, fmt.Errorf("subscription body is empty")
	}

	parsed, err := parseExternalSubscriptionBody(string(bodyBytes))
	if err != nil {
		return externalSubscriptionParseResult{}, err
	}
	if len(parsed.Keys) == 0 {
		return externalSubscriptionParseResult{}, fmt.Errorf("no supported keys found in source response")
	}
	if len(parsed.Keys) > maxExternalImportItems {
		return externalSubscriptionParseResult{}, fmt.Errorf("too many keys in source response (max %d)", maxExternalImportItems)
	}

	parsed.Metadata = externalSubscriptionMetadata{
		Title:           decodeSubscriptionHeaderValue(resp.Header.Get("profile-title")),
		RefreshHours:    parsePositiveInt(resp.Header.Get("profile-update-interval")),
		SupportURL:      strings.TrimSpace(resp.Header.Get("support-url")),
		WebPageURL:      strings.TrimSpace(resp.Header.Get("profile-web-page-url")),
		Announce:        decodeSubscriptionHeaderValue(resp.Header.Get("announce")),
		ContentType:     strings.TrimSpace(resp.Header.Get("content-type")),
		ContentDisp:     strings.TrimSpace(resp.Header.Get("content-disposition")),
		SourceFinalURL:  strings.TrimSpace(resp.Request.URL.String()),
		HTTPStatusCode:  resp.StatusCode,
		HTTPStatusLabel: strings.TrimSpace(resp.Status),
	}
	return parsed, nil
}

func formatNullableTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Local().Format("2006-01-02 15:04:05")
}

func sourceRowToModel(row externalSourceRow) model.ExternalSubscriptionSource {
	return model.ExternalSubscriptionSource{
		ID:                  row.ID,
		Name:                row.Name,
		Category:            row.Category,
		KeyCategory:         row.KeyCategory,
		KeyInsertMode:       row.KeyInsertMode,
		SourceURL:           row.SourceURL,
		Enabled:             row.Enabled,
		ApplyRemoteMetadata: row.ApplyRemoteMetadata,
		PassHWID:            row.PassHWID,
		HWIDVersion:         row.HWIDVersion,
		HWIDModelName:       row.HWIDModelName,
		HWIDValue:           row.HWIDValue,
		LastImportCount:     row.LastImportCount,
		ImportStatus:        row.ImportStatus,
		LastError:           row.LastError,
		LastSyncedAt:        formatNullableTime(row.LastSyncedAt),
		MetaTitle:           row.MetaTitle,
		MetaRefreshHours:    row.MetaRefreshHours,
		MetaSupportURL:      row.MetaSupportURL,
		MetaWebPageURL:      row.MetaWebPageURL,
		MetaAnnounce:        row.MetaAnnounce,
		CreatedAt:           formatNullableTime(row.CreatedAt),
		UpdatedAt:           formatNullableTime(row.UpdatedAt),
	}
}

func (a *App) listExternalSources() ([]model.ExternalSubscriptionSource, error) {
	rows, err := a.db.Query(`
		SELECT id, name, category, key_category, key_insert_mode, source_url, enabled, apply_remote_metadata, pass_hwid, hwid_version, hwid_model_name, hwid_value, last_import_count, import_status, last_error,
		       last_synced_at, meta_title, meta_refresh_hours, meta_support_url, meta_web_page_url, meta_announce, created_at, updated_at
		FROM external_subscription_sources
		ORDER BY LOWER(category), id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ExternalSubscriptionSource, 0, 16)
	for rows.Next() {
		var row externalSourceRow
		var enabledInt int64
		var applyMetaInt int64
		var passHWIDInt int64
		var hwidVersion sql.NullString
		var hwidModelName sql.NullString
		var hwidValue sql.NullString
		var lastImportCountInt int64
		var importStatus sql.NullString
		var lastError sql.NullString
		var lastSyncedAt sql.NullTime
		var metaTitle sql.NullString
		var metaRefresh sql.NullInt64
		var metaSupport sql.NullString
		var metaWeb sql.NullString
		var metaAnnounce sql.NullString
		var createdAt sql.NullTime
		var updatedAt sql.NullTime
		if err := rows.Scan(
			&row.ID,
			&row.Name,
			&row.Category,
			&row.KeyCategory,
			&row.KeyInsertMode,
			&row.SourceURL,
			&enabledInt,
			&applyMetaInt,
			&passHWIDInt,
			&hwidVersion,
			&hwidModelName,
			&hwidValue,
			&lastImportCountInt,
			&importStatus,
			&lastError,
			&lastSyncedAt,
			&metaTitle,
			&metaRefresh,
			&metaSupport,
			&metaWeb,
			&metaAnnounce,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		row.Category = normalizeExternalSourceCategory(row.Category)
		row.KeyCategory = normalizeKeyCategory(row.KeyCategory)
		row.KeyInsertMode = normalizeKeyInsertMode(row.KeyInsertMode)
		row.Enabled = enabledInt != 0
		row.ApplyRemoteMetadata = applyMetaInt != 0
		row.PassHWID = passHWIDInt != 0
		row.HWIDVersion = strings.TrimSpace(hwidVersion.String)
		row.HWIDModelName = strings.TrimSpace(hwidModelName.String)
		row.HWIDValue = strings.TrimSpace(hwidValue.String)
		row.LastImportCount = int(lastImportCountInt)
		row.ImportStatus = strings.TrimSpace(importStatus.String)
		if row.ImportStatus == "" {
			row.ImportStatus = "idle"
		}
		row.LastError = strings.TrimSpace(lastError.String)
		row.LastSyncedAt = lastSyncedAt
		row.MetaTitle = strings.TrimSpace(metaTitle.String)
		if metaRefresh.Valid && metaRefresh.Int64 > 0 {
			row.MetaRefreshHours = int(metaRefresh.Int64)
		}
		row.MetaSupportURL = strings.TrimSpace(metaSupport.String)
		row.MetaWebPageURL = strings.TrimSpace(metaWeb.String)
		row.MetaAnnounce = strings.TrimSpace(metaAnnounce.String)
		row.CreatedAt = createdAt
		row.UpdatedAt = updatedAt
		out = append(out, sourceRowToModel(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *App) listExternalSourceCategories() ([]model.ExternalSourceCategory, error) {
	rows, err := a.db.Query(`SELECT name FROM external_source_categories ORDER BY LOWER(name), name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	countByName := make(map[string]int)
	categories := make([]model.ExternalSourceCategory, 0, 16)
	for rows.Next() {
		var name sql.NullString
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		normalized := normalizeExternalSourceCategory(name.String)
		if _, exists := countByName[normalized]; exists {
			continue
		}
		countByName[normalized] = 0
		categories = append(categories, model.ExternalSourceCategory{Name: normalized, SourcesCount: 0})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	countRows, err := a.db.Query(`
		SELECT category, COUNT(*)
		FROM external_subscription_sources
		GROUP BY category
	`)
	if err != nil {
		return nil, err
	}
	defer countRows.Close()

	for countRows.Next() {
		var category sql.NullString
		var count int64
		if err := countRows.Scan(&category, &count); err != nil {
			return nil, err
		}
		normalized := normalizeExternalSourceCategory(category.String)
		if _, exists := countByName[normalized]; !exists {
			categories = append(categories, model.ExternalSourceCategory{Name: normalized, SourcesCount: 0})
		}
		countByName[normalized] += int(count)
	}
	if err := countRows.Err(); err != nil {
		return nil, err
	}

	if len(categories) == 0 {
		categories = append(categories, model.ExternalSourceCategory{Name: "Общее", SourcesCount: 0})
		countByName["Общее"] = 0
	}

	for index := range categories {
		categories[index].SourcesCount = countByName[categories[index].Name]
	}

	sort.Slice(categories, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(categories[i].Name))
		right := strings.ToLower(strings.TrimSpace(categories[j].Name))
		if left == right {
			return categories[i].Name < categories[j].Name
		}
		return left < right
	})
	return categories, nil
}

func (a *App) getExternalSourceByID(id int64) (externalSourceRow, error) {
	var row externalSourceRow
	var enabledInt int64
	var applyMetaInt int64
	var passHWIDInt int64
	var hwidVersion sql.NullString
	var hwidModelName sql.NullString
	var hwidValue sql.NullString
	var lastImportCountInt int64
	var importStatus sql.NullString
	var lastError sql.NullString
	var lastSyncedAt sql.NullTime
	var metaTitle sql.NullString
	var metaRefresh sql.NullInt64
	var metaSupport sql.NullString
	var metaWeb sql.NullString
	var metaAnnounce sql.NullString
	var createdAt sql.NullTime
	var updatedAt sql.NullTime

	err := a.db.QueryRow(`
		SELECT id, name, category, key_category, key_insert_mode, source_url, enabled, apply_remote_metadata, pass_hwid, hwid_version, hwid_model_name, hwid_value, last_import_count, import_status, last_error,
		       last_synced_at, meta_title, meta_refresh_hours, meta_support_url, meta_web_page_url, meta_announce, created_at, updated_at
		FROM external_subscription_sources
		WHERE id = ?
	`, id).Scan(
		&row.ID,
		&row.Name,
		&row.Category,
		&row.KeyCategory,
		&row.KeyInsertMode,
		&row.SourceURL,
		&enabledInt,
		&applyMetaInt,
		&passHWIDInt,
		&hwidVersion,
		&hwidModelName,
		&hwidValue,
		&lastImportCountInt,
		&importStatus,
		&lastError,
		&lastSyncedAt,
		&metaTitle,
		&metaRefresh,
		&metaSupport,
		&metaWeb,
		&metaAnnounce,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return externalSourceRow{}, err
	}

	row.Category = normalizeExternalSourceCategory(row.Category)
	row.KeyCategory = normalizeKeyCategory(row.KeyCategory)
	row.KeyInsertMode = normalizeKeyInsertMode(row.KeyInsertMode)
	row.Enabled = enabledInt != 0
	row.ApplyRemoteMetadata = applyMetaInt != 0
	row.PassHWID = passHWIDInt != 0
	row.HWIDVersion = strings.TrimSpace(hwidVersion.String)
	row.HWIDModelName = strings.TrimSpace(hwidModelName.String)
	row.HWIDValue = strings.TrimSpace(hwidValue.String)
	row.LastImportCount = int(lastImportCountInt)
	row.ImportStatus = strings.TrimSpace(importStatus.String)
	if row.ImportStatus == "" {
		row.ImportStatus = "idle"
	}
	row.LastError = strings.TrimSpace(lastError.String)
	row.LastSyncedAt = lastSyncedAt
	row.MetaTitle = strings.TrimSpace(metaTitle.String)
	if metaRefresh.Valid && metaRefresh.Int64 > 0 {
		row.MetaRefreshHours = int(metaRefresh.Int64)
	}
	row.MetaSupportURL = strings.TrimSpace(metaSupport.String)
	row.MetaWebPageURL = strings.TrimSpace(metaWeb.String)
	row.MetaAnnounce = strings.TrimSpace(metaAnnounce.String)
	row.CreatedAt = createdAt
	row.UpdatedAt = updatedAt

	return row, nil
}

func normalizeImportStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "idle", "syncing", "ok", "error":
		return value
	default:
		return "idle"
	}
}

func (a *App) markExternalSourceStatus(sourceID int64, status string, errMessage string) {
	_, err := a.db.Exec(
		`UPDATE external_subscription_sources
		 SET import_status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		normalizeImportStatus(status),
		nullStringValue(errMessage),
		sourceID,
	)
	if err != nil {
		log.Printf("markExternalSourceStatus: source_id=%d err=%v", sourceID, err)
	}
}

func (a *App) syncExternalSource(sourceID int64, parsed externalSubscriptionParseResult) (int, int, error) {
	source, err := a.getExternalSourceByID(sourceID)
	if err != nil {
		return 0, 0, err
	}

	if len(parsed.Keys) == 0 {
		return 0, 0, fmt.Errorf("no keys to import")
	}

	tx, err := a.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	statusValue := model.KeyStatusActive
	if !source.Enabled {
		statusValue = model.KeyStatusNonActive
	}
	targetCategory := normalizeKeyCategory(source.KeyCategory)
	insertMode := normalizeKeyInsertMode(source.KeyInsertMode)

	type existingKey struct {
		ID  int64
		Ref string
	}
	existingRows, err := tx.Query(`
		SELECT id, external_key_ref
		FROM vless_keys
		WHERE external_source_id = ?
	`, sourceID)
	if err != nil {
		return 0, 0, err
	}
	existingByRef := make(map[string]existingKey)
	existingIDs := make(map[int64]struct{})
	for existingRows.Next() {
		var key existingKey
		if err := existingRows.Scan(&key.ID, &key.Ref); err != nil {
			_ = existingRows.Close()
			return 0, 0, err
		}
		key.Ref = strings.TrimSpace(key.Ref)
		if key.Ref != "" {
			existingByRef[key.Ref] = key
		}
		existingIDs[key.ID] = struct{}{}
	}
	if err := existingRows.Err(); err != nil {
		_ = existingRows.Close()
		return 0, 0, err
	}
	_ = existingRows.Close()

	seenRefs := make(map[string]struct{}, len(parsed.Keys))
	importedCount := 0
	skippedCount := 0

	var nextSortOrder int64
	if insertMode == "top" {
		if err := tx.QueryRow(
			`SELECT COALESCE(MIN(sort_order), 1) - ? FROM vless_keys WHERE category = ? AND external_source_id != ?`,
			len(parsed.Keys)+8,
			targetCategory,
			sourceID,
		).Scan(&nextSortOrder); err != nil {
			return 0, 0, err
		}
	} else {
		if err := tx.QueryRow(
			`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM vless_keys WHERE category = ? AND external_source_id != ?`,
			targetCategory,
			sourceID,
		).Scan(&nextSortOrder); err != nil {
			return 0, 0, err
		}
	}

	for index, item := range parsed.Keys {
		ref := strings.TrimSpace(item.Ref)
		if ref == "" {
			ref = buildExternalKeyRef(item.URL)
		}
		if _, exists := seenRefs[ref]; exists {
			continue
		}
		seenRefs[ref] = struct{}{}

		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = fmt.Sprintf("Импорт %03d", index+1)
		}
		if len(label) > 255 {
			label = label[:255]
		}

		urlValue := strings.TrimSpace(item.URL)
		if urlValue == "" {
			skippedCount++
			continue
		}
		if len(urlValue) > 65535 {
			skippedCount++
			continue
		}

		if existing, ok := existingByRef[ref]; ok {
			if _, err := tx.Exec(
				`UPDATE vless_keys
				 SET label = ?, url = ?, category = ?, status = ?, key_kind = 'real', template_text = NULL,
				     check_status = 'unknown', check_error = NULL, last_checked_at = NULL, last_latency_ms = NULL,
				     external_source_id = ?, external_key_ref = ?, sort_order = ?
				 WHERE id = ?`,
				label, urlValue, targetCategory, statusValue, sourceID, ref, nextSortOrder, existing.ID,
			); err != nil {
				return importedCount, skippedCount, err
			}
			delete(existingIDs, existing.ID)
			nextSortOrder++
			importedCount++
			continue
		}

		result, err := tx.Exec(
			`INSERT INTO vless_keys(
				label, url, category, status, check_status, key_kind, template_text, sort_order,
				external_source_id, external_key_ref
			) VALUES(?, ?, ?, ?, 'unknown', 'real', NULL, ?, ?, ?)`,
			label, urlValue, targetCategory, statusValue, nextSortOrder, sourceID, ref,
		)
		if err != nil {
			// Keep idempotent behavior: duplicate URL in local DB -> skip entry.
			errText := strings.ToLower(err.Error())
			if strings.Contains(errText, "vless_keys.url") || strings.Contains(errText, "unique") {
				skippedCount++
				continue
			}
			return importedCount, skippedCount, err
		}
		keyID, err := result.LastInsertId()
		if err != nil {
			return importedCount, skippedCount, err
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO user_keys(user_id, key_id)
			 SELECT id, ? FROM users`,
			keyID,
		); err != nil {
			return importedCount, skippedCount, err
		}
		nextSortOrder++
		importedCount++
	}

	// Remove stale keys that are no longer provided by the source.
	for keyID := range existingIDs {
		if _, err := tx.Exec(`DELETE FROM vless_keys WHERE id = ? AND external_source_id = ?`, keyID, sourceID); err != nil {
			return importedCount, skippedCount, err
		}
	}

	if _, err := tx.Exec(
		`UPDATE external_subscription_sources
		 SET import_status = 'ok',
		     last_error = NULL,
		     last_synced_at = CURRENT_TIMESTAMP,
		     last_import_count = ?,
		     meta_title = ?,
		     meta_refresh_hours = ?,
		     meta_support_url = ?,
		     meta_web_page_url = ?,
		     meta_announce = ?,
		     updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		importedCount,
		nullStringValue(parsed.Metadata.Title),
		nullInt64Value(int64(parsed.Metadata.RefreshHours)),
		nullStringValue(parsed.Metadata.SupportURL),
		nullStringValue(parsed.Metadata.WebPageURL),
		nullStringValue(parsed.Metadata.Announce),
		sourceID,
	); err != nil {
		return importedCount, skippedCount, err
	}

	if err := tx.Commit(); err != nil {
		return importedCount, skippedCount, err
	}

	return importedCount, skippedCount, nil
}

func (a *App) apiListExternalSources(w http.ResponseWriter, r *http.Request) {
	sources, err := a.listExternalSources()
	if err != nil {
		log.Printf("apiListExternalSources: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list external sources")
		return
	}
	if sources == nil {
		sources = []model.ExternalSubscriptionSource{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sources": sources,
	})
}

func (a *App) apiListExternalSourceCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.listExternalSourceCategories()
	if err != nil {
		log.Printf("apiListExternalSourceCategories: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list source categories")
		return
	}
	if categories == nil {
		categories = []model.ExternalSourceCategory{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
	})
}

func (a *App) apiCreateExternalSourceCategory(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceCategoryCreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rawName := strings.TrimSpace(req.Name)
	if rawName == "" {
		writeError(w, http.StatusBadRequest, "category name is required")
		return
	}
	name := normalizeExternalSourceCategory(rawName)

	if err := a.upsertExternalSourceCategory(name); err != nil {
		log.Printf("apiCreateExternalSourceCategory: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create source category")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"category": model.ExternalSourceCategory{Name: name},
		"message":  "source category saved",
	})
}

func (a *App) apiRenameExternalSourceCategory(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceCategoryRenameRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	oldRaw := strings.TrimSpace(req.OldName)
	newRaw := strings.TrimSpace(req.NewName)
	if oldRaw == "" || newRaw == "" {
		writeError(w, http.StatusBadRequest, "both old_name and new_name are required")
		return
	}
	oldName := normalizeExternalSourceCategory(oldRaw)
	newName := normalizeExternalSourceCategory(newRaw)
	if oldName == newName {
		writeMessage(w, "category name unchanged")
		return
	}

	var sourceCount int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources WHERE category = ?`, oldName).Scan(&sourceCount); err != nil {
		log.Printf("apiRenameExternalSourceCategory count sources: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}
	var categoryCount int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM external_source_categories WHERE name = ?`, oldName).Scan(&categoryCount); err != nil {
		log.Printf("apiRenameExternalSourceCategory count categories: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}
	if sourceCount == 0 && categoryCount == 0 {
		writeError(w, http.StatusNotFound, "source category not found")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO external_source_categories(name, updated_at)
		 VALUES(?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		newName,
	); err != nil {
		log.Printf("apiRenameExternalSourceCategory upsert new category: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	if _, err := tx.Exec(
		`UPDATE external_subscription_sources
		 SET category = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE category = ?`,
		newName,
		oldName,
	); err != nil {
		log.Printf("apiRenameExternalSourceCategory update sources: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	if _, err := tx.Exec(`DELETE FROM external_source_categories WHERE name = ?`, oldName); err != nil {
		log.Printf("apiRenameExternalSourceCategory delete old category: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	writeMessage(w, "source category renamed")
}

func (a *App) apiPreviewExternalSource(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourcePreviewRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sourceURL, err := validateExternalSourceURL(req.SourceURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	hwidProfile := normalizeExternalHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)
	metadata := externalSubscriptionMetadata{
		ContentType:    strings.TrimSpace(req.RawContentType),
		SourceFinalURL: strings.TrimSpace(req.RawFinalURL),
	}
	parsed, err := func() (externalSubscriptionParseResult, error) {
		if strings.TrimSpace(req.RawBody) != "" {
			return parseExternalSubscriptionFromRawBody(sourceURL, req.RawBody, metadata)
		}
		return fetchExternalSubscription(sourceURL, hwidProfile)
	}()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	previewItems := make([]map[string]any, 0, 12)
	for index, item := range parsed.Keys {
		if index >= 12 {
			break
		}
		previewItems = append(previewItems, map[string]any{
			"label":     item.Label,
			"scheme":    item.Scheme,
			"url_short": vless.TruncateMiddle(item.URL, 88),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"source_url":      sourceURL,
		"suggested_name":  suggestExternalSourceName(sourceURL, parsed.Metadata),
		"detected_format": parsed.DetectedFormat,
		"key_count":       len(parsed.Keys),
		"metadata": map[string]any{
			"title":            parsed.Metadata.Title,
			"refresh_hours":    parsed.Metadata.RefreshHours,
			"support_url":      parsed.Metadata.SupportURL,
			"profile_web_page": parsed.Metadata.WebPageURL,
			"announce":         parsed.Metadata.Announce,
			"content_type":     parsed.Metadata.ContentType,
			"content_disp":     parsed.Metadata.ContentDisp,
			"http_status":      parsed.Metadata.HTTPStatusLabel,
			"final_url":        parsed.Metadata.SourceFinalURL,
		},
		"warnings": nonNilWarnings(parsed.Warnings),
		"keys":     previewItems,
	})
}

func (a *App) apiImportExternalSource(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceImportRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sourceURL, err := validateExternalSourceURL(req.SourceURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Сторонняя подписка"
	}
	if len(name) > 255 {
		writeError(w, http.StatusBadRequest, "name is too long (max 24 characters)")
		return
	}
	if len(name) > 24 {
		name = name[:24]
	}
	category := normalizeExternalSourceCategory(req.Category)
	keyCategory := normalizeKeyCategory(req.KeyCategory)
	keyInsertMode := normalizeKeyInsertMode(req.KeyInsertMode)
	if err := a.upsertExternalSourceCategory(category); err != nil {
		log.Printf("apiImportExternalSource: upsert category: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save source category")
		return
	}
	hwidProfile := normalizeExternalHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)

	if _, err := a.db.Exec(
		`INSERT INTO external_subscription_sources(
			name, category, key_category, key_insert_mode, source_url, enabled, apply_remote_metadata, pass_hwid, hwid_version, hwid_model_name, hwid_value, import_status, updated_at
		)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'syncing', CURRENT_TIMESTAMP)`,
		name,
		category,
		keyCategory,
		keyInsertMode,
		sourceURL,
		boolToInt(req.Enabled),
		0,
		boolToInt(hwidProfile.PassHWID),
		nullStringValue(hwidProfile.Version),
		nullStringValue(hwidProfile.ModelName),
		nullStringValue(hwidProfile.HWID),
	); err != nil {
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "source_url") || strings.Contains(errText, "unique") {
			writeError(w, http.StatusConflict, "source_url already exists")
			return
		}
		log.Printf("apiImportExternalSource: create source: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create external source")
		return
	}

	var sourceID int64
	if err := a.db.QueryRow(`SELECT id FROM external_subscription_sources WHERE source_url = ?`, sourceURL).Scan(&sourceID); err != nil {
		log.Printf("apiImportExternalSource: load source id: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create external source")
		return
	}

	metadata := externalSubscriptionMetadata{
		ContentType:    strings.TrimSpace(req.RawContentType),
		SourceFinalURL: strings.TrimSpace(req.RawFinalURL),
	}
	parsed, err := func() (externalSubscriptionParseResult, error) {
		if strings.TrimSpace(req.RawBody) != "" {
			return parseExternalSubscriptionFromRawBody(sourceURL, req.RawBody, metadata)
		}
		return fetchExternalSubscription(sourceURL, hwidProfile)
	}()
	if err != nil {
		a.markExternalSourceStatus(sourceID, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) == "" && strings.TrimSpace(parsed.Metadata.Title) != "" {
		name = strings.TrimSpace(parsed.Metadata.Title)
		if len(name) > 24 {
			name = name[:24]
		}
		_, _ = a.db.Exec(`UPDATE external_subscription_sources SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, name, sourceID)
	}

	importedCount, skippedCount, syncErr := a.syncExternalSource(sourceID, parsed)
	if syncErr != nil {
		a.markExternalSourceStatus(sourceID, "error", syncErr.Error())
		writeError(w, http.StatusBadRequest, syncErr.Error())
		return
	}

	source, err := a.getExternalSourceByID(sourceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "source was imported but failed to load response")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":         "external source imported",
		"imported_count":  importedCount,
		"skipped_count":   skippedCount,
		"warnings":        nonNilWarnings(parsed.Warnings),
		"detected_format": parsed.DetectedFormat,
		"source":          sourceRowToModel(source),
	})
}

func (a *App) apiUpdateExternalSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req model.ExternalSourceUpdateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sourceURL, err := validateExternalSourceURL(req.SourceURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(name) > 255 {
		writeError(w, http.StatusBadRequest, "name is too long (max 24 characters)")
		return
	}
	if len(name) > 24 {
		name = name[:24]
	}
	category := normalizeExternalSourceCategory(req.Category)
	keyCategory := normalizeKeyCategory(req.KeyCategory)
	keyInsertMode := normalizeKeyInsertMode(req.KeyInsertMode)
	if err := a.upsertExternalSourceCategory(category); err != nil {
		log.Printf("apiUpdateExternalSource: upsert category: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save source category")
		return
	}
	hwidProfile := normalizeExternalHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)

	res, err := a.db.Exec(
		`UPDATE external_subscription_sources
		 SET name = ?, category = ?, key_category = ?, key_insert_mode = ?, source_url = ?, enabled = ?, apply_remote_metadata = ?, pass_hwid = ?,
		     hwid_version = ?, hwid_model_name = ?, hwid_value = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		name,
		category,
		keyCategory,
		keyInsertMode,
		sourceURL,
		boolToInt(req.Enabled),
		0,
		boolToInt(hwidProfile.PassHWID),
		nullStringValue(hwidProfile.Version),
		nullStringValue(hwidProfile.ModelName),
		nullStringValue(hwidProfile.HWID),
		id,
	)
	if err != nil {
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "source_url") || strings.Contains(errText, "unique") {
			writeError(w, http.StatusConflict, "source_url already exists")
			return
		}
		log.Printf("apiUpdateExternalSource: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update external source")
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "external source not found")
		return
	}

	keyStatus := model.KeyStatusActive
	if !req.Enabled {
		keyStatus = model.KeyStatusNonActive
	}
	if _, err := a.db.Exec(`UPDATE vless_keys SET status = ? WHERE external_source_id = ?`, keyStatus, id); err != nil {
		log.Printf("apiUpdateExternalSource: update key statuses: %v", err)
	}

	writeMessage(w, "external source updated")
}

func (a *App) apiDeleteExternalSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete external source")
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM vless_keys WHERE external_source_id = ?`, id); err != nil {
		log.Printf("apiDeleteExternalSource: delete keys: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete source keys")
		return
	}

	res, err := tx.Exec(`DELETE FROM external_subscription_sources WHERE id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteExternalSource: delete source: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete external source")
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "external source not found")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete external source")
		return
	}

	writeMessage(w, "external source deleted")
}

func (a *App) apiSyncExternalSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	source, err := a.getExternalSourceByID(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "external source not found")
			return
		}
		log.Printf("apiSyncExternalSource: load source: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load external source")
		return
	}

	a.markExternalSourceStatus(id, "syncing", "")
	hwidProfile := normalizeExternalHWIDProfile(source.PassHWID, source.HWIDVersion, source.HWIDModelName, source.HWIDValue)
	parsed, err := fetchExternalSubscription(source.SourceURL, hwidProfile)
	if err != nil {
		a.markExternalSourceStatus(id, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	importedCount, skippedCount, syncErr := a.syncExternalSource(id, parsed)
	if syncErr != nil {
		a.markExternalSourceStatus(id, "error", syncErr.Error())
		writeError(w, http.StatusBadRequest, syncErr.Error())
		return
	}

	updatedSource, err := a.getExternalSourceByID(id)
	if err != nil {
		log.Printf("apiSyncExternalSource: reload source: %v", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"message":        "external source synced",
			"imported_count": importedCount,
			"skipped_count":  skippedCount,
			"warnings":       nonNilWarnings(parsed.Warnings),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":        "external source synced",
		"imported_count": importedCount,
		"skipped_count":  skippedCount,
		"warnings":       nonNilWarnings(parsed.Warnings),
		"source":         sourceRowToModel(updatedSource),
	})
}
