package main

import (
	"net/http"
	"testing"
)

func TestSourceUpdateValidationPrecedence(t *testing.T) {
	f := newSourceUpdateFixture(t, newUserMutationFixture(t))
	before := f.snapshot(t)
	tests := []struct {
		name, path, body string
		want             sourceErrorWant
	}{
		{"missing source before body", "/api/v1/sources/999999", "{", sourceErrorWant{404, "source_not_found", "source not found", ""}},
		{"invalid body", f.path, "{", sourceErrorWant{400, "validation_failed", "invalid request body", ""}},
		{"unknown field", f.path, `{"unexpected":true}`, sourceErrorWant{400, "validation_failed", "invalid request body", ""}},
		{"trailing value", f.path, `{} {}`, sourceErrorWant{400, "validation_failed", "invalid request body", ""}},
		{"name before URL", f.path, `{"name":"", "source_url":"invalid"}`, sourceErrorWant{400, "source_name_invalid", "name is required", "name"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSourceError(t, f.request(http.MethodPut, test.path, test.body), test.want)
			requireRepositoryEqual(t, "rejected update writes nothing", f.snapshot(t), before)
		})
	}
}

func TestSourceUpdateCategoryFailureWithholdsMutation(t *testing.T) {
	tests := []struct{ name, trigger, code, message string }{
		{"source category", `CREATE TRIGGER fixture_update_failure BEFORE INSERT ON external_source_categories BEGIN SELECT RAISE(ABORT,'fixture sensitive category failure'); END`, "source_category_failed", "failed to save source category"},
		{"key category", `CREATE TRIGGER fixture_update_failure BEFORE INSERT ON key_categories BEGIN SELECT RAISE(ABORT,'fixture sensitive category failure'); END`, "key_category_failed", "failed to save key category"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newSourceUpdateFixture(t, newUserMutationFixture(t))
			before := f.snapshot(t)
			execRepositoryFixtureSQL(t, f.app, test.trigger)
			response := f.update(t, map[string]any{"key_category": "Faulted profiles"})
			assertSourceError(t, response, sourceErrorWant{500, test.code, test.message, ""})
			requireRepositoryEqual(t, "category failure writes no source, key or audit", f.snapshot(t), before)
		})
	}
}
