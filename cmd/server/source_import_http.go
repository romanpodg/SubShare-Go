package main

import (
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"net/http"
	"strconv"
)

func (a *App) apiV1PreviewSource(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourcePreviewRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		writeSourceFailure(w, r, errSourceRequestBody)
		return
	}
	sourceURL, parsed, err := a.prepareSourcePreview(req)
	if err != nil {
		writeSourceFailure(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"source_url": sourceURL, "suggested_name": sources.SuggestName(sourceURL, parsed.Metadata),
		"detected_format": parsed.DetectedFormat, "key_count": len(parsed.Keys),
		"metadata": map[string]any{
			"title": parsed.Metadata.Title, "refresh_hours": parsed.Metadata.RefreshHours,
			"support_url": parsed.Metadata.SupportURL, "profile_web_page": parsed.Metadata.WebPageURL,
			"announce": parsed.Metadata.Announce, "content_type": parsed.Metadata.ContentType,
			"content_disp": parsed.Metadata.ContentDisp, "http_status": parsed.Metadata.HTTPStatusLabel,
			"final_url": parsed.Metadata.SourceFinalURL,
		},
		"warnings": sources.NonNilWarnings(parsed.Warnings), "result_counts": parsed.Counts, "keys": parsed.Items,
	})
}

func (a *App) apiV1CreateSource(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalSourceImportRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		writeSourceFailure(w, r, errSourceRequestBody)
		return
	}
	source, parsed, err := a.prepareSourceImport(req)
	if err != nil {
		writeSourceFailure(w, r, err)
		return
	}
	created, err := a.createExternalSource(source, parsed)
	if err != nil {
		writeSourceFailure(w, r, err)
		return
	}
	sourceID, syncResult := created.ID, created.Sync
	row, err := a.getExternalSourceByID(sourceID)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "source_create_failed", "source created but response could not be loaded")
		return
	}
	a.recordAuditEvent(r, "external_source.create", "external_source", strconv.FormatInt(sourceID, 10), map[string]any{
		"imported_count": syncResult.Imported, "skipped_count": syncResult.Skipped, "result_counts": syncResult.Counts,
	})
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{
		"data": sourceDetailFromRow(row, syncResult.Imported), "imported_count": syncResult.Imported,
		"skipped_count": syncResult.Skipped, "warnings": sources.NonNilWarnings(parsed.Warnings),
		"detected_format": parsed.DetectedFormat, "result_counts": syncResult.Counts, "items": syncResult.Items,
	})
}
