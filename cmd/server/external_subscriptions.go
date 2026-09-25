package main

import (
	"context"
	"database/sql"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/sources"
)

func (a *App) externalProfileFingerprintKeys() [][]byte {
	if a == nil || len(a.profileFingerprintKey) == 0 {
		return nil
	}
	keys := make([][]byte, 0, 1+len(a.profileFingerprintOldKeys))
	keys = append(keys, append([]byte(nil), a.profileFingerprintKey...))
	keys = append(keys, cloneByteSlices(a.profileFingerprintOldKeys)...)
	return keys
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

func (a *App) upsertExternalSourceCategory(category string) error {
	category = sources.NormalizeCategory(category)
	_, err := a.db.Exec(
		`INSERT INTO external_source_categories(name, updated_at)
		 VALUES(?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		category,
	)
	return err
}

func formatNullableTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Local().Format("2006-01-02 15:04:05")
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
		normalized := sources.NormalizeCategory(name.String)
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
		normalized := sources.NormalizeCategory(category.String)
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

	row.Category = sources.NormalizeCategory(row.Category)
	row.KeyCategory = normalizeKeyCategory(row.KeyCategory)
	row.KeyInsertMode = sources.NormalizeKeyInsertMode(row.KeyInsertMode)
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

func (a *App) syncExternalSource(sourceID int64, parsed sources.ParseResult) (sources.SyncResult, error) {
	// SQLite uniqueness constraints are the final duplicate barrier. Serializing
	// the short read/upsert transaction also makes overlapping retries produce
	// deterministic classifications instead of SQLITE_BUSY failures.
	a.mu.Lock()
	defer a.mu.Unlock()

	source, err := a.getExternalSourceByID(sourceID)
	if err != nil {
		return sources.SyncResult{}, err
	}

	sync, err := a.store().BeginSourceSync(context.Background())
	if err != nil {
		return sources.SyncResult{}, err
	}
	defer sync.Rollback()

	result, err := sources.Sync(sync, source.syncTarget(), parsed, a.externalProfileFingerprintKeys())
	if err != nil {
		return result, err
	}
	if err := sync.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func (a *App) apiListExternalSourceCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := a.listExternalSourceCategories()
	if err != nil {
		log.Printf("apiListExternalSourceCategories: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to list source categories")
		return
	}
	if categories == nil {
		categories = []model.ExternalSourceCategory{}
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
	})
}

func (a *App) apiCreateExternalSourceCategory(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceCategoryCreateRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	rawName := strings.TrimSpace(req.Name)
	if rawName == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "category name is required")
		return
	}
	name := sources.NormalizeCategory(rawName)

	if err := a.upsertExternalSourceCategory(name); err != nil {
		log.Printf("apiCreateExternalSourceCategory: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create source category")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"category": model.ExternalSourceCategory{Name: name},
		"message":  "source category saved",
	})
}

func (a *App) apiRenameExternalSourceCategory(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceCategoryRenameRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	oldRaw := strings.TrimSpace(req.OldName)
	newRaw := strings.TrimSpace(req.NewName)
	if oldRaw == "" || newRaw == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "both old_name and new_name are required")
		return
	}
	oldName := sources.NormalizeCategory(oldRaw)
	newName := sources.NormalizeCategory(newRaw)
	if oldName == newName {
		httpapi.WriteMessage(w, "category name unchanged")
		return
	}

	var sourceCount int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM external_subscription_sources WHERE category = ?`, oldName).Scan(&sourceCount); err != nil {
		log.Printf("apiRenameExternalSourceCategory count sources: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to rename source category")
		return
	}
	var categoryCount int64
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM external_source_categories WHERE name = ?`, oldName).Scan(&categoryCount); err != nil {
		log.Printf("apiRenameExternalSourceCategory count categories: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to rename source category")
		return
	}
	if sourceCount == 0 && categoryCount == 0 {
		httpapi.WriteError(w, r, http.StatusNotFound, "source category not found")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to rename source category")
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
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to rename source category")
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
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	if _, err := tx.Exec(`DELETE FROM external_source_categories WHERE name = ?`, oldName); err != nil {
		log.Printf("apiRenameExternalSourceCategory delete old category: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to rename source category")
		return
	}

	httpapi.WriteMessage(w, "source category renamed")
}

func (row externalSourceRow) syncTarget() sources.SyncTarget {
	return sources.SyncTarget{ID: row.ID, Enabled: row.Enabled, KeyCategory: row.KeyCategory, KeyInsertMode: row.KeyInsertMode}
}

// sourceClient is the HTTP client used to fetch external subscriptions. Tests
// may replace it before the first fetch.
func (a *App) sourceClient() *http.Client {
	a.sourceClientOnce.Do(func() {
		if a.sourceHTTPClient == nil {
			a.sourceHTTPClient = sources.NewHTTPClient()
		}
	})
	return a.sourceHTTPClient
}
