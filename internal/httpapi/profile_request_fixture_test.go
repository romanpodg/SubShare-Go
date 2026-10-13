package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

type profileRequest struct {
	method, path, id, body string
}

func serveProfileRequest(handler http.HandlerFunc, input profileRequest) *httptest.ResponseRecorder {
	var request *http.Request
	if input.body == "" {
		request = httptest.NewRequest(input.method, input.path, nil)
	} else {
		request = httptest.NewRequest(input.method, input.path, strings.NewReader(input.body))
	}
	if input.id != "" {
		request.SetPathValue("id", input.id)
	}
	response := httptest.NewRecorder()
	handler(response, request)
	return response
}
