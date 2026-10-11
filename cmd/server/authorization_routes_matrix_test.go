package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActualRegisteredRouteAuthorizationMatrix(t *testing.T) {
	f := newUserMutationFixture(t)
	principals := authorizationRoutePrincipals(t, f)
	for _, contract := range loadAuthorizationRouteContracts(t) {
		t.Run(contract.Pattern, func(t *testing.T) {
			assertAuthorizationRegisteredPattern(t, f, contract)
			for _, method := range authorizationContractMethods(contract) {
				for _, principal := range principals {
					t.Run(method+"/"+principal.name, func(t *testing.T) {
						assertAuthorizationRouteResponse(t, f, authorizationRouteCase{contract, principal, method})
					})
				}
			}
		})
	}
}

type authorizationRouteCase struct {
	contract  authorizationRouteContract
	principal authorizationRoutePrincipal
	method    string
}

func authorizationContractMethods(contract authorizationRouteContract) []string {
	method, _, _ := strings.Cut(contract.Pattern, " ")
	if method == http.MethodGet {
		return []string{method, http.MethodHead}
	}
	return []string{method}
}

func assertAuthorizationRegisteredPattern(t *testing.T, f userMutationFixture, contract authorizationRouteContract) {
	t.Helper()
	method, path, _ := strings.Cut(contract.Pattern, " ")
	path = strings.NewReplacer("{id}", "900000", "{hwid}", "fixture-hwid").Replace(path)
	_, pattern := f.handler.(*http.ServeMux).Handler(httptest.NewRequest(method, path, nil))
	requireRepositoryEqual(t, "actual registered route", pattern, contract.Pattern)
}

func assertAuthorizationRouteResponse(t *testing.T, f userMutationFixture, test authorizationRouteCase) {
	t.Helper()
	expect := authorizationExpectedResponse(test.contract, test.principal, test.method)
	recorder := authorizationRouteRequest(f, test.contract, test.principal, test.method)
	joinAuthorizationRouteJob(t, f.app, recorder)
	requireRepositoryEqual(t, "actual route authorization status", recorder.Code, expect.status)
	var payload map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &payload)
	code, _ := payload["code"].(string)
	requireRepositoryEqual(t, "actual route authorization code", code, authorizationExpectedCode(test.contract, expect))
}
