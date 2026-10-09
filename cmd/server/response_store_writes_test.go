package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func responseStoreWriteRequest(t *testing.T, method string, input any) *http.Request {
	t.Helper()
	body, err := json.Marshal(input)
	requireRepositorySuccess(t, err)
	request := httptest.NewRequest(method, "/fixture/9001", bytes.NewReader(body))
	request.SetPathValue("id", "9001")
	return request
}

func assertResponseStoreWriteError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	requireRepositoryEqual(t, "failed write status", recorder.Code, status)
	requireRepositoryEqual(t, "failed write code", decodeJSONMap(t, recorder)["code"], code)
}

func assertResponseStoreNoMutationAudit(t *testing.T, app *App, action string) {
	t.Helper()
	var count int
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE action = ?`, action).Scan(&count))
	requireRepositoryEqual(t, "failed write did not audit success", count, 0)
}

func TestResponseStoreRuleWriteFailureRetainsState(t *testing.T) {
	cases := []struct{ method, operation, code, action string }{
		{http.MethodPost, "INSERT", "rule_create_failed", "response_rule.create"},
		{http.MethodPut, "UPDATE", "rule_update_failed", "response_rule.update"},
	}
	for _, test := range cases {
		t.Run(test.operation, func(t *testing.T) {
			app := responseStoreFixture(t, "[]", "[]")
			execRepositoryFixtureSQL(t, app, fmt.Sprintf(`CREATE TRIGGER fixture_write_failure BEFORE %s ON response_rules BEGIN SELECT RAISE(ABORT, 'fixture write failure'); END`, test.operation))
			recorder := httptest.NewRecorder()
			request := responseStoreWriteRequest(t, test.method, responsePolicyInput())
			if test.method == http.MethodPost {
				app.apiV1CreateResponseRule(recorder, request)
			} else {
				app.apiV1UpdateResponseRule(recorder, request)
			}
			assertResponseStoreWriteError(t, recorder, http.StatusConflict, test.code)
			var count int
			var name string
			requireRepositorySuccess(t, app.db.QueryRow(`SELECT COUNT(*), MIN(name) FROM response_rules`).Scan(&count, &name))
			requireRepositoryEqual(t, "retained rule count", count, 1)
			requireRepositoryEqual(t, "retained rule name", name, "Fixture")
			assertResponseStoreNoMutationAudit(t, app, test.action)
		})
	}
}

func responseStoreTemplateFixture(t *testing.T) *App {
	t.Helper()
	app := responseStoreFixture(t, "[]", "[]")
	execRepositoryFixtureSQL(t, app, `INSERT INTO subscription_templates(id, slug, name, format, content, enabled) VALUES(9001, 'fixture', 'Fixture', 'plain', 'original body', 1)`)
	return app
}

func TestResponseStoreTemplateWriteFailureRetainsState(t *testing.T) {
	cases := []struct{ method, operation, code, action string }{
		{http.MethodPost, "INSERT", "template_create_failed", "template.create"},
		{http.MethodPut, "UPDATE", "template_update_failed", "template.update"},
	}
	for _, test := range cases {
		t.Run(test.operation, func(t *testing.T) {
			app := responseStoreTemplateFixture(t)
			execRepositoryFixtureSQL(t, app, fmt.Sprintf(`CREATE TRIGGER fixture_write_failure BEFORE %s ON subscription_templates BEGIN SELECT RAISE(ABORT, 'fixture write failure'); END`, test.operation))
			request := responseStoreWriteRequest(t, test.method, templateInput{Name: "Changed", Format: "plain", Content: "changed body", Enabled: true})
			recorder := httptest.NewRecorder()
			if test.method == http.MethodPost {
				app.apiV1CreateTemplate(recorder, request)
			} else {
				app.apiV1UpdateTemplate(recorder, request)
			}
			assertResponseStoreWriteError(t, recorder, http.StatusConflict, test.code)
			var name, content string
			requireRepositorySuccess(t, app.db.QueryRow(`SELECT name, content FROM subscription_templates WHERE id = 9001`).Scan(&name, &content))
			requireRepositoryEqual(t, "retained template name", name, "Fixture")
			requireRepositoryEqual(t, "retained template content", content, "original body")
			assertResponseStoreNoMutationAudit(t, app, test.action)
		})
	}
}

func TestResponseStoreDeleteFailureRetainsState(t *testing.T) {
	app := responseStoreTemplateFixture(t)
	execRepositoryFixtureSQL(t, app, `CREATE TRIGGER fixture_rule_delete_failure BEFORE DELETE ON response_rules BEGIN SELECT RAISE(ABORT, 'fixture delete failure'); END`)
	execRepositoryFixtureSQL(t, app, `CREATE TRIGGER fixture_template_delete_failure BEFORE DELETE ON subscription_templates BEGIN SELECT RAISE(ABORT, 'fixture delete failure'); END`)
	request := responseStoreWriteRequest(t, http.MethodDelete, nil)
	ruleRecorder := httptest.NewRecorder()
	app.apiV1DeleteResponseRule(ruleRecorder, request)
	assertResponseStoreWriteError(t, ruleRecorder, http.StatusInternalServerError, "rule_delete_failed")
	templateRecorder := httptest.NewRecorder()
	app.apiV1DeleteTemplate(templateRecorder, request)
	assertResponseStoreWriteError(t, templateRecorder, http.StatusConflict, "template_in_use")
	var name string
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT name FROM response_rules WHERE id = 9001`).Scan(&name))
	requireRepositoryEqual(t, "retained deleted rule", name, "Fixture")
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT name FROM subscription_templates WHERE id = 9001`).Scan(&name))
	requireRepositoryEqual(t, "retained deleted template", name, "Fixture")
	assertResponseStoreNoMutationAudit(t, app, "response_rule.delete")
	assertResponseStoreNoMutationAudit(t, app, "template.delete")
}
