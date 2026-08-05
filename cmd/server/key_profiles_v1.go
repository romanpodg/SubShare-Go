package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func sanitizeCheckError(raw string) model.SanitizedCheckError {
	return keymanagement.SanitizeCheckError(raw)
}

func applyNoStoreHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Pragma", "no-cache")
}

func applyStrictNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func (a *App) buildSafeStructuredProfile(parsed *profiles.Profile) *model.SafeStructuredProfile {
	return keymanagement.BuildSafeStructuredProfile(parsed)
}

func (a *App) buildCapabilitiesMap(parsed *profiles.Profile) map[string]map[string]any {
	caps := make(map[string]map[string]any)
	if parsed == nil {
		return caps
	}
	matrix := subscriptionCapabilityMatrix()
	for _, item := range matrix {
		if strings.EqualFold(item.Protocol, string(parsed.Protocol)) {
			for fmtName, outCap := range item.Outputs {
				payload := map[string]any{
					"status": string(outCap.Status),
				}
				if outCap.ReasonCode != "" {
					payload["reason_code"] = outCap.ReasonCode
				}
				if outCap.TargetVersion != "" {
					payload["target_version"] = outCap.TargetVersion
				}
				caps[fmtName] = payload
			}

			// Apply TUIC v4 overrides
			if parsed.Protocol == profiles.ProtocolTUIC && parsed.Data != nil {
				if tuicData, ok := parsed.Data.(profiles.TUICData); ok && tuicData.Generation == 4 {
					caps["mihomo"] = map[string]any{"status": string(capabilityUnsupported), "reason_code": generationReasonCompatibility}
					caps["sing-box"] = map[string]any{"status": string(capabilityUnsupported), "reason_code": generationReasonCompatibility}
					caps["xray-json"] = map[string]any{"status": string(capabilityUnsupported), "reason_code": generationReasonCompatibility}
					caps["plain"] = map[string]any{"status": string(capabilityCompatibility), "reason_code": "raw_delivery_only"}
					caps["base64"] = map[string]any{"status": string(capabilityCompatibility), "reason_code": "raw_delivery_only"}
				}
			}
			break
		}
	}
	if len(caps) == 0 {
		caps["plain"] = map[string]any{"status": "supported"}
		caps["base64"] = map[string]any{"status": "supported"}
	}
	return caps
}

func (a *App) buildKeyProfileDetailResponse(key model.VLESSKey, decryptedURI string) model.KeyProfileDetailResponse {
	return keymanagement.BuildKeyProfileDetailResponse(key, decryptedURI, a.buildCapabilitiesMap)
}

func (a *App) fetchKeyByID(id int64) (*model.VLESSKey, string, error) {
	repo := storage.NewProfileRepository(a.db, a.profileKeyring)
	key, decURI, err := repo.GetByID(context.Background(), id)
	if errors.Is(err, profilepersistence.ErrProfileNotFound) || errors.Is(err, keymanagement.ErrKeyNotFound) {
		return nil, "", sql.ErrNoRows
	}
	return key, decURI, err
}

func (a *App) keyService() *keymanagement.Service {
	repo := storage.NewProfileRepository(a.db, a.profileKeyring)
	return keymanagement.NewService(repo, a.buildCapabilitiesMap)
}

