package main

import (
	"net/http"
	"testing"
)

type responsePermissionRoute struct {
	method, path string
	input        any
	status       int
}

func TestResponsePolicyRegisteredPermissions(t *testing.T) {
	template := templateInput{Name: "Fixture", Format: "plain", Enabled: true}
	rule := responsePolicyInput()
	routes := []responsePermissionRoute{
		{http.MethodGet, "/api/v1/templates", nil, http.StatusOK},
		{http.MethodPost, "/api/v1/templates/preview", template, http.StatusOK},
		{http.MethodPost, "/api/v1/templates", template, http.StatusCreated},
		{http.MethodPut, "/api/v1/templates/900000", template, http.StatusNotFound},
		{http.MethodDelete, "/api/v1/templates/900000", nil, http.StatusNotFound},
		{http.MethodGet, "/api/v1/response-rules", nil, http.StatusOK},
		{http.MethodPost, "/api/v1/response-rules", rule, http.StatusCreated},
		{http.MethodPut, "/api/v1/response-rules/900000", rule, http.StatusNotFound},
		{http.MethodDelete, "/api/v1/response-rules/900000", nil, http.StatusNotFound},
	}
	for _, role := range []string{"", "viewer", "operator", "super_admin", "owner"} {
		t.Run("role-"+role, func(t *testing.T) {
			f := responsePermissionFixture(t, role)
			for _, route := range routes {
				t.Run(route.method+route.path, func(t *testing.T) {
					recorder := f.request(route.method, route.path, mutationJSON(t, route.input))
					status, code := responsePermissionWant(role, route)
					requireRepositoryEqual(t, "route authorization status", recorder.Code, status)
					if code != "" {
						requireRepositoryEqual(t, "route authorization code", decodeJSONMap(t, recorder)["code"], code)
					}
				})
			}
		})
	}
}

func responsePermissionFixture(t *testing.T, role string) userMutationFixture {
	t.Helper()
	f := newUserMutationFixture(t)
	if role == "" {
		f.session, f.csrf = "", ""
	} else if role != "owner" {
		f.session, f.csrf, _ = seedIntegrationSession(t, f.app, role)
	}
	return f
}

func responsePermissionWant(role string, route responsePermissionRoute) (int, string) {
	if role == "" {
		return http.StatusUnauthorized, "unauthorized"
	}
	if route.method == http.MethodGet {
		return route.status, ""
	}
	if role == "owner" || role == "super_admin" {
		return route.status, ""
	}
	return http.StatusForbidden, "owner_required"
}

func TestResponsePolicyRegisteredCSRFPrecedesMutation(t *testing.T) {
	f := newUserMutationFixture(t)
	f.csrf = "incorrect-fixture"
	for _, path := range []string{"/api/v1/templates", "/api/v1/templates/preview", "/api/v1/response-rules"} {
		recorder := f.request(http.MethodPost, path, "{}")
		assertResponseRouteError(t, recorder, responseRouteErrorWant{http.StatusForbidden, "csrf_invalid", "invalid CSRF token"})
	}
}
