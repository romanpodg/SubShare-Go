package main

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

type authorizationCredentialFailure struct {
	name   string
	token  bool
	code   string
	mutate func(*testing.T, userMutationFixture, *mutationSQLFaults)
}

func authorizationLookupFault(part string) func(string) error {
	return func(query string) error {
		if strings.Contains(strings.Join(strings.Fields(query), " "), part) {
			return errors.New("fixture credential lookup failure")
		}
		return nil
	}
}

func TestActualRouteInvalidCredentialsFailClosed(t *testing.T) {
	cases := []authorizationCredentialFailure{
		{"expired session", false, "unauthorized", func(t *testing.T, f userMutationFixture, _ *mutationSQLFaults) {
			execRepositoryFixtureSQL(t, f.app, `UPDATE admin_sessions SET expires_at = ?`, time.Now().Add(-time.Hour))
		}},
		{"revoked token", true, "unauthorized", func(t *testing.T, f userMutationFixture, _ *mutationSQLFaults) {
			execRepositoryFixtureSQL(t, f.app, `UPDATE api_tokens SET revoked_at = CURRENT_TIMESTAMP`)
		}},
		{"expired token", true, "unauthorized", func(t *testing.T, f userMutationFixture, _ *mutationSQLFaults) {
			execRepositoryFixtureSQL(t, f.app, `UPDATE api_tokens SET expires_at = ?`, time.Now().Add(-time.Hour))
		}},
		{"deleted session administrator", false, "unauthorized", func(t *testing.T, f userMutationFixture, _ *mutationSQLFaults) {
			execRepositoryFixtureSQL(t, f.app, `DELETE FROM admins WHERE username = 'admin-owner'`)
		}},
		{"deleted token administrator", true, "unauthorized", func(t *testing.T, f userMutationFixture, _ *mutationSQLFaults) {
			execRepositoryFixtureSQL(t, f.app, `DELETE FROM admins WHERE username = 'admin-owner'`)
		}},
		{"object scopes", true, "unauthorized", func(t *testing.T, f userMutationFixture, _ *mutationSQLFaults) {
			execRepositoryFixtureSQL(t, f.app, `UPDATE api_tokens SET scopes_json = '{}'`)
		}},
		{"numeric scopes", true, "unauthorized", func(t *testing.T, f userMutationFixture, _ *mutationSQLFaults) {
			execRepositoryFixtureSQL(t, f.app, `UPDATE api_tokens SET scopes_json = '[1]'`)
		}},
		{"unknown scope", true, "token_scope_forbidden", func(t *testing.T, f userMutationFixture, _ *mutationSQLFaults) {
			execRepositoryFixtureSQL(t, f.app, `UPDATE api_tokens SET scopes_json = '["unknown"]'`)
		}},
		{"null scopes", true, "token_scope_forbidden", func(t *testing.T, f userMutationFixture, _ *mutationSQLFaults) {
			execRepositoryFixtureSQL(t, f.app, `UPDATE api_tokens SET scopes_json = 'null'`)
		}},
		{"session lookup error", false, "unauthorized", func(_ *testing.T, _ userMutationFixture, faults *mutationSQLFaults) {
			faults.queryErr = authorizationLookupFault("FROM admin_sessions")
		}},
		{"token lookup error", true, "unauthorized", func(_ *testing.T, _ userMutationFixture, faults *mutationSQLFaults) {
			faults.queryErr = authorizationLookupFault("FROM api_tokens t")
		}},
		{"role lookup error", false, "unauthorized", func(_ *testing.T, _ userMutationFixture, faults *mutationSQLFaults) {
			faults.queryErr = authorizationLookupFault("SELECT role FROM admins")
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) { assertAuthorizationCredentialFailure(t, test) })
	}
}

func assertAuthorizationCredentialFailure(t *testing.T, test authorizationCredentialFailure) {
	t.Helper()
	f, faults := newFaultedMutationFixture(t)
	var ownerID int64
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT admin_id FROM admin_sessions WHERE id = ?`, hashSessionID(f.session)).Scan(&ownerID))
	principal := authorizationRoutePrincipal{cookie: f.session}
	if test.token {
		principal.cookie = ""
		principal.bearer = seedAuthorizationRouteToken(t, f.app, authorizationRouteTokenSeed{ownerID, 1, "*"})
	}
	test.mutate(t, f, faults)
	contract := authorizationRouteContract{Pattern: "GET /api/v1/dashboard"}
	recorder := authorizationRouteRequest(f, contract, principal, http.MethodGet)
	status := http.StatusUnauthorized
	if test.code == "token_scope_forbidden" {
		status = http.StatusForbidden
	}
	requireRepositoryEqual(t, "invalid credential status", recorder.Code, status)
	requireRepositoryEqual(t, "invalid credential safe code", decodeJSONMap(t, recorder)["code"], test.code)
}
