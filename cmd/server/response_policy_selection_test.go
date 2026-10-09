package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestResponsePolicyPriorityAndTieOrdering(t *testing.T) {
	app := newIntegrationApp(t)
	execRepositoryFixtureSQL(t, app, `DELETE FROM response_rules`)
	execRepositoryFixtureSQL(t, app, `
		INSERT INTO response_rules(id, name, enabled, priority, operator, conditions_json, response_type, headers_json)
		VALUES(40, 'later priority', 1, 20, 'AND', '[]', 'block', '[]'),
		      (30, 'later tie', 1, 10, 'OR', '[]', 'not-found', '[]'),
		      (20, 'first enabled', 1, 10, 'AND', '[]', 'browser', '[]'),
		      (10, 'disabled first', 0, 0, 'AND', '[]', 'block', '[]')
	`)
	rules, err := app.listResponseRules()
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	if !reflect.DeepEqual(ids, []int64{10, 20, 30, 40}) {
		t.Fatalf("priority/id ordering = %v", ids)
	}
	assertSelectedResponseRule(t, app, 20)
	execRepositoryFixtureSQL(t, app, `UPDATE response_rules SET enabled = 0 WHERE id = 20`)
	assertSelectedResponseRule(t, app, 30)
	execRepositoryFixtureSQL(t, app, `UPDATE response_rules SET enabled = 0`)
	assertSelectedResponseRule(t, app, 0)
}

func assertSelectedResponseRule(t *testing.T, app *App, want int64) {
	t.Helper()
	rule, err := app.matchSubscriptionResponseRule(httptest.NewRequest(http.MethodGet, "/subscription/fixture", nil))
	if err != nil {
		t.Fatal(err)
	}
	var got int64
	if rule != nil {
		got = rule.ID
	}
	if got != want {
		t.Fatalf("selected rule = %d, want %d", got, want)
	}
}

func TestResponsePolicyTemplateCompatibility(t *testing.T) {
	cases := []struct {
		name, format, response, code string
		enabled                      int
		missing                      bool
	}{
		{"plain", "plain", " PLAIN ", "", 1, false},
		{"base64", "base64", "base64", "", 1, false},
		{"xray", "xray-json", "xray-json", "", 1, false},
		{"mihomo", "mihomo", "mihomo", "", 1, false},
		{"sing-box", "sing-box", "sing-box", "", 1, false},
		{"mismatch", "plain", "mihomo", "template_format_mismatch", 1, false},
		{"disabled", "plain", "plain", "template_invalid", 0, false},
		{"missing", "plain", "plain", "template_invalid", 1, true},
		{"browser-template", "plain", "browser", "template_format_mismatch", 1, false},
		{"block-template", "plain", "block", "template_format_mismatch", 1, false},
		{"not-found-template", "plain", "not-found", "template_format_mismatch", 1, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			app := newIntegrationApp(t)
			execRepositoryFixtureSQL(t, app, `INSERT INTO subscription_templates(id, slug, name, format, content, enabled) VALUES(9001, 'fixture', 'Fixture', ?, '', ?)`, test.format, test.enabled)
			id := int64(9001)
			if test.missing {
				id = 9002
			}
			input := responsePolicyInput()
			input.ResponseType, input.TemplateID = test.response, &id
			assertResponsePolicySave(t, app, input, test.code)
		})
	}
}

func TestResponsePolicyNoTemplateResponses(t *testing.T) {
	for _, response := range []string{"browser", "block", "not-found", "plain"} {
		t.Run(response, func(t *testing.T) {
			input := responsePolicyInput()
			input.ResponseType = response
			assertResponsePolicySave(t, newIntegrationApp(t), input, "")
		})
	}
}

func assertResponsePolicySave(t *testing.T, app *App, input responseRuleInput, code string) {
	t.Helper()
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	app.saveResponseRule(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/response-rules", bytes.NewReader(body)), nil)
	wantStatus := http.StatusCreated
	if code != "" {
		wantStatus = http.StatusBadRequest
		if !strings.Contains(recorder.Body.String(), `"code":"`+code+`"`) {
			t.Fatalf("error code missing: %s", recorder.Body.String())
		}
	}
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d", recorder.Code, wantStatus)
	}
}
