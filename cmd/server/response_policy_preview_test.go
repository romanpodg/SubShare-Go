package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
)

const responsePreviewURI = "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&sni=edge.example.com&type=ws&path=%2Fws#Example"

func TestResponsePolicyRegisteredPreviewFormats(t *testing.T) {
	f := newUserMutationFixture(t)
	cases := []struct{ format, contentType string }{
		{"plain", "text/plain; charset=utf-8"},
		{"base64", "text/plain; charset=utf-8"},
		{"mihomo", "application/yaml; charset=utf-8"},
		{"sing-box", "application/json; charset=utf-8"},
		{"xray-json", "application/json; charset=utf-8"},
	}
	for _, test := range cases {
		t.Run(test.format, func(t *testing.T) {
			input := templateInput{Name: "Preview", Format: test.format}
			recorder := f.request(http.MethodPost, "/api/v1/templates/preview", mutationJSON(t, input))
			requireRepositoryEqual(t, "preview status", recorder.Code, http.StatusOK)
			payload := decodeJSONMap(t, recorder)
			requireRepositoryEqual(t, "preview envelope", len(payload), 2)
			requireRepositoryEqual(t, "preview content type", payload["content_type"], test.contentType)
			assertResponsePreviewBody(t, test.format, payload["content"].(string))
		})
	}
}

func assertResponsePreviewBody(t *testing.T, format, body string) {
	t.Helper()
	switch format {
	case "plain":
		requireRepositoryEqual(t, "plain preview bytes", body, responsePreviewURI)
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(body)
		requireRepositorySuccess(t, err)
		requireRepositoryEqual(t, "base64 preview bytes", string(decoded), responsePreviewURI)
	case "xray-json":
		requireRepositoryEqual(t, "Xray preview bytes", body, `[{"outbounds":[{"protocol":"vless","tag":"Example"}]}]`)
	default:
		assertResponsePreviewStructuredNode(t, format, body)
	}
}

func assertResponsePreviewStructuredNode(t *testing.T, format, body string) {
	t.Helper()
	var document map[string]any
	requireRepositorySuccess(t, json.Unmarshal([]byte(body), &document))
	key := "outbounds"
	if format == "mihomo" {
		key = "proxies"
	}
	nodes := document[key].([]any)
	requireRepositoryEqual(t, "preview node count", len(nodes), 1)
	node := nodes[0].(map[string]any)
	requireRepositoryEqual(t, "preview node protocol", node["type"], "vless")
	requireRepositoryEqual(t, "preview node server", node["server"], "example.com")
}

func TestResponsePolicyRegisteredPreviewErrors(t *testing.T) {
	f := newUserMutationFixture(t)
	cases := []struct {
		name, body string
		want       responseRouteErrorWant
	}{
		{"malformed", "{", responseRouteErrorWant{http.StatusBadRequest, "invalid_body", "invalid request body"}},
		{"unknown field", `{"name":"Preview","format":"plain","unknown":true}`, responseRouteErrorWant{http.StatusBadRequest, "invalid_body", "invalid request body"}},
		{"invalid name", `{"name":" ","format":"plain"}`, responseRouteErrorWant{http.StatusBadRequest, "template_invalid", "name must contain 1..80 characters"}},
		{"invalid format", `{"name":"Preview","format":"unknown"}`, responseRouteErrorWant{http.StatusBadRequest, "template_invalid", "unsupported template format"}},
		{"invalid rendered JSON", `{"name":"Preview","format":"sing-box","content":"{broken {{subscription}}"}`, responseRouteErrorWant{http.StatusUnprocessableEntity, "template_render_invalid", "rendered preview is not valid sing-box"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertResponseRouteError(t, f.request(http.MethodPost, "/api/v1/templates/preview", test.body), test.want)
		})
	}
}
