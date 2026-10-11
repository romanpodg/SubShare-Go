package main

import (
	"net/http"
	"strings"
)

type authorizationRouteExpectation struct {
	status int
	code   string
}

func authorizationRoleRejection(c authorizationRouteContract, p authorizationRoutePrincipal, unsafe bool) authorizationRouteExpectation {
	switch c.Guard {
	case "owner":
		if p.role != "owner" && p.role != "super_admin" {
			return authorizationRouteExpectation{http.StatusForbidden, "owner_required"}
		}
	case "admin":
		if unsafe && p.role == "viewer" {
			return authorizationRouteExpectation{http.StatusForbidden, "viewer_read_only"}
		}
	}
	return authorizationRouteExpectation{}
}

func authorizationFixtureTokenScopeAllows(c authorizationRouteContract, p authorizationRoutePrincipal) bool {
	if c.Scope == "" {
		return false
	}
	return p.scope == "*" || p.scope == c.Scope
}
func authorizationTokenRejection(c authorizationRouteContract, p authorizationRoutePrincipal) authorizationRouteExpectation {
	if p.bearer == "" {
		return authorizationRouteExpectation{}
	}
	if !authorizationFixtureTokenScopeAllows(c, p) {
		return authorizationRouteExpectation{http.StatusForbidden, "token_scope_forbidden"}
	}
	if c.Interactive {
		return authorizationRouteExpectation{http.StatusForbidden, "interactive_session_required"}
	}
	return authorizationRouteExpectation{}
}

func authorizationCSRFRejection(p authorizationRoutePrincipal, unsafe bool) authorizationRouteExpectation {
	if !unsafe || p.bearer != "" {
		return authorizationRouteExpectation{}
	}
	if p.csrf != "csrf-"+p.role {
		return authorizationRouteExpectation{http.StatusForbidden, "csrf_invalid"}
	}
	return authorizationRouteExpectation{}
}
func authorizationExpectedResponse(c authorizationRouteContract, p authorizationRoutePrincipal, method string) authorizationRouteExpectation {
	permit := authorizationRouteExpectation{c.Status, c.Code}
	if c.Guard == "public" {
		return permit
	}
	if p.cookie == "" && p.bearer == "" {
		return authorizationRouteExpectation{http.StatusUnauthorized, "unauthorized"}
	}
	unsafe := method != "GET" && method != "HEAD"
	for _, reject := range []authorizationRouteExpectation{
		authorizationRoleRejection(c, p, unsafe), authorizationTokenRejection(c, p), authorizationCSRFRejection(p, unsafe),
	} {
		if reject.status != 0 {
			return reject
		}
	}
	return permit
}
func authorizationExpectedCode(c authorizationRouteContract, expectation authorizationRouteExpectation) string {
	if !strings.Contains(c.Pattern, " /api/v1/") {
		return ""
	}
	return expectation.code
}
