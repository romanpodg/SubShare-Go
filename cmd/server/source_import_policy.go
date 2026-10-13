package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/sources"
)

type sourceBodyInput struct {
	Body, ContentType, FinalURL string
}

var errSourceRequestBody = sourceFailure{status: http.StatusBadRequest, code: "validation_failed", message: "invalid request body"}

// parseOrFetchSource parses a pasted raw body when supplied, otherwise fetches
// the source URL. Its metadata belongs to the pasted body as one input.
func (a *App) parseOrFetchSource(sourceURL string, hwidProfile sources.HWIDProfile, body sourceBodyInput) (sources.ParseResult, error) {
	if strings.TrimSpace(body.Body) != "" {
		metadata := sources.Metadata{
			ContentType: strings.TrimSpace(body.ContentType), SourceFinalURL: strings.TrimSpace(body.FinalURL),
		}
		return sources.ParseRawBody(sourceURL, body.Body, metadata, a.externalProfileFingerprintKeys())
	}
	return sources.Fetch(context.Background(), a.sourceClient(), sourceURL, hwidProfile, a.externalProfileFingerprintKeys())
}

func (a *App) prepareSourcePreview(req model.ExternalSourcePreviewRequest) (string, sources.ParseResult, error) {
	sourceURL, err := sources.ValidateURL(req.SourceURL)
	if err != nil {
		return "", sources.ParseResult{}, sourceURLFailure(err)
	}
	hwidProfile := sources.NormalizeHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)
	parsed, err := a.parseOrFetchSource(sourceURL, hwidProfile, sourceBodyInput{req.RawBody, req.RawContentType, req.RawFinalURL})
	if err != nil {
		return "", sources.ParseResult{}, sourceFailure{status: http.StatusBadRequest, code: "source_preview_failed", message: err.Error(), field: "source_url"}
	}
	return sourceURL, parsed, nil
}

func sourceURLFailure(err error) sourceFailure {
	return sourceFailure{status: http.StatusBadRequest, code: "source_url_invalid", message: err.Error(), field: "source_url"}
}

func sourceNameFailure(err error) sourceFailure {
	return sourceFailure{status: http.StatusBadRequest, code: "source_name_invalid", message: err.Error(), field: "name"}
}

// URL validation and duplicate detection precede name validation and fetching.
func (a *App) validateSourceImport(req model.ExternalSourceImportRequest) (externalSourceRow, error) {
	sourceURL, err := sources.ValidateURL(req.SourceURL)
	if err != nil {
		return externalSourceRow{}, sourceURLFailure(err)
	}
	var exists bool
	if err := a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM external_subscription_sources WHERE source_url = ?)`, sourceURL).Scan(&exists); err != nil {
		return externalSourceRow{}, errSourceCreate
	}
	if exists {
		return externalSourceRow{}, errSourceURLConflict
	}
	name, err := sources.ValidateName(req.Name)
	if err != nil {
		return externalSourceRow{}, sourceNameFailure(err)
	}
	hwidProfile := sources.NormalizeHWIDProfile(req.PassHWID, req.HWIDVersion, req.HWIDModelName, req.HWIDValue)
	return externalSourceRow{
		Name: name, Category: sources.NormalizeCategory(req.Category), KeyCategory: normalizeKeyCategory(req.KeyCategory),
		KeyInsertMode: sources.NormalizeKeyInsertMode(req.KeyInsertMode), SourceURL: sourceURL,
		Enabled: req.Enabled, ApplyRemoteMetadata: req.ApplyRemoteMetadata,
		PassHWID: hwidProfile.PassHWID, HWIDVersion: hwidProfile.Version,
		HWIDModelName: hwidProfile.ModelName, HWIDValue: hwidProfile.HWID, ImportStatus: "syncing",
	}, nil
}

func (a *App) prepareSourceImport(req model.ExternalSourceImportRequest) (externalSourceRow, sources.ParseResult, error) {
	source, err := a.validateSourceImport(req)
	if err != nil {
		return externalSourceRow{}, sources.ParseResult{}, err
	}
	hwidProfile := sources.HWIDProfile{PassHWID: source.PassHWID, Version: source.HWIDVersion, ModelName: source.HWIDModelName, HWID: source.HWIDValue}
	parsed, err := a.parseOrFetchSource(source.SourceURL, hwidProfile, sourceBodyInput{req.RawBody, req.RawContentType, req.RawFinalURL})
	if err != nil {
		return externalSourceRow{}, sources.ParseResult{}, sourceFailure{status: http.StatusBadRequest, code: "source_import_failed", message: err.Error(), field: "source_url"}
	}
	parsed, err = sources.FilterSelection(parsed, req.SelectedItemRefs)
	if err != nil {
		return externalSourceRow{}, sources.ParseResult{}, sourceFailure{status: http.StatusBadRequest, code: "source_selection_invalid", message: err.Error(), field: "selected_item_refs"}
	}
	if len(parsed.Keys) == 0 {
		return externalSourceRow{}, sources.ParseResult{}, sourceFailure{status: http.StatusBadRequest, code: "source_empty", message: "no keys to import", field: "source_url"}
	}
	return source, parsed, nil
}
