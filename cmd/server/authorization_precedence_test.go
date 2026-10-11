package main

import (
	"net/http"
	"testing"
	"time"
)

type authorizationPrecedenceCase struct {
	name, cookieRole, csrf, path, method, code string
	status                                     int
	expired                                    bool
}

func TestActualRouteCookieBearerPrecedence(t *testing.T) {
	cases := []authorizationPrecedenceCase{
		{name: "unknown cookie blocks bearer", cookieRole: "unknown", path: "/api/v1/dashboard", method: http.MethodGet, status: http.StatusUnauthorized, code: "unauthorized"},
		{name: "expired cookie blocks bearer", cookieRole: "owner", expired: true, path: "/api/v1/dashboard", method: http.MethodGet, status: http.StatusUnauthorized, code: "unauthorized"},
		{name: "empty cookie permits bearer", path: "/api/v1/dashboard", method: http.MethodGet, status: http.StatusOK},
		{name: "viewer cookie wins over owner bearer", cookieRole: "viewer", csrf: "csrf-viewer", path: "/api/v1/users", method: http.MethodPost, status: http.StatusForbidden, code: "viewer_read_only"},
		{name: "operator cookie cannot acquire owner role", cookieRole: "operator", path: "/api/v1/admins", method: http.MethodGet, status: http.StatusForbidden, code: "owner_required"},
		{name: "bearer does not bypass cookie CSRF", cookieRole: "owner", path: "/api/v1/users", method: http.MethodPost, status: http.StatusForbidden, code: "csrf_invalid"},
		{name: "cookie CSRF trims whitespace", cookieRole: "owner", csrf: "  csrf-owner  ", path: "/api/v1/users", method: http.MethodPost, status: http.StatusBadRequest, code: "compatibility_error"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) { assertAuthorizationPrecedence(t, test) })
	}
}

func assertAuthorizationPrecedence(t *testing.T, test authorizationPrecedenceCase) {
	t.Helper()
	f := newUserMutationFixture(t)
	var ownerID int64
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT admin_id FROM admin_sessions WHERE id = ?`, hashSessionID(f.session)).Scan(&ownerID))
	principal := authorizationRoutePrincipal{csrf: test.csrf, bearer: seedAuthorizationRouteToken(t, f.app, authorizationRouteTokenSeed{ownerID, 1, "*"})}
	switch test.cookieRole {
	case "owner":
		principal.cookie = f.session
	case "unknown":
		principal.cookie = "fixture-unknown-cookie"
	case "":
		principal.cookie = ""
		principal.emptyCookie = true
	default:
		principal.cookie, _, _ = seedIntegrationSession(t, f.app, test.cookieRole)
	}
	if test.expired {
		execRepositoryFixtureSQL(t, f.app, `UPDATE admin_sessions SET expires_at = ?`, time.Now().Add(-time.Hour))
	}
	contract := authorizationRouteContract{Pattern: test.method + " " + test.path}
	recorder := authorizationRouteRequest(f, contract, principal, test.method)
	requireRepositoryEqual(t, "cookie/bearer precedence status", recorder.Code, test.status)
	var code string
	if test.status >= http.StatusBadRequest {
		code, _ = decodeJSONMap(t, recorder)["code"].(string)
	}
	requireRepositoryEqual(t, "cookie/bearer precedence code", code, test.code)
	if test.expired {
		requireRepositoryEqual(t, "expired cookie removed", mutationCount(t, f.app, `SELECT COUNT(*) FROM admin_sessions`), 0)
	}
}
