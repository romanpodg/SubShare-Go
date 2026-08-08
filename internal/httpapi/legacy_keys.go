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

func writeMessage(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusOK, map[string]any{"message": msg})
}

func readJSON(r *http.Request, dst any) error {
	return readJSONWithLimit(nil, r, dst, 1<<20)
}

func (h *LegacyKeyHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req model.CreateKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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
		mapLegacyCreateError(w, err)
		return
	}

	if h.audit != nil {
		h.audit(r, "key.create", "key", strconv.FormatInt(keyID, 10), map[string]any{"label": label})
	}
	writeMessage(w, "key added")
}

func (h *LegacyKeyHandler) UpdateKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateKeyRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
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
		mapLegacyUpdateError(w, err)
		return
	}

	if h.audit != nil {
		h.audit(r, "key.update", "key", strconv.FormatInt(id, 10), map[string]any{"label": label})
	}
	writeMessage(w, "key updated")
}

func (h *LegacyKeyHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}

	err := h.service.DeleteLegacy(r.Context(), id)
	if err != nil {
		if errors.Is(err, keymanagement.ErrKeyNotFound) {
			writeError(w, http.StatusNotFound, "key not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete key")
		return
	}

	if h.audit != nil {
		h.audit(r, "key.delete", "key", strconv.FormatInt(id, 10), nil)
	}
	writeMessage(w, "key deleted")
}

func mapLegacyCreateError(w http.ResponseWriter, err error) {
	if errors.Is(err, keymanagement.ErrInvalidKeyKind) {
		writeError(w, http.StatusBadRequest, "invalid key kind")
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidKeyStatus) {
		writeError(w, http.StatusBadRequest, "invalid key status")
		return
	}
	if errors.Is(err, keymanagement.ErrLabelRequired) {
		writeError(w, http.StatusBadRequest, "label is required")
		return
	}
	if errors.Is(err, keymanagement.ErrURLRequired) {
		writeError(w, http.StatusBadRequest, "url is required for real keys")
		return
	}
	if errors.Is(err, keymanagement.ErrLabelTooLong) {
		writeError(w, http.StatusBadRequest, "label is too long (max 255 characters)")
		return
	}
	if errors.Is(err, keymanagement.ErrConfigurationTooLong) {
		writeError(w, http.StatusBadRequest, "configuration is too long (max 65535 characters)")
		return
	}
	if errors.Is(err, keymanagement.ErrTemplateTextTooLong) {
		writeError(w, http.StatusBadRequest, "template_text is too long (max 8192 characters)")
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidProfileURI) {
		writeError(w, http.StatusBadRequest, keymanagement.InvalidProfileURIMessage(err))
		return
	}
	if errors.Is(err, keymanagement.ErrKeyCreateConflict) {
		writeError(w, http.StatusConflict, "failed to add key (maybe duplicate)")
		return
	}
	if errors.Is(err, keymanagement.ErrCategoryPersistence) {
		writeError(w, http.StatusInternalServerError, "failed to save key category")
		return
	}
	if errors.Is(err, keymanagement.ErrKeyOrderPersistence) {
		writeError(w, http.StatusInternalServerError, "failed to prepare key order")
		return
	}
	if errors.Is(err, keymanagement.ErrEncryptionUnavailable) {
		writeError(w, http.StatusInternalServerError, "encryption key unavailable")
		return
	}
	if errors.Is(err, keymanagement.ErrBlindIndexUnavailable) {
		writeError(w, http.StatusInternalServerError, "blind index key unavailable")
		return
	}
	if errors.Is(err, keymanagement.ErrCredentialEncryption) {
		writeError(w, http.StatusInternalServerError, "failed to encrypt key")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to add key")
}

func mapLegacyUpdateError(w http.ResponseWriter, err error) {
	if errors.Is(err, keymanagement.ErrKeyNotFound) {
		writeError(w, http.StatusNotFound, "key not found")
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidKeyKind) {
		writeError(w, http.StatusBadRequest, "invalid key kind")
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidKeyStatus) {
		writeError(w, http.StatusBadRequest, "invalid key status")
		return
	}
	if errors.Is(err, keymanagement.ErrLabelRequired) {
		writeError(w, http.StatusBadRequest, "label is required")
		return
	}
	if errors.Is(err, keymanagement.ErrLabelTooLong) {
		writeError(w, http.StatusBadRequest, "label is too long (max 255 characters)")
		return
	}
	if errors.Is(err, keymanagement.ErrConfigurationTooLong) {
		writeError(w, http.StatusBadRequest, "configuration is too long (max 65535 characters)")
		return
	}
	if errors.Is(err, keymanagement.ErrTemplateTextTooLong) {
		writeError(w, http.StatusBadRequest, "template_text is too long (max 8192 characters)")
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidProfileURI) {
		writeError(w, http.StatusBadRequest, keymanagement.InvalidProfileURIMessage(err))
		return
	}
	if errors.Is(err, keymanagement.ErrCategoryPersistence) {
		writeError(w, http.StatusInternalServerError, "failed to save key category")
		return
	}
	if errors.Is(err, keymanagement.ErrKeyOrderPersistence) {
		writeError(w, http.StatusInternalServerError, "failed to prepare key order")
		return
	}
	if errors.Is(err, keymanagement.ErrEncryptionUnavailable) {
		writeError(w, http.StatusInternalServerError, "encryption key unavailable")
		return
	}
	if errors.Is(err, keymanagement.ErrBlindIndexUnavailable) {
		writeError(w, http.StatusInternalServerError, "blind index key unavailable")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to update key")
}
