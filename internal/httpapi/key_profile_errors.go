package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/middleware"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func writeV1Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID, _ := r.Context().Value(middleware.CtxKeyRequestID).(string)
	writeJSON(w, status, map[string]any{
		"error":        message,
		"code":         code,
		"message":      message,
		"field_errors": map[string][]string{},
		"request_id":   requestID,
	})
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

func readJSONWithLimit(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON value")
		}
		return err
	}
	return nil
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
		writeV1Error(w, r, http.StatusBadRequest, "invalid_profile_uri", keymanagement.InvalidProfileURIMessage(err))
		return
	}
	if errors.Is(err, keymanagement.ErrProfileCreateConflict) {
		writeV1Error(w, r, http.StatusConflict, "create_failed", "failed to create key (maybe duplicate)")
		return
	}
	writeV1Error(w, r, http.StatusInternalServerError, defaultCode, defaultMsg)
}
