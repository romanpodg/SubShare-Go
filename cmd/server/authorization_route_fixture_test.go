package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type authorizationRouteContract struct {
	Pattern     string `json:"pattern"`
	Guard       string `json:"guard"`
	Scope       string `json:"scope"`
	Interactive bool   `json:"interactive,omitempty"`
	Status      int    `json:"status"`
	Code        string `json:"code,omitempty"`
}
type authorizationRoutePrincipal struct {
	name, role, cookie, csrf, bearer, scope string
	emptyCookie                             bool
}

type authorizationRouteTokenSeed struct {
	adminID  int64
	sequence int
	scope    string
}

func loadAuthorizationRouteContracts(t *testing.T) []authorizationRouteContract {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "authorization_routes_contract.json"))
	requireRepositorySuccess(t, err)
	var routes []authorizationRouteContract
	requireRepositorySuccess(t, json.Unmarshal(data, &routes))
	return routes
}
func seedAuthorizationRouteToken(t *testing.T, app *App, seed authorizationRouteTokenSeed) string {
	t.Helper()
	raw := fmt.Sprintf("ss_%064x", seed.sequence)
	sum := sha256.Sum256([]byte(raw))
	scopes := []string{}
	if seed.scope != "" {
		scopes = append(scopes, seed.scope)
	}
	data, err := json.Marshal(scopes)
	requireRepositorySuccess(t, err)
	execRepositoryFixtureSQL(t, app, `INSERT INTO api_tokens(name,token_prefix,token_hash,scopes_json,created_by_admin_id) VALUES(?,?,?,?,?)`, fmt.Sprintf("authorization-fixture-%d", seed.sequence), raw[:13], hex.EncodeToString(sum[:]), string(data), seed.adminID)
	return raw
}
func authorizationRoutePrincipals(t *testing.T, f userMutationFixture) []authorizationRoutePrincipal {
	t.Helper()
	var ownerID int64
	requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT admin_id FROM admin_sessions WHERE id = ?`, hashSessionID(f.session)).Scan(&ownerID))
	result := []authorizationRoutePrincipal{{name: "anonymous"}}
	ids := map[string]int64{"owner": ownerID}
	for _, role := range []string{"viewer", "operator", "super_admin", "support_admin"} {
		cookie, csrf, id := seedIntegrationSession(t, f.app, role)
		ids[role] = id
		result = append(result, authorizationRoutePrincipal{name: role, role: role, cookie: cookie, csrf: csrf})
	}
	result = append(result, authorizationRoutePrincipal{name: "owner missing CSRF", role: "owner", cookie: f.session}, authorizationRoutePrincipal{name: "owner invalid CSRF", role: "owner", cookie: f.session, csrf: "fixture-invalid"})
	for _, scope := range []string{"read", "users:write", "keys:write", "settings:write", "*", ""} {
		result = append(result, authorizationRoutePrincipal{name: "owner token " + scope, role: "owner", scope: scope, bearer: seedAuthorizationRouteToken(t, f.app, authorizationRouteTokenSeed{ownerID, len(result) + 1, scope})})
	}
	for _, role := range []string{"viewer", "operator", "super_admin", "support_admin"} {
		result = append(result, authorizationRoutePrincipal{name: role + " token wildcard", role: role, scope: "*", bearer: seedAuthorizationRouteToken(t, f.app, authorizationRouteTokenSeed{ids[role], len(result) + 1, "*"})})
	}
	return append(result, authorizationRoutePrincipal{name: "owner valid CSRF", role: "owner", cookie: f.session, csrf: f.csrf})
}
func authorizationRouteRequest(f userMutationFixture, contract authorizationRouteContract, principal authorizationRoutePrincipal, method string) *httptest.ResponseRecorder {
	_, path, _ := strings.Cut(contract.Pattern, " ")
	path = strings.NewReplacer("{id}", "900000", "{hwid}", "fixture-hwid").Replace(path)
	request := httptest.NewRequest(method, path, strings.NewReader("{"))
	if principal.cookie != "" || principal.emptyCookie {
		request.AddCookie(&http.Cookie{Name: "subshare_admin_session", Value: principal.cookie})
	}
	if principal.csrf != "" {
		request.Header.Set("X-CSRF-Token", principal.csrf)
	}
	if principal.bearer != "" {
		request.Header.Set("Authorization", "Bearer "+principal.bearer)
	}
	recorder := httptest.NewRecorder()
	f.handler.ServeHTTP(recorder, request)
	return recorder
}
func joinAuthorizationRouteJob(t *testing.T, app *App, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var payload map[string]any
	if json.Unmarshal(recorder.Body.Bytes(), &payload) != nil {
		return
	}
	id, ok := payload["job_id"].(float64)
	if !ok {
		return
	}
	deadline := time.Now().Add(10 * time.Second)
	for mutationCount(t, app, `SELECT COUNT(*) FROM audit_events WHERE action = 'keys.health_check' AND json_extract(metadata_json,'$.job_id') = ?`, int64(id)) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("authorization fixture job did not reach completion")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
