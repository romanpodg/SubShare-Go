package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func responseStoreFixture(t *testing.T, conditions, headers string) *App {
	t.Helper()
	app := newIntegrationApp(t)
	execRepositoryFixtureSQL(t, app, `DELETE FROM response_rules`)
	execRepositoryFixtureSQL(t, app, `INSERT INTO response_rules(id, name, enabled, priority, operator, conditions_json, response_type, headers_json) VALUES(9001, 'Fixture', 1, 0, 'AND', ?, 'plain', ?)`, conditions, headers)
	return app
}

func assertResponseStoreDecodeFailure(t *testing.T, app *App, field string) {
	t.Helper()
	rules, err := app.listResponseRules()
	if err == nil {
		t.Fatal("corrupt rule did not fail closed")
	}
	if rules != nil {
		t.Fatal("corrupt rule returned a partial fallback list")
	}
	want := "response rule 9001 has invalid " + strings.TrimSuffix(field, "_json") + " JSON:"
	if !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("decode error = %q, want prefix %q", err.Error(), want)
	}
}

func assertResponseStoreRepairState(t *testing.T, app *App, enabled, audits int) {
	t.Helper()
	var gotEnabled, gotAudits int
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT enabled FROM response_rules WHERE id = 9001`).Scan(&gotEnabled))
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE action = 'response_rule.disabled_invalid' AND target_id = '9001'`).Scan(&gotAudits))
	requireRepositoryEqual(t, "corrupt rule enabled", gotEnabled, enabled)
	requireRepositoryEqual(t, "corrupt rule audits", gotAudits, audits)
}

func TestResponseStoreCorruptRowsRepeatedReadContract(t *testing.T) {
	for _, field := range []string{"conditions_json", "headers_json"} {
		t.Run(field, func(t *testing.T) {
			conditions, headers := "[]", "[]"
			if field == "conditions_json" {
				conditions = "{broken"
			} else {
				headers = "{broken"
			}
			app := responseStoreFixture(t, conditions, headers)
			app.db.SetMaxOpenConns(1)
			assertResponseStoreDecodeFailure(t, app, field)
			assertResponseStoreRepairState(t, app, 0, 1)
			// Disabled corrupt rows remain visible to the list read: they still
			// return an error and produce another best-effort repair/audit.
			assertResponseStoreDecodeFailure(t, app, field)
			assertResponseStoreRepairState(t, app, 0, 2)
			assertResponseStoreAuditMetadata(t, app, field)
		})
	}
}

func assertResponseStoreAuditMetadata(t *testing.T, app *App, field string) {
	t.Helper()
	var metadata string
	requireRepositorySuccess(t, app.db.QueryRow(`SELECT metadata_json FROM audit_events WHERE action = 'response_rule.disabled_invalid' AND target_id = ? ORDER BY id LIMIT 1`, strconv.FormatInt(9001, 10)).Scan(&metadata))
	var got map[string]string
	requireRepositorySuccess(t, json.Unmarshal([]byte(metadata), &got))
	requireRepositoryEqual(t, "corrupt row metadata field", got["field"], field)
	requireRepositoryEqual(t, "corrupt row metadata keys", len(got), 2)
	if got["error"] == "" {
		t.Fatal("corrupt row audit omitted its decode error")
	}
}

func TestResponseStoreRepairFailuresAreBestEffort(t *testing.T) {
	cases := []struct {
		name, trigger   string
		enabled, audits int
	}{
		{"disable", `CREATE TRIGGER fixture_disable_failure BEFORE UPDATE ON response_rules BEGIN SELECT RAISE(ABORT, 'fixture disable failure'); END`, 1, 1},
		{"audit", `CREATE TRIGGER fixture_audit_failure BEFORE INSERT ON audit_events BEGIN SELECT RAISE(ABORT, 'fixture audit failure'); END`, 0, 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			app := responseStoreFixture(t, "{broken", "[]")
			execRepositoryFixtureSQL(t, app, test.trigger)
			assertResponseStoreDecodeFailure(t, app, "conditions_json")
			assertResponseStoreRepairState(t, app, test.enabled, test.audits)
		})
	}
}

func TestResponseStoreEmptyNullAndClosedReadContracts(t *testing.T) {
	app := responseStoreFixture(t, "null", "null")
	rules, err := app.listResponseRules()
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "null rule list size", len(rules), 1)
	requireRepositoryEqual(t, "null conditions", rules[0].Conditions == nil, true)
	requireRepositoryEqual(t, "null headers", rules[0].Headers == nil, true)
	assertResponseStoreRepairState(t, app, 1, 0)
	execRepositoryFixtureSQL(t, app, `DELETE FROM response_rules`)
	rules, err = app.listResponseRules()
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "empty rule list nonnil", rules != nil, true)
	requireRepositoryEqual(t, "empty rule list size", len(rules), 0)
	requireRepositorySuccess(t, app.db.Close())
	_, err = app.listResponseRules()
	if err == nil {
		t.Fatal("closed database did not fail the rule read")
	}
}

func TestResponseStoreTemplateProjectionAndMissingContracts(t *testing.T) {
	app := responseStoreFixture(t, "[]", "[]")
	execRepositoryFixtureSQL(t, app, `DELETE FROM response_rules`)
	execRepositoryFixtureSQL(t, app, `DELETE FROM subscription_templates`)
	execRepositoryFixtureSQL(t, app, `INSERT INTO subscription_templates(id, slug, name, format, content, enabled, is_system) VALUES(9001, 'b', 'B', 'plain', 'body-b', 0, 0), (9002, 'a', 'A', 'base64', 'body-a', 1, 0), (9003, 'z', 'Z', 'sing-box', '', 1, 1), (9004, 'sa', 'A', 'mihomo', '', 1, 1)`)
	templates, err := app.listSubscriptionTemplates()
	requireRepositorySuccess(t, err)
	var ids []int64
	for _, item := range templates {
		ids = append(ids, item.ID)
	}
	requireRepositoryEqual(t, "template projection order", fmt.Sprint(ids), "[9004 9003 9002 9001]")
	id := int64(9001)
	item, err := app.loadTemplate(&id)
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "disabled template still projected", item.Enabled, false)
	requireRepositoryEqual(t, "template content", item.Content, "body-b")
	item, err = app.loadTemplate(nil)
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "absent template reference", item == nil, true)
	id = 9005
	_, err = app.loadTemplate(&id)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing template error = %v", err)
	}
}
