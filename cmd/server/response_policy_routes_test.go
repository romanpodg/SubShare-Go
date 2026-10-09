package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func assertResponseRouteMessage(t *testing.T, recorder *httptest.ResponseRecorder, message string) {
	t.Helper()
	requireRepositoryEqual(t, "success status", recorder.Code, http.StatusOK)
	requireRepositoryEqual(t, "success envelope", decodeJSONMap(t, recorder), map[string]any{"message": message})
}

type responseRouteErrorWant struct {
	status        int
	code, message string
}

func assertResponseRouteError(t *testing.T, recorder *httptest.ResponseRecorder, want responseRouteErrorWant) {
	t.Helper()
	requireRepositoryEqual(t, "error status", recorder.Code, want.status)
	requireRepositoryEqual(t, "error envelope", decodeJSONMap(t, recorder), map[string]any{
		"code": want.code, "message": want.message, "error": want.message,
		"request_id": "mutation-fixture", "field_errors": map[string]any{},
	})
}

func responseRouteItem(t *testing.T, recorder *httptest.ResponseRecorder, id string) map[string]any {
	t.Helper()
	requireRepositoryEqual(t, "list status", recorder.Code, http.StatusOK)
	payload := decodeJSONMap(t, recorder)
	requireRepositoryEqual(t, "list envelope", len(payload), 1)
	for _, value := range payload["data"].([]any) {
		item := value.(map[string]any)
		if fmt.Sprint(item["id"]) == id {
			return item
		}
	}
	t.Fatal("created item missing from registered list route")
	return nil
}

func TestResponsePolicyRegisteredTemplateCRUD(t *testing.T) {
	f := newUserMutationFixture(t)
	input := templateInput{Name: " Portable Template ", Format: " PLAIN ", Content: "{{title}}\n{{subscription}}", Enabled: true}
	created := f.request(http.MethodPost, "/api/v1/templates", mutationJSON(t, input))
	requireRepositoryEqual(t, "template create status", created.Code, http.StatusCreated)
	payload := decodeJSONMap(t, created)
	requireRepositoryEqual(t, "template create envelope", len(payload), 2)
	requireRepositoryEqual(t, "template normalized slug", payload["slug"], "portable-template")
	id := fmt.Sprint(payload["id"])
	input.Name, input.Content, input.Enabled = "Renamed", "replacement", false
	updated := f.request(http.MethodPut, "/api/v1/templates/"+id, mutationJSON(t, input))
	assertResponseRouteMessage(t, updated, "template updated")
	listed := responseRouteItem(t, f.request(http.MethodGet, "/api/v1/templates", ""), id)
	requireRepositoryEqual(t, "template normalized name", listed["name"], "Renamed")
	requireRepositoryEqual(t, "template stable slug", listed["slug"], "portable-template")
	requireRepositoryEqual(t, "template normalized format", listed["format"], "plain")
	requireRepositoryEqual(t, "template enabled state", listed["enabled"], false)
	requireRepositoryEqual(t, "template updated content", listed["content"], "replacement")
	deleted := f.request(http.MethodDelete, "/api/v1/templates/"+id, "")
	assertResponseRouteMessage(t, deleted, "template deleted")
	missing := f.request(http.MethodDelete, "/api/v1/templates/"+id, "")
	assertResponseRouteError(t, missing, responseRouteErrorWant{http.StatusNotFound, "template_not_found", "template not found"})
}

func TestResponsePolicyRegisteredRuleCRUD(t *testing.T) {
	f := newUserMutationFixture(t)
	input := responsePolicyInput()
	input.Enabled = true
	created := f.request(http.MethodPost, "/api/v1/response-rules", mutationJSON(t, input))
	requireRepositoryEqual(t, "rule create status", created.Code, http.StatusCreated)
	payload := decodeJSONMap(t, created)
	requireRepositoryEqual(t, "rule create envelope", len(payload), 1)
	id := fmt.Sprint(payload["id"])
	listed := responseRouteItem(t, f.request(http.MethodGet, "/api/v1/response-rules", ""), id)
	requireRepositoryEqual(t, "rule normalized name", listed["name"], "Rule")
	requireRepositoryEqual(t, "rule normalized operator", listed["operator"], "AND")
	requireRepositoryEqual(t, "rule normalized type", listed["response_type"], "plain")
	input.Name, input.Enabled, input.Conditions, input.Headers = "Renamed", false, nil, nil
	updated := f.request(http.MethodPut, "/api/v1/response-rules/"+id, mutationJSON(t, input))
	assertResponseRouteMessage(t, updated, "response rule updated")
	listed = responseRouteItem(t, f.request(http.MethodGet, "/api/v1/response-rules", ""), id)
	requireRepositoryEqual(t, "rule updated name", listed["name"], "Renamed")
	requireRepositoryEqual(t, "rule disabled", listed["enabled"], false)
	requireRepositoryEqual(t, "rule null conditions", listed["conditions"] == nil, true)
	requireRepositoryEqual(t, "rule null headers", listed["headers"] == nil, true)
	assertResponseRouteMessage(t, f.request(http.MethodDelete, "/api/v1/response-rules/"+id, ""), "response rule deleted")
	assertResponseRouteError(t, f.request(http.MethodDelete, "/api/v1/response-rules/"+id, ""), responseRouteErrorWant{http.StatusNotFound, "rule_not_found", "response rule not found"})
}

func TestResponsePolicyRegisteredCorruptReadFailsClosedRepeatedly(t *testing.T) {
	f := newUserMutationFixture(t)
	execRepositoryFixtureSQL(t, f.app, `DELETE FROM response_rules`)
	execRepositoryFixtureSQL(t, f.app, `INSERT INTO response_rules(id, name, enabled, priority, operator, conditions_json, response_type, headers_json) VALUES(9001, 'Fixture', 1, 0, 'AND', '[]', 'plain', '{broken')`)
	want := responseRouteErrorWant{http.StatusInternalServerError, "rules_list_failed", "failed to load response rules"}
	for count := 1; count <= 2; count++ {
		assertResponseRouteError(t, f.request(http.MethodGet, "/api/v1/response-rules", ""), want)
		assertResponseStoreRepairState(t, f.app, 0, count)
	}
}

func TestResponsePolicyRegisteredListDatabaseErrors(t *testing.T) {
	cases := []struct{ table, path, code, message string }{
		{"response_rules", "/api/v1/response-rules", "rules_list_failed", "failed to load response rules"},
		{"subscription_templates", "/api/v1/templates", "templates_list_failed", "failed to load templates"},
	}
	for _, test := range cases {
		t.Run(test.table, func(t *testing.T) {
			f := newUserMutationFixture(t)
			execRepositoryFixtureSQL(t, f.app, `DELETE FROM response_rules`)
			execRepositoryFixtureSQL(t, f.app, `DROP TABLE `+test.table)
			assertResponseRouteError(t, f.request(http.MethodGet, test.path, ""), responseRouteErrorWant{http.StatusInternalServerError, test.code, test.message})
		})
	}
}
