package main

import (
	"errors"
	"net/http"
	"testing"
)

func TestSourceUpdateHWIDValueContracts(t *testing.T) {
	tests := []struct {
		name    string
		changes map[string]any
		want    string
	}{
		{"omitted", nil, "fixture-device"},
		{"null", map[string]any{"hwid_value": nil}, "fixture-device"},
		{"empty", map[string]any{"hwid_value": ""}, "fixture-device"},
		{"whitespace", map[string]any{"hwid_value": " \t "}, "fixture-device"},
		{"replacement", map[string]any{"hwid_value": " replacement-device "}, "replacement-device"},
		{"clear", map[string]any{"clear_hwid_value": true}, ""},
		{"clear wins", map[string]any{"clear_hwid_value": true, "hwid_value": "replacement-device"}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newSourceUpdateFixture(t, newUserMutationFixture(t))
			response := f.update(t, test.changes)
			requireRepositoryEqual(t, "update success", response.Code, http.StatusOK)
			requireRepositoryEqual(t, "success envelope", decodeJSONMap(t, response), map[string]any{"message": "source updated"})
			var value, audit string
			requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT COALESCE(hwid_value,'') FROM external_subscription_sources WHERE id = ?`, f.id).Scan(&value))
			requireRepositoryEqual(t, "HWID policy", value, test.want)
			requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT metadata_json FROM audit_events WHERE action = 'external_source.update'`).Scan(&audit))
			requireRepositoryEqual(t, "secret-free audit", audit, `{"enabled":true}`)
		})
	}
}

func TestSourceUpdateTransactionStatementFailuresRollBack(t *testing.T) {
	tests := []struct {
		name, trigger string
		want          sourceErrorWant
	}{
		{"source write", `CREATE TRIGGER fixture_update_failure BEFORE UPDATE ON external_subscription_sources BEGIN SELECT RAISE(ABORT,'fixture sensitive source failure'); END`, sourceErrorWant{500, "source_update_failed", "failed to update source", ""}},
		{"key status", `CREATE TRIGGER fixture_update_failure BEFORE UPDATE OF status ON vless_keys BEGIN SELECT RAISE(ABORT,'fixture sensitive key failure'); END`, sourceErrorWant{500, "source_update_failed", "failed to update source keys", ""}},
		{"source disappeared", `CREATE TRIGGER fixture_update_failure BEFORE UPDATE ON external_subscription_sources BEGIN DELETE FROM external_subscription_sources WHERE id = OLD.id; SELECT RAISE(IGNORE); END`, sourceErrorWant{404, "source_not_found", "source not found", ""}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newSourceUpdateFixture(t, newUserMutationFixture(t))
			before := f.snapshot(t)
			execRepositoryFixtureSQL(t, f.app, test.trigger)
			assertSourceError(t, f.update(t, map[string]any{"enabled": false, "hwid_value": "replacement-device"}), test.want)
			requireRepositoryEqual(t, "settings, keys, credentials and audit roll back", f.snapshot(t), before)
		})
	}
}

func TestSourceUpdateBeginAndCommitFailuresRollBack(t *testing.T) {
	for _, phase := range []string{"begin", "commit"} {
		t.Run(phase, func(t *testing.T) {
			base, faults := newFaultedMutationFixture(t)
			f := newSourceUpdateFixture(t, base)
			before := f.snapshot(t)
			if phase == "begin" {
				faults.beginErr = errors.New("fixture sensitive begin failure")
			} else {
				faults.commitErr = errors.New("fixture sensitive commit failure")
			}
			assertSourceError(t, f.update(t, map[string]any{"enabled": false}), sourceErrorWant{500, "source_update_failed", "failed to update source", ""})
			requireRepositoryEqual(t, "failed transaction publishes nothing", f.snapshot(t), before)
		})
	}
}

func TestSourceUpdateConflictRetainsPreparedCategories(t *testing.T) {
	f := newSourceUpdateFixture(t, newUserMutationFixture(t))
	otherURL := "https://provider.example/other-source"
	createSourceForTest(t, f.app, otherURL, "")
	before := f.snapshot(t)
	response := f.update(t, map[string]any{"source_url": otherURL, "category": "Updated category", "key_category": "Updated profiles"})
	assertSourceError(t, response, sourceErrorWant{409, "source_url_conflict", "source URL already exists", "source_url"})
	requireRepositoryEqual(t, "conflict retains original source and keys", f.snapshot(t), before)
	requireRepositoryEqual(t, "category preparation remains outside transaction", mutationCount(t, f.app, `SELECT COUNT(*) FROM external_source_categories WHERE name = 'Updated category'`), 1)
	requireRepositoryEqual(t, "key category preparation remains outside transaction", mutationCount(t, f.app, `SELECT COUNT(*) FROM key_categories WHERE name = 'Updated profiles'`), 1)
}

func TestSourceUpdateEnablementPublishesLinkedKeyStatus(t *testing.T) {
	f := newSourceUpdateFixture(t, newUserMutationFixture(t))
	for _, step := range []struct {
		enabled bool
		status  string
	}{{false, "non-active"}, {true, "active"}} {
		response := f.update(t, map[string]any{"enabled": step.enabled})
		requireRepositoryEqual(t, "enablement response", response.Code, http.StatusOK)
		var enabled int
		var version, modelName, status string
		requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT enabled, hwid_version, hwid_model_name FROM external_subscription_sources WHERE id = ?`, f.id).Scan(&enabled, &version, &modelName))
		requireRepositoryEqual(t, "source enablement", enabled != 0, step.enabled)
		requireRepositoryEqual(t, "trimmed version", version, "2.0")
		requireRepositoryEqual(t, "trimmed model", modelName, "Updated")
		requireRepositorySuccess(t, f.app.db.QueryRow(`SELECT status FROM vless_keys WHERE external_source_id = ?`, f.id).Scan(&status))
		requireRepositoryEqual(t, "linked key enablement", status, step.status)
	}
	requireRepositoryEqual(t, "one audit per committed update", mutationCount(t, f.app, `SELECT COUNT(*) FROM audit_events WHERE action = 'external_source.update'`), 2)
}
