package main

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

var errSourceUpdate = sourceFailure{status: http.StatusInternalServerError, code: "source_update_failed", message: "failed to update source"}

// Source settings and linked key status publish together; the caller audits only
// after commit. Category preparation retains its existing separate lifecycle.
func (a *App) updateExternalSource(source externalSourceRow) error {
	tx, err := a.db.Begin()
	if err != nil {
		return errSourceUpdate
	}
	defer tx.Rollback()
	if err := updateSourceSettings(tx, source); err != nil {
		return err
	}
	if err := updateSourceKeysStatus(tx, source); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return errSourceUpdate
	}
	return nil
}

func updateSourceSettings(tx *sql.Tx, source externalSourceRow) error {
	result, err := tx.Exec(`
		UPDATE external_subscription_sources
		SET name = ?, category = ?, key_category = ?, key_insert_mode = ?, source_url = ?, enabled = ?,
		    apply_remote_metadata = ?, pass_hwid = ?, hwid_version = ?, hwid_model_name = ?,
		    hwid_value = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, source.Name, source.Category, source.KeyCategory, source.KeyInsertMode, source.SourceURL, boolToInt(source.Enabled),
		boolToInt(source.ApplyRemoteMetadata), boolToInt(source.PassHWID), nullStringValue(source.HWIDVersion),
		nullStringValue(source.HWIDModelName), nullStringValue(source.HWIDValue), source.ID)
	if err != nil {
		return sourceUpdateWriteError(err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return errSourceNotFound
	}
	return nil
}

func sourceUpdateWriteError(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "unique") {
		return errSourceURLConflict
	}
	return errSourceUpdate
}

func updateSourceKeysStatus(tx *sql.Tx, source externalSourceRow) error {
	keyStatus := model.KeyStatusActive
	if !source.Enabled {
		keyStatus = model.KeyStatusNonActive
	}
	if err := storage.SetSourceKeysStatus(context.Background(), tx, source.ID, keyStatus); err != nil {
		return sourceFailure{status: http.StatusInternalServerError, code: "source_update_failed", message: "failed to update source keys"}
	}
	return nil
}
