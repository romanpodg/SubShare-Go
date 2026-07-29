package main

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type bufferedAPIResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newBufferedAPIResponse() *bufferedAPIResponse {
	return &bufferedAPIResponse{header: make(http.Header), status: http.StatusOK}
}

func (response *bufferedAPIResponse) Header() http.Header {
	return response.header
}

func (response *bufferedAPIResponse) WriteHeader(status int) {
	if response.status == http.StatusOK {
		response.status = status
	}
}

func (response *bufferedAPIResponse) Write(payload []byte) (int, error) {
	return response.body.Write(payload)
}

// v1Compatibility gives migrated legacy handlers the v1 error contract while
// their successful response payload remains backward compatible.
func (a *App) v1Compatibility(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buffered := newBufferedAPIResponse()
		next.ServeHTTP(buffered, r)
		if buffered.status >= http.StatusBadRequest {
			var payload struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(buffered.body.Bytes(), &payload)
			message := payload.Message
			if message == "" {
				message = payload.Error
			}
			if message == "" {
				message = http.StatusText(buffered.status)
			}
			writeV1Error(w, r, buffered.status, "compatibility_error", message)
			return
		}
		for key, values := range buffered.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(buffered.status)
		_, _ = w.Write(buffered.body.Bytes())
	})
}
