package httpapi

import (
	"errors"
	"net/http"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
)

func mapServiceError(w http.ResponseWriter, r *http.Request, err error, defaultCode string, defaultMsg string) {
	if errors.Is(err, keymanagement.ErrKeyNotFound) {
		WriteV1Error(w, r, http.StatusNotFound, "key_not_found", "key not found")
		return
	}
	if errors.Is(err, keymanagement.ErrProfileRevisionConflict) {
		WriteV1Error(w, r, http.StatusConflict, "profile_revision_conflict", "profile revision conflict")
		return
	}
	if errors.Is(err, keymanagement.ErrSourceOwnedReadOnly) {
		WriteV1Error(w, r, http.StatusForbidden, "source_owned_read_only", "profile material for source-owned keys is read-only")
		return
	}
	if errors.Is(err, keymanagement.ErrStorageIntegrity) {
		WriteV1Error(w, r, http.StatusInternalServerError, "storage_integrity_error", "profile storage integrity error")
		return
	}
	if errors.Is(err, keymanagement.ErrEncryptionUnavailable) {
		WriteV1Error(w, r, http.StatusInternalServerError, "encryption_unavailable", "encryption key unavailable")
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidInput) {
		WriteV1Error(w, r, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrLabelTooLong) {
		WriteV1Error(w, r, http.StatusBadRequest, "label_too_long", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrRawURIRequired) {
		WriteV1Error(w, r, http.StatusBadRequest, "raw_uri_required", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrStructuredPayloadRequired) {
		WriteV1Error(w, r, http.StatusBadRequest, "structured_payload_required", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrStructuredPatchRequired) {
		WriteV1Error(w, r, http.StatusBadRequest, "structured_patch_required", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrMutuallyExclusiveMode) {
		WriteV1Error(w, r, http.StatusBadRequest, "mutually_exclusive_mode", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidCreationMode) {
		WriteV1Error(w, r, http.StatusBadRequest, "invalid_creation_mode", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidPatchMode) {
		WriteV1Error(w, r, http.StatusBadRequest, "invalid_patch_mode", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrTUICv4StructuredForbidden) {
		WriteV1Error(w, r, http.StatusBadRequest, "tuic_v4_structured_forbidden", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInformationalStructuredForbidden) {
		WriteV1Error(w, r, http.StatusBadRequest, "informational_structured_forbidden", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidTarget) {
		WriteV1Error(w, r, http.StatusBadRequest, "invalid_target", err.Error())
		return
	}
	if errors.Is(err, keymanagement.ErrInvalidProfileURI) {
		WriteV1Error(w, r, http.StatusBadRequest, "invalid_profile_uri", keymanagement.InvalidProfileURIMessage(err))
		return
	}
	if errors.Is(err, keymanagement.ErrProfileCreateConflict) {
		WriteV1Error(w, r, http.StatusConflict, "create_failed", "failed to create key (maybe duplicate)")
		return
	}
	WriteV1Error(w, r, http.StatusInternalServerError, defaultCode, defaultMsg)
}
