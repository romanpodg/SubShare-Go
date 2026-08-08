package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdministrationAuthorizationMatrix(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		role       string
		scopes     []string
		csrf       bool
		ownerOnly  bool
		wantStatus int
	}{
		{name: "unauthenticated read", method: http.MethodGet, path: "/api/v1/keys", wantStatus: http.StatusUnauthorized},
		{name: "viewer read", method: http.MethodGet, path: "/api/v1/keys", role: "viewer", wantStatus: http.StatusNoContent},
		{name: "viewer mutation", method: http.MethodPost, path: "/api/v1/keys", role: "viewer", csrf: true, wantStatus: http.StatusForbidden},
		{name: "operator mutation without csrf", method: http.MethodPost, path: "/api/v1/keys", role: "operator", wantStatus: http.StatusForbidden},
		{name: "operator mutation with csrf", method: http.MethodPost, path: "/api/v1/keys", role: "operator", csrf: true, wantStatus: http.StatusNoContent},
		{name: "owner mutation with csrf", method: http.MethodPost, path: "/api/v1/keys", role: "owner", csrf: true, wantStatus: http.StatusNoContent},
		{name: "operator owner-only source mutation", method: http.MethodPost, path: "/api/v1/sources", role: "operator", csrf: true, ownerOnly: true, wantStatus: http.StatusForbidden},
		{name: "owner owner-only source mutation", method: http.MethodPost, path: "/api/v1/sources", role: "owner", csrf: true, ownerOnly: true, wantStatus: http.StatusNoContent},
		{name: "read token read", method: http.MethodGet, path: "/api/v1/keys", scopes: []string{"read"}, wantStatus: http.StatusNoContent},
		{name: "read token mutation", method: http.MethodPost, path: "/api/v1/keys/1/reveal", scopes: []string{"read"}, wantStatus: http.StatusForbidden},
		{name: "keys token mutation", method: http.MethodPost, path: "/api/v1/keys/1/reveal", scopes: []string{"keys:write"}, wantStatus: http.StatusNoContent},
		{name: "settings token key mutation", method: http.MethodPost, path: "/api/v1/keys/1/reveal", scopes: []string{"settings:write"}, wantStatus: http.StatusForbidden},
		{name: "owner keys token source mutation", method: http.MethodPost, path: "/api/v1/sources/1/sync", scopes: []string{"keys:write"}, ownerOnly: true, wantStatus: http.StatusNoContent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			request := httptest.NewRequest(test.method, test.path, nil)
			if test.scopes != nil {
				request.Header.Set("Authorization", "Bearer "+seedIntegrationAPIToken(t, app, test.scopes))
			} else if test.role != "" {
				sessionID, csrfToken, _ := seedIntegrationSession(t, app, test.role)
				request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: sessionID})
				if test.csrf {
					request.Header.Set("X-CSRF-Token", csrfToken)
				}
			}

			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			var handler http.Handler
			if test.ownerOnly {
				handler = app.requireSuperAdmin(next)
			} else {
				handler = app.requireAdmin(next)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != test.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}
}
