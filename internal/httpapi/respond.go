package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/romanpodg/SubShare-Go/internal/middleware"
)

// The API has two wire shapes for errors. The legacy shape is {"error": msg}.
// The v1 shape adds code, message, field_errors and request_id. Which one a
// response gets is decided once, here, from a marker the route sets; handlers
// only ever call WriteError / WriteV1Error.

type envelopeKey struct{}

// V1Envelope marks every request through next so that legacy-style errors are
// written in the v1 envelope. Successful payloads are untouched.
func V1Envelope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), envelopeKey{}, true)))
	})
}

func wantsV1Envelope(r *http.Request) bool {
	if r == nil {
		return false
	}
	marked, _ := r.Context().Value(envelopeKey{}).(bool)
	return marked
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteMessage(w http.ResponseWriter, msg string) {
	WriteJSON(w, http.StatusOK, map[string]any{"message": msg})
}

// WriteError writes a legacy-shaped error, or the v1 envelope with code
// "compatibility_error" when the route is marked with V1Envelope.
func WriteError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	if wantsV1Envelope(r) {
		WriteV1Error(w, r, status, "compatibility_error", msg)
		return
	}
	WriteJSON(w, status, map[string]any{"error": msg})
}

func WriteV1Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	WriteFieldError(w, r, status, code, message, "")
}

// WriteFieldError writes the v1 envelope; a non-empty field attributes the
// message to that request field.
func WriteFieldError(w http.ResponseWriter, r *http.Request, status int, code, message, field string) {
	requestID := ""
	if r != nil {
		requestID, _ = r.Context().Value(middleware.CtxKeyRequestID).(string)
	}
	fieldErrors := map[string][]string{}
	if field != "" {
		fieldErrors[field] = []string{message}
	}
	WriteJSON(w, status, map[string]any{
		"error":        message,
		"code":         code,
		"message":      message,
		"field_errors": fieldErrors,
		"request_id":   requestID,
	})
}

// PathID parses a positive integer path value or writes a 400.
func PathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := r.PathValue(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, r, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return id, true
}

// ReadJSON decodes exactly one JSON value (unknown fields rejected) of at most 1 MiB.
func ReadJSON(r *http.Request, dst any) error {
	return ReadJSONWithLimit(nil, r, dst, 1<<20)
}

func ReadJSONWithLimit(w http.ResponseWriter, r *http.Request, dst any, limit int64) error {
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
