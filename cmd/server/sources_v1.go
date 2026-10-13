package main

import (
	"context"
	"database/sql"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"github.com/romanpodg/SubShare-Go/internal/storage"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	if err != nil {
		return "URL hidden"
	}
	if parsed.Scheme == "" {
		return "URL hidden"
	}
	if parsed.Host == "" {
		return "URL hidden"
	}
	return parsed.Scheme + "://" + parsed.Host + "/•••"
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
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "sources_list_failed", "failed to load sources")
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
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "sources_list_failed", "failed to load sources")
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
			httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "sources_list_failed", "failed to load sources")
			return
		}
		item.Category = sources.NormalizeCategory(item.Category)
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
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "sources_list_failed", "failed to load sources")
		return
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"data": items,
		"meta": pageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages},
	})
}

func (a *App) apiV1GetSource(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	row, err := a.loadSourceForV1(id)
	if err != nil {
		writeSourceFailure(w, r, err)
		return
	}
	importedKeys, err := storage.CountSourceKeys(context.Background(), a.db, id)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "source_load_failed", "failed to load source")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": sourceDetailFromRow(row, importedKeys)})
}

func (a *App) apiV1UpdateSource(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	current, err := a.loadSourceForV1(id)
	if err != nil {
		writeSourceFailure(w, r, err)
		return
	}
	var req sourceV1UpdateRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		writeSourceFailure(w, r, errSourceRequestBody)
		return
	}
	source, err := a.prepareSourceUpdate(current, req)
	if err != nil {
		writeSourceFailure(w, r, err)
		return
	}
	if err := a.updateExternalSource(source); err != nil {
		writeSourceFailure(w, r, err)
		return
	}
	a.recordAuditEvent(r, "external_source.update", "external_source", strconv.FormatInt(id, 10), map[string]any{"enabled": req.Enabled})
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"message": "source updated"})
}

func (a *App) apiV1DeleteSource(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "source_delete_failed", "failed to delete source")
		return
	}
	defer tx.Rollback()
	deletedKeys, err := storage.DeleteSourceKeys(context.Background(), tx, id)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "source_delete_failed", "failed to delete source keys")
		return
	}
	result, err := tx.Exec(`DELETE FROM external_subscription_sources WHERE id = ?`, id)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "source_delete_failed", "failed to delete source")
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		httpapi.WriteV1Error(w, r, http.StatusNotFound, "source_not_found", "source not found")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "source_delete_failed", "failed to delete source")
		return
	}
	a.recordAuditEvent(r, "external_source.delete", "external_source", strconv.FormatInt(id, 10), map[string]any{"deleted_keys": deletedKeys})
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"message": "source deleted", "deleted_keys": deletedKeys})
}

func (a *App) apiV1ListSourceCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.listExternalSourceCategories()
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "source_categories_failed", "failed to load source categories")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": categories})
}
