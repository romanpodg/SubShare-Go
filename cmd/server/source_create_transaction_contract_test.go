package main

import (
	"errors"
	"net/http"
	"testing"
)

func TestSourceCreateTransactionStatementFailuresRetainWholeUnit(t *testing.T) {
	cases := []struct{ name, trigger, code, message string }{
		{"source category insert", `CREATE TRIGGER fixture_create_failure BEFORE INSERT ON external_source_categories WHEN NEW.name = 'CreateFixtureCategory' BEGIN SELECT RAISE(ABORT,'fixture category failure'); END`, "source_category_failed", "failed to save source category"},
		{"source category resolve", `CREATE TRIGGER fixture_create_failure AFTER INSERT ON external_source_categories WHEN NEW.name = 'CreateFixtureCategory' BEGIN DELETE FROM external_source_categories WHERE id = NEW.id; END`, "source_category_failed", "failed to resolve source category"},
		{"key category insert", `CREATE TRIGGER fixture_create_failure BEFORE INSERT ON key_categories WHEN NEW.name = 'CreateFixtureProfiles' BEGIN SELECT RAISE(ABORT,'fixture key category failure'); END`, "key_category_failed", "failed to save key category"},
		{"key category resolve", `CREATE TRIGGER fixture_create_failure AFTER INSERT ON key_categories WHEN NEW.name = 'CreateFixtureProfiles' BEGIN DELETE FROM key_categories WHERE id = NEW.id; END`, "key_category_failed", "failed to resolve key category"},
		{"source insert", `CREATE TRIGGER fixture_create_failure BEFORE INSERT ON external_subscription_sources BEGIN SELECT RAISE(ABORT,'fixture source failure'); END`, "source_create_failed", "failed to create source"},
		{"second profile", `CREATE TRIGGER fixture_create_failure BEFORE INSERT ON vless_keys WHEN NEW.label = 'Second' BEGIN SELECT RAISE(ABORT,'fixture profile failure'); END`, "source_create_failed", "failed to save imported source"},
		{"second credential", `CREATE TRIGGER fixture_create_failure BEFORE INSERT ON vless_key_secrets WHEN (SELECT label FROM vless_keys WHERE id = NEW.vless_key_id) = 'Second' BEGIN SELECT RAISE(ABORT,'fixture credential failure'); END`, "source_create_failed", "failed to save imported source"},
		{"second assignment", `CREATE TRIGGER fixture_create_failure BEFORE INSERT ON user_keys WHEN (SELECT label FROM vless_keys WHERE id = NEW.key_id) = 'Second' BEGIN SELECT RAISE(ABORT,'fixture assignment failure'); END`, "source_create_failed", "failed to save imported source"},
		{"source completion", `CREATE TRIGGER fixture_create_failure BEFORE UPDATE ON external_subscription_sources WHEN NEW.import_status = 'ok' BEGIN SELECT RAISE(ABORT,'fixture completion failure'); END`, "source_create_failed", "failed to save imported source"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			seedSourceCreateAllModeUser(t, f.app)
			before := sourceCreateMutationCounts(t, f.app)
			execRepositoryFixtureSQL(t, f.app, test.trigger)
			recorder := sourceCreateContractRequest(t, f)
			requireRepositoryEqual(t, "failed create HTTP status", recorder.Code, http.StatusInternalServerError)
			payload := decodeJSONMap(t, recorder)
			requireRepositoryEqual(t, "failed create code", payload["code"], test.code)
			requireRepositoryEqual(t, "failed create safe message", payload["error"], test.message)
			requireRepositoryEqual(t, "failed create rolls back all data and audit", sourceCreateMutationCounts(t, f.app), before)
		})
	}
}

func TestSourceCreateBeginAndCommitFailureWithholdIdentity(t *testing.T) {
	for _, phase := range []string{"begin", "commit"} {
		t.Run(phase, func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			seedSourceCreateAllModeUser(t, f.app)
			before := sourceCreateMutationCounts(t, f.app)
			if phase == "begin" {
				faults.beginErr = errors.New("fixture begin failure")
			} else {
				faults.commitErr = errors.New("fixture commit failure")
			}
			recorder := sourceCreateContractRequest(t, f)
			requireRepositoryEqual(t, "transaction failure status", recorder.Code, http.StatusInternalServerError)
			payload := decodeJSONMap(t, recorder)
			requireRepositoryEqual(t, "transaction failure code", payload["code"], "source_create_failed")
			requireRepositoryEqual(t, "transaction failure withholds data", payload["data"], nil)
			requireRepositoryEqual(t, "transaction failure rolls back full unit", sourceCreateMutationCounts(t, f.app), before)
		})
	}
}
