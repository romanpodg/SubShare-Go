package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"subshare/internal/middleware"
	"subshare/internal/model"
	"subshare/internal/vless"
)

type sourceV1Summary struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Category        string `json:"category"`
	KeyCategory     string `json:"key_category"`
	SourceURLMasked string `json:"source_url_masked"`
	Enabled         bool   `json:"enabled"`
	PassHWID        bool   `json:"pass_hwid"`
	HasHWIDValue    bool   `json:"has_hwid_value"`
	LastImportCount int    `json:"last_import_count"`
	ImportedKeys    int    `json:"imported_keys"`
	ImportStatus    string `json:"import_status"`
	LastError       string `json:"last_error"`
	LastSyncedAt    string `json:"last_synced_at"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type sourceV1Detail struct {
	sourceV1Summary
	SourceURL        string `json:"source_url"`
	KeyInsertMode    string `json:"key_insert_mode"`
	HWIDVersion      string `json:"hwid_version"`
	HWIDModelName    string `json:"hwid_model_name"`
	MetaTitle        string `json:"meta_title"`
	MetaRefreshHours int    `json:"meta_refresh_hours"`
	MetaSupportURL   string `json:"meta_support_url"`
	MetaWebPageURL   string `json:"meta_web_page_url"`
	MetaAnnounce     string `json:"meta_announce"`
}

type sourceV1UpdateRequest struct {
	Name                string  `json:"name"`
	Category            string  `json:"category"`
	KeyCategory         string  `json:"key_category"`
	KeyInsertMode       string  `json:"key_insert_mode"`
	SourceURL           string  `json:"source_url"`
	Enabled             bool    `json:"enabled"`
	PassHWID            bool    `json:"pass_hwid"`
	HWIDVersion         string  `json:"hwid_version"`
	HWIDModelName       string  `json:"hwid_model_name"`
	HWIDValue           *string `json:"hwid_value"`
	ClearHWIDValue      bool    `json:"clear_hwid_value"`
	ApplyRemoteMetadata bool    `json:"apply_remote_metadata"`
}

func maskExternalSourceURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "URL hidden"
	}
	return parsed.Scheme + "://" + parsed.Host + "/•••"
}

func normalizeExternalSourceName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		name = "Сторонняя подписка"
	}
	runes := []rune(name)
	if len(runes) > 24 {
		return "", fmt.Errorf("name is too long (max 24 characters)")
	}
	return name, nil
}

func sourceSummaryFromRow(row externalSourceRow, importedKeys int) sourceV1Summary {
	return sourceV1Summary{
		ID:              row.ID,
		Name:            row.Name,
		Category:        row.Category,
		KeyCategory:     row.KeyCategory,
		SourceURLMasked: maskExternalSourceURL(row.SourceURL),
		Enabled:         row.Enabled,
		PassHWID:        row.PassHWID,
		HasHWIDValue:    strings.TrimSpace(row.HWIDValue) != "",
		LastImportCount: row.LastImportCount,
		ImportedKeys:    importedKeys,
		ImportStatus:    row.ImportStatus,
		LastError:       row.LastError,
		LastSyncedAt:    formatNullableTime(row.LastSyncedAt),
		CreatedAt:       formatNullableTime(row.CreatedAt),
		UpdatedAt:       formatNullableTime(row.UpdatedAt),
	}
}

func sourceDetailFromRow(row externalSourceRow, importedKeys int) sourceV1Detail {
	return sourceV1Detail{
		sourceV1Summary:  sourceSummaryFromRow(row, importedKeys),
		SourceURL:        row.SourceURL,
		KeyInsertMode:    row.KeyInsertMode,
		HWIDVersion:      row.HWIDVersion,
		HWIDModelName:    row.HWIDModelName,
		MetaTitle:        row.MetaTitle,
		MetaRefreshHours: row.MetaRefreshHours,
		MetaSupportURL:   row.MetaSupportURL,
		MetaWebPageURL:   row.MetaWebPageURL,
		MetaAnnounce:     row.MetaAnnounce,
	}
}

func writeV1FieldError(w http.ResponseWriter, r *http.Request, status int, code, message, field string) {
	requestID, _ := r.Context().Value(middleware.CtxKeyRequestID).(string)
	fieldErrors := map[string][]string{}
	if field != "" {
		fieldErrors[field] = []string{message}
	}
	writeJSON(w, status, map[string]any{
		"error": message, "code": code, "message": message,
		"field_errors": fieldErrors, "request_id": requestID,
	})
}

func sourceListFilter(r *http.Request) (string, []any) {
	clauses := []string{"1 = 1"}
	args := make([]any, 0, 3)
	if query := strings.TrimSpace(r.URL.Query().Get("query")); query != "" {
		like := "%" + strings.ToLower(query) + "%"
		clauses = append(clauses, "(LOWER(s.name) LIKE ? OR LOWER(s.category) LIKE ? OR LOWER(s.source_url) LIKE ?)")
		args = append(args, like, like, like)
	}
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))) {
	case "", "all":
	case "disabled":
		clauses = append(clauses, "s.enabled = 0")
	case "idle", "syncing", "ok", "error":
		clauses = append(clauses, "s.import_status = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))))
	default:
		clauses = append(clauses, "1 = 0")
	}
	return strings.Join(clauses, " AND "), args
}

func sourceListOrder(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "name_asc":
		return "LOWER(s.name), s.id"
	case "last_sync_desc":
		return "s.last_synced_at DESC, s.id DESC"
	case "created_asc":
		return "s.id ASC"
	default:
		return "s.id DESC"
	}
}

func (a *App) apiV1ListSources(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	where, args := sourceListFilter(r)
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources s WHERE `+where, args...).Scan(&total); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "sources_list_failed", "failed to load sources")
		return
	}
	query := `
		SELECT s.id, s.name, s.category, s.key_category, s.source_url, s.enabled, s.pass_hwid,
		       CASE WHEN TRIM(COALESCE(s.hwid_value, '')) <> '' THEN 1 ELSE 0 END,
		       s.last_import_count, s.import_status, COALESCE(s.last_error, ''), s.last_synced_at,
		       s.created_at, s.updated_at, COUNT(k.id)
		FROM external_subscription_sources s
		LEFT JOIN vless_keys k ON k.external_source_id = s.id
		WHERE ` + where + `
		GROUP BY s.id
		ORDER BY ` + sourceListOrder(r.URL.Query().Get("sort")) + `
		LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := a.db.Query(query, queryArgs...)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "sources_list_failed", "failed to load sources")
		return
	}
	defer rows.Close()
	items := make([]sourceV1Summary, 0, pageSize)
	for rows.Next() {
		var item sourceV1Summary
		var enabled, passHWID, hasHWID int
		var lastSynced, createdAt, updatedAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Category, &item.KeyCategory, &item.SourceURLMasked,
			&enabled, &passHWID, &hasHWID, &item.LastImportCount, &item.ImportStatus,
			&item.LastError, &lastSynced, &createdAt, &updatedAt, &item.ImportedKeys,
		); err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "sources_list_failed", "failed to load sources")
			return
		}
		item.Category = normalizeExternalSourceCategory(item.Category)
		item.KeyCategory = normalizeKeyCategory(item.KeyCategory)
		item.SourceURLMasked = maskExternalSourceURL(item.SourceURLMasked)
		item.Enabled = enabled != 0
		item.PassHWID = passHWID != 0
		item.HasHWIDValue = hasHWID != 0
		item.LastSyncedAt = formatNullableTime(lastSynced)
		item.CreatedAt = formatNullableTime(createdAt)
		item.UpdatedAt = formatNullableTime(updatedAt)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "sources_list_failed", "failed to load sources")
		return
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": pageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages},
	})
}

func (a *App) apiV1GetSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	row, err := a.getExternalSourceByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_load_failed", "failed to load source")
		return
	}
	var importedKeys int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, id).Scan(&importedKeys); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_load_failed", "failed to load source")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": sourceDetailFromRow(row, importedKeys)})
}

func (a *App) apiV1PreviewSource(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourcePreviewRequest
	if err := readJSON(r, &req); err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "validation_failed", "invalid request body", "")
		return
	}
	sourceURL, err := validateExternalSourceURL(req.SourceURL)
	if err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "source_url_invalid", err.Error(), "source_url")
		return
	}
	hwidProfile := normalizeExternalHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)
	metadata := externalSubscriptionMetadata{
		ContentType: strings.TrimSpace(req.RawContentType), SourceFinalURL: strings.TrimSpace(req.RawFinalURL),
	}
	var parsed externalSubscriptionParseResult
	if strings.TrimSpace(req.RawBody) != "" {
		parsed, err = parseExternalSubscriptionFromRawBody(sourceURL, req.RawBody, metadata)
	} else {
		parsed, err = fetchExternalSubscription(sourceURL, hwidProfile)
	}
	if err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "source_preview_failed", err.Error(), "source_url")
		return
	}
	previewItems := make([]map[string]any, 0, len(parsed.Keys))
	for _, item := range parsed.Keys {
		previewItems = append(previewItems, map[string]any{
			"label": item.Label, "scheme": item.Scheme, "url_short": vless.TruncateMiddle(item.URL, 88),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source_url": sourceURL, "suggested_name": suggestExternalSourceName(sourceURL, parsed.Metadata),
		"detected_format": parsed.DetectedFormat, "key_count": len(parsed.Keys),
		"metadata": map[string]any{
			"title": parsed.Metadata.Title, "refresh_hours": parsed.Metadata.RefreshHours,
			"support_url": parsed.Metadata.SupportURL, "profile_web_page": parsed.Metadata.WebPageURL,
			"announce": parsed.Metadata.Announce, "content_type": parsed.Metadata.ContentType,
			"content_disp": parsed.Metadata.ContentDisp, "http_status": parsed.Metadata.HTTPStatusLabel,
			"final_url": parsed.Metadata.SourceFinalURL,
		},
		"warnings": nonNilWarnings(parsed.Warnings), "keys": previewItems,
	})
}

func (a *App) apiV1CreateSource(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceImportRequest
	if err := readJSON(r, &req); err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "validation_failed", "invalid request body", "")
		return
	}
	sourceURL, err := validateExternalSourceURL(req.SourceURL)
	if err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "source_url_invalid", err.Error(), "source_url")
		return
	}
	var exists bool
	if err := a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM external_subscription_sources WHERE source_url = ?)`, sourceURL).Scan(&exists); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_create_failed", "failed to create source")
		return
	}
	if exists {
		writeV1FieldError(w, r, http.StatusConflict, "source_url_conflict", "source URL already exists", "source_url")
		return
	}
	name, err := normalizeExternalSourceName(req.Name)
	if err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "source_name_invalid", err.Error(), "name")
		return
	}
	hwidProfile := normalizeExternalHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)
	metadata := externalSubscriptionMetadata{
		ContentType: strings.TrimSpace(req.RawContentType), SourceFinalURL: strings.TrimSpace(req.RawFinalURL),
	}
	var parsed externalSubscriptionParseResult
	if strings.TrimSpace(req.RawBody) != "" {
		parsed, err = parseExternalSubscriptionFromRawBody(sourceURL, req.RawBody, metadata)
	} else {
		parsed, err = fetchExternalSubscription(sourceURL, hwidProfile)
	}
	if err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "source_import_failed", err.Error(), "source_url")
		return
	}
	if len(parsed.Keys) == 0 {
		writeV1FieldError(w, r, http.StatusBadRequest, "source_empty", "no keys to import", "source_url")
		return
	}
	category := normalizeExternalSourceCategory(req.Category)
	keyCategory := normalizeKeyCategory(req.KeyCategory)
	tx, err := a.db.Begin()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_create_failed", "failed to create source")
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO external_source_categories(name) VALUES(?)`, category); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_category_failed", "failed to save source category")
		return
	}
	var sourceCategoryID int64
	if err := tx.QueryRow(`SELECT id FROM external_source_categories WHERE name = ?`, category).Scan(&sourceCategoryID); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_category_failed", "failed to resolve source category")
		return
	}
	var keyCategoryID any
	if keyCategory != "" {
		var nextCategoryOrder int64
		if err := tx.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextCategoryOrder); err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "key_category_failed", "failed to save key category")
			return
		}
		if _, err := tx.Exec(`
			INSERT INTO key_categories(name, color, sort_order, updated_at)
			VALUES(?, '#d8b33d', ?, CURRENT_TIMESTAMP)
			ON CONFLICT(name) DO NOTHING
		`, keyCategory, nextCategoryOrder); err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "key_category_failed", "failed to save key category")
			return
		}
		var resolvedID int64
		if err := tx.QueryRow(`SELECT id FROM key_categories WHERE name = ?`, keyCategory).Scan(&resolvedID); err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "key_category_failed", "failed to resolve key category")
			return
		}
		keyCategoryID = resolvedID
	}

	result, err := tx.Exec(`
		INSERT INTO external_subscription_sources(
			name, source_category_id, category, key_category_id, key_category, key_insert_mode, source_url, enabled, apply_remote_metadata,
			pass_hwid, hwid_version, hwid_model_name, hwid_value, import_status, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'syncing', CURRENT_TIMESTAMP)
	`, name, sourceCategoryID, category, keyCategoryID, keyCategory, normalizeKeyInsertMode(req.KeyInsertMode), sourceURL,
		boolToInt(req.Enabled), boolToInt(req.ApplyRemoteMetadata), boolToInt(hwidProfile.PassHWID),
		nullStringValue(hwidProfile.Version), nullStringValue(hwidProfile.ModelName), nullStringValue(hwidProfile.HWID))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeV1FieldError(w, r, http.StatusConflict, "source_url_conflict", "source URL already exists", "source_url")
			return
		}
		writeV1Error(w, r, http.StatusInternalServerError, "source_create_failed", "failed to create source")
		return
	}
	sourceID, _ := result.LastInsertId()
	source := externalSourceRow{
		ID:                  sourceID,
		Name:                name,
		Category:            category,
		KeyCategory:         keyCategory,
		KeyInsertMode:       normalizeKeyInsertMode(req.KeyInsertMode),
		SourceURL:           sourceURL,
		Enabled:             req.Enabled,
		ApplyRemoteMetadata: req.ApplyRemoteMetadata,
		PassHWID:            hwidProfile.PassHWID,
		HWIDVersion:         hwidProfile.Version,
		HWIDModelName:       hwidProfile.ModelName,
		HWIDValue:           hwidProfile.HWID,
		ImportStatus:        "syncing",
	}
	imported, skipped, err := syncExternalSourceTx(tx, source, parsed)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_create_failed", "failed to save imported source")
		return
	}
	if err := tx.Commit(); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_create_failed", "failed to save imported source")
		return
	}
	row, err := a.getExternalSourceByID(sourceID)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_create_failed", "source created but response could not be loaded")
		return
	}
	a.recordAuditEvent(r, "external_source.create", "external_source", strconv.FormatInt(sourceID, 10), map[string]any{
		"imported_count": imported, "skipped_count": skipped,
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"data": sourceDetailFromRow(row, imported), "imported_count": imported,
		"skipped_count": skipped, "warnings": nonNilWarnings(parsed.Warnings),
		"detected_format": parsed.DetectedFormat,
	})
}

func (a *App) apiV1UpdateSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	current, err := a.getExternalSourceByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_load_failed", "failed to load source")
		return
	}
	var req sourceV1UpdateRequest
	if err := readJSON(r, &req); err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "validation_failed", "invalid request body", "")
		return
	}
	name, err := normalizeExternalSourceName(req.Name)
	if err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "source_name_invalid", err.Error(), "name")
		return
	}
	sourceURL, err := validateExternalSourceURL(req.SourceURL)
	if err != nil {
		writeV1FieldError(w, r, http.StatusBadRequest, "source_url_invalid", err.Error(), "source_url")
		return
	}
	category := normalizeExternalSourceCategory(req.Category)
	keyCategory := normalizeKeyCategory(req.KeyCategory)
	if err := a.upsertExternalSourceCategory(category); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_category_failed", "failed to save source category")
		return
	}
	if keyCategory != "" {
		if err := a.upsertKeyCategory(keyCategory); err != nil {
			writeV1Error(w, r, http.StatusInternalServerError, "key_category_failed", "failed to save key category")
			return
		}
	}
	hwidValue := current.HWIDValue
	if req.ClearHWIDValue {
		hwidValue = ""
	} else if req.HWIDValue != nil && strings.TrimSpace(*req.HWIDValue) != "" {
		hwidValue = strings.TrimSpace(*req.HWIDValue)
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_update_failed", "failed to update source")
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec(`
		UPDATE external_subscription_sources
		SET name = ?, category = ?, key_category = ?, key_insert_mode = ?, source_url = ?, enabled = ?,
		    apply_remote_metadata = ?, pass_hwid = ?, hwid_version = ?, hwid_model_name = ?,
		    hwid_value = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, name, category, keyCategory, normalizeKeyInsertMode(req.KeyInsertMode), sourceURL, boolToInt(req.Enabled),
		boolToInt(req.ApplyRemoteMetadata), boolToInt(req.PassHWID), nullStringValue(strings.TrimSpace(req.HWIDVersion)),
		nullStringValue(strings.TrimSpace(req.HWIDModelName)), nullStringValue(hwidValue), id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeV1FieldError(w, r, http.StatusConflict, "source_url_conflict", "source URL already exists", "source_url")
			return
		}
		writeV1Error(w, r, http.StatusInternalServerError, "source_update_failed", "failed to update source")
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeV1Error(w, r, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	keyStatus := model.KeyStatusActive
	if !req.Enabled {
		keyStatus = model.KeyStatusNonActive
	}
	if _, err := tx.Exec(`UPDATE vless_keys SET status = ? WHERE external_source_id = ?`, keyStatus, id); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_update_failed", "failed to update source keys")
		return
	}
	if err := tx.Commit(); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_update_failed", "failed to update source")
		return
	}
	a.recordAuditEvent(r, "external_source.update", "external_source", strconv.FormatInt(id, 10), map[string]any{"enabled": req.Enabled})
	writeJSON(w, http.StatusOK, map[string]any{"message": "source updated"})
}

func (a *App) apiV1DeleteSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_delete_failed", "failed to delete source")
		return
	}
	defer tx.Rollback()
	var deletedKeys int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM vless_keys WHERE external_source_id = ?`, id).Scan(&deletedKeys); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_delete_failed", "failed to delete source")
		return
	}
	if _, err := tx.Exec(`DELETE FROM vless_keys WHERE external_source_id = ?`, id); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_delete_failed", "failed to delete source keys")
		return
	}
	result, err := tx.Exec(`DELETE FROM external_subscription_sources WHERE id = ?`, id)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_delete_failed", "failed to delete source")
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeV1Error(w, r, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	if err := tx.Commit(); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_delete_failed", "failed to delete source")
		return
	}
	a.recordAuditEvent(r, "external_source.delete", "external_source", strconv.FormatInt(id, 10), map[string]any{"deleted_keys": deletedKeys})
	writeJSON(w, http.StatusOK, map[string]any{"message": "source deleted", "deleted_keys": deletedKeys})
}

func (a *App) apiV1ListSourceCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.listExternalSourceCategories()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "source_categories_failed", "failed to load source categories")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": categories})
}

func (a *App) apiV1ListKeyCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.listKeyCategories()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "key_categories_failed", "failed to load key categories")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": categories})
}