func mapServiceError(w http.ResponseWriter, r *http.Request, err error, defaultCode string, defaultMsg string) {
	if errors.Is(err, keymanagement.ErrKeyNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "key_not_found", "key not found")
		return
	}
	if errors.Is(err, keymanagement.ErrProfileRevisionConflict) {
		writeV1Error(w, r, http.StatusConflict, "profile_revision_conflict", "profile revision conflict")
		return
	}
	if errors.Is(err, keymanagement.ErrSourceOwnedReadOnly) {
		writeV1Error(w, r, http.StatusForbidden, "source_owned_read_only", "profile material for source-owned keys is read-only")
		return
	}
	if errors.Is(err, keymanagement.ErrStorageIntegrity) {
		writeV1Error(w, r, http.StatusInternalServerError, "storage_integrity_error", "profile storage integrity error")
		return
	}
	if errors.Is(err, keymanagement.ErrEncryptionUnavailable) {
		writeV1Error(w, r, http.StatusInternalServerError, "encryption_unavailable", "encryption key unavailable")
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidInput) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrLabelTooLong) {
		writeV1Error(w, r, http.StatusBadRequest, "label_too_long", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrRawURIRequired) {
		writeV1Error(w, r, http.StatusBadRequest, "raw_uri_required", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrStructuredPayloadRequired) {
		writeV1Error(w, r, http.StatusBadRequest, "structured_payload_required", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrStructuredPatchRequired) {
		writeV1Error(w, r, http.StatusBadRequest, "structured_patch_required", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrMutuallyExclusiveMode) {
		writeV1Error(w, r, http.StatusBadRequest, "mutually_exclusive_mode", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidCreationMode) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_creation_mode", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidPatchMode) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_patch_mode", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrTUICv4StructuredForbidden) {
		writeV1Error(w, r, http.StatusBadRequest, "tuic_v4_structured_forbidden", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInformationalStructuredForbidden) {
		writeV1Error(w, r, http.StatusBadRequest, "informational_structured_forbidden", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidTarget) {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_target", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidProfileURI) {
		msg := err.Error()
		if idx := strings.Index(msg, ": "); idx != -1 {
			msg = msg[idx+2:]
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_profile_uri", msg)
		return
	}
	if strings.Contains(err.Error(), "create_failed") {
		writeV1Error(w, r, http.StatusConflict, "create_failed", "failed to create key (maybe duplicate)")
		return
	}
	writeV1Error(w, r, http.StatusInternalServerError, defaultCode, defaultMsg)
}

func (a *App) apiV1GetKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyNoStoreHeaders(w)

	detail, err := a.keyService().GetDetail(r.Context(), id)
	if err != nil {
		mapServiceError(w, r, err, "key_load_failed", "failed to load key")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func (a *App) apiV1RevealKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyStrictNoCacheHeaders(w)

	var req model.KeySecretRevealRequest
	if err := readJSON(r, &req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	result, err := a.keyService().Reveal(r.Context(), keymanagement.RevealParams{
		ID:              id,
		Target:          req.Target,
		ProfileRevision: req.ProfileRevision,
	})
	if err != nil {
		mapServiceError(w, r, err, "key_load_failed", "failed to load key")
		return
	}

	a.recordAuditEvent(r, "key.reveal", "key", strconv.FormatInt(id, 10), map[string]any{
		"target": req.Target,
	})

	if result.Target == "raw" {
		writeJSON(w, http.StatusOK, map[string]any{
			"data": model.KeyRawSecretResponse{
				KeyID:                    id,
				ConfirmedProfileRevision: result.ConfirmedProfileRevision,
				Target:                   "raw",
				RawURI:                   result.RawURI,
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": model.KeyStructuredSecretsResponse{
			KeyID:                    id,
			ConfirmedProfileRevision: result.ConfirmedProfileRevision,
			Target:                   "structured-secrets",
			Secrets:                  result.Secrets,
		},
	})
}

func (a *App) apiV1CloneKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyNoStoreHeaders(w)

	var req model.KeyCloneRequest
	if err := readJSON(r, &req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	detail, err := a.keyService().CloneAsLocal(r.Context(), keymanagement.CloneParams{
		ID:                      id,
		ExpectedProfileRevision: req.ExpectedProfileRevision,
		NewLabel:                req.NewLabel,
	})
	if err != nil {
		mapServiceError(w, r, err, "clone_failed", "failed to clone key")
		return
	}

	a.recordAuditEvent(r, "key.clone", "key", strconv.FormatInt(id, 10), map[string]any{
		"source_key_id": id,
		"new_key_id":    detail.ID,
	})

	writeJSON(w, http.StatusCreated, map[string]any{"data": detail})
}

func (a *App) apiV1GetKeyEditorSchema(w http.ResponseWriter, r *http.Request) {
	applyNoStoreHeaders(w)
	schema := a.keyService().EditorSchema()
	writeJSON(w, http.StatusOK, map[string]any{
		"data": schema,
	})
}

func (a *App) apiV1CreateKeyProfile(w http.ResponseWriter, r *http.Request) {
	applyNoStoreHeaders(w)

	var req model.CreateKeyProfileRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid or unallowed JSON request body")
		return
	}

	detail, err := a.keyService().CreateLocal(r.Context(), keymanagement.CreateLocalParams{
		Label:        req.Label,
		Status:       req.Status,
		Kind:         req.Kind,
		Category:     req.Category,
		CategoryID:   req.CategoryID,
		TemplateText: req.TemplateText,
		CreationMode: req.CreationMode,
		RawURI:       req.RawURI,
		Protocol:     req.Protocol,
		Structured:   req.Structured,
	})
	if err != nil {
		mapServiceError(w, r, err, "create_failed", "failed to create key")
		return
	}

	a.recordAuditEvent(r, "key.create", "key", strconv.FormatInt(detail.ID, 10), map[string]any{"label": detail.Label})

	writeJSON(w, http.StatusCreated, map[string]any{"data": detail})
}

func (a *App) buildURIFromStructuredCreate(proto, label string, patch *model.StructuredProfilePatch) (string, error) {
	return keymanagement.BuildURIFromStructuredCreate(proto, label, patch)
}

func (a *App) apiV1UpdateKeyProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyNoStoreHeaders(w)

	var req model.UpdateKeyProfileRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid or unallowed JSON request body")
		return
	}

	detail, err := a.keyService().UpdateLocal(r.Context(), keymanagement.UpdateLocalParams{
		ID:              id,
		ProfileRevision: req.ProfileRevision,
		Label:           req.Label,
		Status:          req.Status,
		Kind:            req.Kind,
		Category:        req.Category,
		CategoryID:      req.CategoryID,
		TemplateText:    req.TemplateText,
		PatchMode:       req.PatchMode,
		RawURI:          req.RawURI,
		StructuredPatch: req.StructuredPatch,
	})
	if err != nil {
		mapServiceError(w, r, err, "update_failed", "failed to update key")
		return
	}

	a.recordAuditEvent(r, "key.update", "key", strconv.FormatInt(detail.ID, 10), map[string]any{"label": detail.Label})

	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func (a *App) applyStructuredPatchToURI(protocol, currentURI, label string, patch *model.StructuredProfilePatch) (string, error) {
	return keymanagement.ApplyStructuredPatchToURI(protocol, currentURI, label, patch)
}
