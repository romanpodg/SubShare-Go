package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

type LegacyKeyHandler struct {
	service *keymanagement.Service
	audit   AuditRecorder
}

func NewLegacyKeyHandler(service *keymanagement.Service, audit AuditRecorder) *LegacyKeyHandler {
	return &LegacyKeyHandler{
		service: service,
		audit:   audit,
	}
}

func (h *LegacyKeyHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req model.CreateKeyRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	keyID, label, err := h.service.CreateLegacy(r.Context(), keymanagement.CreateLegacyParams{
		Label:        req.Label,
		URL:          req.URL,
		Category:     req.Category,
		TemplateText: req.TemplateText,
		Kind:         req.Kind,
		Status:       req.Status,
	})
	if err != nil {
		mapLegacyError(w, r, err, "failed to add key")
		return
	}

	if h.audit != nil {
		h.audit(r, "key.create", "key", strconv.FormatInt(keyID, 10), map[string]any{"label": label})
	}
	WriteMessage(w, "key added")
}

func (h *LegacyKeyHandler) UpdateKey(w http.ResponseWriter, r *http.Request) {
	id, ok := PathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateKeyRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	label, err := h.service.UpdateLegacy(r.Context(), id, keymanagement.UpdateLegacyParams{
		Label:        req.Label,
		Category:     req.Category,
		TemplateText: req.TemplateText,
		Kind:         req.Kind,
		Status:       req.Status,
		RawURL:       req.RawURL,
		UUID:         req.UUID,
		Host:         req.Host,
		Port:         req.Port,
		Query:        req.Query,
		Fragment:     req.Fragment,
	})
	if err != nil {
		mapLegacyError(w, r, err, "failed to update key")
		return
	}

	if h.audit != nil {
		h.audit(r, "key.update", "key", strconv.FormatInt(id, 10), map[string]any{"label": label})
	}
	WriteMessage(w, "key updated")
}

func (h *LegacyKeyHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	id, ok := PathID(w, r, "id")
	if !ok {
		return
	}

	err := h.service.DeleteLegacy(r.Context(), id)
	if err != nil {
		if errors.Is(err, keymanagement.ErrKeyNotFound) {
			WriteError(w, r, http.StatusNotFound, "key not found")
			return
		}
		WriteError(w, r, http.StatusInternalServerError, "failed to delete key")
		return
	}

	if h.audit != nil {
		h.audit(r, "key.delete", "key", strconv.FormatInt(id, 10), nil)
	}
	WriteMessage(w, "key deleted")
}

type legacyErrorMapping struct {
	target  error
	status  int
	message func(error) string
}

func static(message string) func(error) string { return func(error) string { return message } }

// legacyErrorMappings is the single ordered table for legacy key CRUD errors.
var legacyErrorMappings = []legacyErrorMapping{
	{keymanagement.ErrKeyNotFound, http.StatusNotFound, static("key not found")},
	{keymanagement.ErrInvalidKeyKind, http.StatusBadRequest, static("invalid key kind")},
	{keymanagement.ErrInvalidKeyStatus, http.StatusBadRequest, static("invalid key status")},
	{keymanagement.ErrLabelRequired, http.StatusBadRequest, static("label is required")},
	{keymanagement.ErrURLRequired, http.StatusBadRequest, static("url is required for real keys")},
	{keymanagement.ErrLabelTooLong, http.StatusBadRequest, static("label is too long (max 255 characters)")},
	{keymanagement.ErrConfigurationTooLong, http.StatusBadRequest, static("configuration is too long (max 65535 characters)")},
	{keymanagement.ErrTemplateTextTooLong, http.StatusBadRequest, static("template_text is too long (max 8192 characters)")},
	{keymanagement.ErrInvalidProfileURI, http.StatusBadRequest, keymanagement.InvalidProfileURIMessage},
	{keymanagement.ErrKeyCreateConflict, http.StatusConflict, static("failed to add key (maybe duplicate)")},
	{keymanagement.ErrCategoryPersistence, http.StatusInternalServerError, static("failed to save key category")},
	{keymanagement.ErrKeyOrderPersistence, http.StatusInternalServerError, static("failed to prepare key order")},
	{keymanagement.ErrEncryptionUnavailable, http.StatusInternalServerError, static("encryption key unavailable")},
	{keymanagement.ErrBlindIndexUnavailable, http.StatusInternalServerError, static("blind index key unavailable")},
	{keymanagement.ErrCredentialEncryption, http.StatusInternalServerError, static("failed to encrypt key")},
}

// mapLegacyError writes the legacy-shaped error for err, or fallback with 500.
func mapLegacyError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	for _, mapping := range legacyErrorMappings {
		if errors.Is(err, mapping.target) {
			WriteError(w, r, mapping.status, mapping.message(err))
			return
		}
	}
	WriteError(w, r, http.StatusInternalServerError, fallback)
}
