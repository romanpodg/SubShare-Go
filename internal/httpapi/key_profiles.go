package httpapi

import (
	"net/http"
	"strconv"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

type AuditRecorder func(r *http.Request, eventName, entityType, entityID string, metadata map[string]any)

type KeyProfileHandler struct {
	service *keymanagement.Service
	audit   AuditRecorder
}

func NewKeyProfileHandler(service *keymanagement.Service, audit AuditRecorder) *KeyProfileHandler {
	return &KeyProfileHandler{
		service: service,
		audit:   audit,
	}
}

func (h *KeyProfileHandler) GetKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyNoStoreHeaders(w)

	detail, err := h.service.GetDetail(r.Context(), id)
	if err != nil {
		mapServiceError(w, r, err, "key_load_failed", "failed to load key")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}

func (h *KeyProfileHandler) RevealKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyStrictNoCacheHeaders(w)

	var req model.KeySecretRevealRequest
	if err := readJSONWithLimit(w, r, &req, 1<<20); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	result, err := h.service.Reveal(r.Context(), keymanagement.RevealParams{
		ID:              id,
		Target:          req.Target,
		ProfileRevision: req.ProfileRevision,
	})
	if err != nil {
		mapServiceError(w, r, err, "key_load_failed", "failed to load key")
		return
	}

	if h.audit != nil {
		h.audit(r, "key.reveal", "key", strconv.FormatInt(id, 10), map[string]any{
			"target": req.Target,
		})
	}

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

func (h *KeyProfileHandler) CloneKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyNoStoreHeaders(w)

	var req model.KeyCloneRequest
	if err := readJSONWithLimit(w, r, &req, 1<<20); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	detail, err := h.service.CloneAsLocal(r.Context(), keymanagement.CloneParams{
		ID:                      id,
		ExpectedProfileRevision: req.ExpectedProfileRevision,
		NewLabel:                req.NewLabel,
	})
	if err != nil {
		mapServiceError(w, r, err, "clone_failed", "failed to clone key")
		return
	}

	if h.audit != nil {
		h.audit(r, "key.clone", "key", strconv.FormatInt(id, 10), map[string]any{
			"source_key_id": id,
			"new_key_id":    detail.ID,
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": detail})
}

func (h *KeyProfileHandler) GetKeyEditorSchema(w http.ResponseWriter, r *http.Request) {
	applyNoStoreHeaders(w)
	schema := h.service.EditorSchema()
	writeJSON(w, http.StatusOK, map[string]any{
		"data": schema,
	})
}

func (h *KeyProfileHandler) CreateKeyProfile(w http.ResponseWriter, r *http.Request) {
	applyNoStoreHeaders(w)

	var req model.CreateKeyProfileRequest
	if err := readJSONWithLimit(w, r, &req, 1<<20); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid or unallowed JSON request body")
		return
	}

	detail, err := h.service.CreateLocal(r.Context(), keymanagement.CreateLocalParams{
		Label:             req.Label,
		ClientDisplayName: req.ClientDisplayName,
		Status:            req.Status,
		Kind:              req.Kind,
		Category:          req.Category,
		CategoryID:        req.CategoryID,
		TemplateText:      req.TemplateText,
		CreationMode:      req.CreationMode,
		RawURI:            req.RawURI,
		Protocol:          req.Protocol,
		Structured:        req.Structured,
	})
	if err != nil {
		mapServiceError(w, r, err, "create_failed", "failed to create key")
		return
	}

	if h.audit != nil {
		h.audit(r, "key.create", "key", strconv.FormatInt(detail.ID, 10), map[string]any{"label": detail.Label})
	}

	writeJSON(w, http.StatusCreated, map[string]any{"data": detail})
}

func (h *KeyProfileHandler) UpdateKeyProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	applyNoStoreHeaders(w)

	var req model.UpdateKeyProfileRequest
	if err := readJSONWithLimit(w, r, &req, 1<<20); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid or unallowed JSON request body")
		return
	}

	detail, err := h.service.UpdateLocal(r.Context(), keymanagement.UpdateLocalParams{
		ID:                id,
		ProfileRevision:   req.ProfileRevision,
		Label:             req.Label,
		ClientDisplayName: req.ClientDisplayName,
		Status:            req.Status,
		Kind:              req.Kind,
		Category:          req.Category,
		CategoryID:        req.CategoryID,
		TemplateText:      req.TemplateText,
		PatchMode:         req.PatchMode,
		RawURI:            req.RawURI,
		StructuredPatch:   req.StructuredPatch,
	})
	if err != nil {
		mapServiceError(w, r, err, "update_failed", "failed to update key")
		return
	}

	if h.audit != nil {
		h.audit(r, "key.update", "key", strconv.FormatInt(detail.ID, 10), map[string]any{"label": detail.Label})
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": detail})
}
