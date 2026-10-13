package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/sources"
)

var (
	errSourceNotFound    = sourceFailure{status: http.StatusNotFound, code: "source_not_found", message: "source not found"}
	errSourceLoad        = sourceFailure{status: http.StatusInternalServerError, code: "source_load_failed", message: "failed to load source"}
	errSourceCategory    = sourceFailure{status: http.StatusInternalServerError, code: "source_category_failed", message: "failed to save source category"}
	errSourceKeyCategory = sourceFailure{status: http.StatusInternalServerError, code: "key_category_failed", message: "failed to save key category"}
)

func (a *App) loadSourceForV1(id int64) (externalSourceRow, error) {
	row, err := a.getExternalSourceByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		return externalSourceRow{}, errSourceNotFound
	}
	if err != nil {
		return externalSourceRow{}, errSourceLoad
	}
	return row, nil
}

func (a *App) prepareSourceUpdate(current externalSourceRow, req sourceV1UpdateRequest) (externalSourceRow, error) {
	name, err := sources.ValidateName(req.Name)
	if err != nil {
		return externalSourceRow{}, sourceNameFailure(err)
	}
	sourceURL, err := sources.ValidateURL(req.SourceURL)
	if err != nil {
		return externalSourceRow{}, sourceURLFailure(err)
	}
	category := sources.NormalizeCategory(req.Category)
	keyCategory := normalizeKeyCategory(req.KeyCategory)
	// Category upserts intentionally remain before the source/key transaction.
	if err := a.upsertExternalSourceCategory(category); err != nil {
		return externalSourceRow{}, errSourceCategory
	}
	if err := a.ensureSourceUpdateKeyCategory(keyCategory); err != nil {
		return externalSourceRow{}, err
	}
	return externalSourceRow{
		ID: current.ID, Name: name, SourceURL: sourceURL, Category: category, KeyCategory: keyCategory,
		KeyInsertMode: sources.NormalizeKeyInsertMode(req.KeyInsertMode), Enabled: req.Enabled,
		ApplyRemoteMetadata: req.ApplyRemoteMetadata, PassHWID: req.PassHWID,
		HWIDVersion: strings.TrimSpace(req.HWIDVersion), HWIDModelName: strings.TrimSpace(req.HWIDModelName),
		HWIDValue: req.resolveHWID(current.HWIDValue),
	}, nil
}

func (a *App) ensureSourceUpdateKeyCategory(category string) error {
	if category == "" {
		return nil
	}
	if err := a.upsertKeyCategory(category); err != nil {
		return errSourceKeyCategory
	}
	return nil
}

// Clear wins over replacement; absent, null and whitespace retain the secret.
func (req sourceV1UpdateRequest) resolveHWID(current string) string {
	if req.ClearHWIDValue {
		return ""
	}
	if req.HWIDValue == nil {
		return current
	}
	value := strings.TrimSpace(*req.HWIDValue)
	if value == "" {
		return current
	}
	return value
}
