package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"net/http"
	"strings"
)

type sourceCreateResult struct {
	ID   int64
	Sync sources.SyncResult
}

var (
	errSourceCreate      = sourceFailure{status: http.StatusInternalServerError, code: "source_create_failed", message: "failed to create source"}
	errSourceImportSave  = sourceFailure{status: http.StatusInternalServerError, code: "source_create_failed", message: "failed to save imported source"}
	errSourceURLConflict = sourceFailure{status: http.StatusConflict, code: "source_url_conflict", message: "source URL already exists", field: "source_url"}
)

type sourceCreateCategories struct {
	sourceCategoryID int64
	keyCategoryID    any
}

// Publish source identity only after category, source, profiles, credentials and
// assignments commit together. Response loading and audit stay in HTTP.
func (a *App) createExternalSource(source externalSourceRow, parsed sources.ParseResult) (sourceCreateResult, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return sourceCreateResult{}, errSourceCreate
	}
	defer tx.Rollback()
	categories, err := resolveSourceCreateCategories(tx, source)
	if err != nil {
		return sourceCreateResult{}, err
	}
	source.ID, err = insertCreatedSource(tx, source, categories)
	if err != nil {
		return sourceCreateResult{}, err
	}
	result, err := sources.Sync(a.store().SourceSyncOn(context.Background(), tx), source.syncTarget(), parsed, a.externalProfileFingerprintKeys())
	if err != nil {
		return sourceCreateResult{}, errSourceImportSave
	}
	if err := tx.Commit(); err != nil {
		return sourceCreateResult{}, errSourceImportSave
	}
	return sourceCreateResult{source.ID, result}, nil
}

func resolveSourceCreateCategories(tx *sql.Tx, source externalSourceRow) (sourceCreateCategories, error) {
	if _, err := tx.Exec(`INSERT OR IGNORE INTO external_source_categories(name) VALUES(?)`, source.Category); err != nil {
		return sourceCreateCategories{}, errSourceCategory
	}
	var categories sourceCreateCategories
	if err := tx.QueryRow(`SELECT id FROM external_source_categories WHERE name = ?`, source.Category).Scan(&categories.sourceCategoryID); err != nil {
		return sourceCreateCategories{}, sourceFailure{status: http.StatusInternalServerError, code: "source_category_failed", message: "failed to resolve source category"}
	}
	var err error
	categories.keyCategoryID, err = ensureKeyCategory(tx, source.KeyCategory)
	if err != nil {
		return sourceCreateCategories{}, sourceFailure{status: http.StatusInternalServerError, code: "key_category_failed", message: err.Error()}
	}
	return categories, nil
}

func insertCreatedSource(tx *sql.Tx, source externalSourceRow, categories sourceCreateCategories) (int64, error) {
	result, err := tx.Exec(`
		INSERT INTO external_subscription_sources(
			name, source_category_id, category, key_category_id, key_category, key_insert_mode, source_url, enabled, apply_remote_metadata,
			pass_hwid, hwid_version, hwid_model_name, hwid_value, import_status, updated_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'syncing', CURRENT_TIMESTAMP)
	`, source.Name, categories.sourceCategoryID, source.Category, categories.keyCategoryID, source.KeyCategory, source.KeyInsertMode, source.SourceURL,
		boolToInt(source.Enabled), boolToInt(source.ApplyRemoteMetadata), boolToInt(source.PassHWID),
		nullStringValue(source.HWIDVersion), nullStringValue(source.HWIDModelName), nullStringValue(source.HWIDValue))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return 0, errSourceURLConflict
		}
		return 0, errSourceCreate
	}
	id, _ := result.LastInsertId()
	return id, nil
}

// ensureKeyCategory creates the key category when missing and returns its id
// as a nullable SQL argument (nil when no category is requested). Error
// messages are safe to return to the client.
func ensureKeyCategory(tx *sql.Tx, keyCategory string) (any, error) {
	if keyCategory == "" {
		return nil, nil
	}
	var nextCategoryOrder int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextCategoryOrder); err != nil {
		return nil, errors.New("failed to save key category")
	}
	if _, err := tx.Exec(`
			INSERT INTO key_categories(name, color, sort_order, updated_at)
			VALUES(?, '#d8b33d', ?, CURRENT_TIMESTAMP)
			ON CONFLICT(name) DO NOTHING
		`, keyCategory, nextCategoryOrder); err != nil {
		return nil, errors.New("failed to save key category")
	}
	var resolvedID int64
	if err := tx.QueryRow(`SELECT id FROM key_categories WHERE name = ?`, keyCategory).Scan(&resolvedID); err != nil {
		return nil, errors.New("failed to resolve key category")
	}
	return resolvedID, nil
}
