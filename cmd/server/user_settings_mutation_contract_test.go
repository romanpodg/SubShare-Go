package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func userSettingsFixtureState(t *testing.T, app *App, id int64) [2]string {
	t.Helper()
	var state [2]string
	requireRepositorySuccess(t, app.db.QueryRow("SELECT time_zone, language FROM users WHERE id = ?", id).Scan(&state[0], &state[1]))
	return state
}

func TestUserSettingsMutationDefaultsAliasesAndNormalization(t *testing.T) {
	for _, test := range []struct {
		name, prefix, body string
		want               [2]string
	}{
		{"v1 defaults", "/api/v1/users/", `{}`, [2]string{"Europe/Moscow", "ru"}},
		{"legacy defaults", "/api/admin/users/", `{}`, [2]string{"Europe/Moscow", "ru"}},
		{"trim and lower", "/api/v1/users/", `{"time_zone":" UTC ","language":" EN "}`, [2]string{"UTC", "en"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			path := test.prefix + strconv.FormatInt(id, 10) + "/settings"
			assertMutationResponse(t, f.request(http.MethodPut, path, test.body), path, mutationResponseWant{http.StatusOK, "user settings updated"})
			requireRepositoryEqual(t, "normalized settings persisted", userSettingsFixtureState(t, f.app, id), test.want)
			assertMutationAudit(t, f.app, "user.settings.update", 1)
		})
	}
}

func TestUserSettingsMutationValidationPriorityAndFailures(t *testing.T) {
	for _, test := range []struct{ name, body, message string }{
		{"malformed", `{`, "invalid request body"},
		{"byte limit before zone and language", `{"time_zone":"` + strings.Repeat("界", 22) + `","language":"bad"}`, "time_zone is too long (max 64 characters)"},
		{"zone before language", `{"time_zone":"not-a-zone","language":"bad"}`, "time_zone must be a valid IANA timezone"},
		{"language", `{"time_zone":"UTC","language":"bad"}`, "language must be one of: ru, en"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			before := userSettingsFixtureState(t, f.app, id)
			path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/settings"
			assertMutationResponse(t, f.request(http.MethodPut, path, test.body), path, mutationResponseWant{http.StatusBadRequest, test.message})
			requireRepositoryEqual(t, "invalid input retains settings", userSettingsFixtureState(t, f.app, id), before)
			assertMutationAudit(t, f.app, "user.settings.update", 0)
		})
	}
}

func TestUserSettingsMutationMissingAndSQLFailure(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	path := "/api/v1/users/999999/settings"
	assertMutationResponse(t, f.request(http.MethodPut, path, `{}`), path, mutationResponseWant{http.StatusNotFound, "user not found"})
	before := userSettingsFixtureState(t, f.app, id)
	execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER reject_settings BEFORE UPDATE ON users BEGIN SELECT RAISE(ABORT, 'private settings fixture'); END`)
	path = "/api/v1/users/" + strconv.FormatInt(id, 10) + "/settings"
	assertMutationResponse(t, f.request(http.MethodPut, path, `{"time_zone":"UTC"}`), path, mutationResponseWant{http.StatusInternalServerError, "failed to update user settings"})
	requireRepositoryEqual(t, "SQL failure retains settings", userSettingsFixtureState(t, f.app, id), before)
	assertMutationAudit(t, f.app, "user.settings.update", 0)
}

// A reporting error cannot prove absence, even if the write already completed.
func TestUserSettingsRowsAffectedFailureDoesNotClaimMissingUser(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	faults.settingsRowsAffectedErr = errors.New("private settings reporting diagnostic")
	path := "/api/v1/users/" + strconv.FormatInt(id, 10) + "/settings"
	response := f.request(http.MethodPut, path, `{"time_zone":"UTC","language":"en"}`)
	requireRepositoryEqual(t, "already performed write is not claimed rolled back", userSettingsFixtureState(t, f.app, id), [2]string{"UTC", "en"})
	t.Logf("reporting failure response status: %d", response.Code)
	assertMutationResponse(t, response, path, mutationResponseWant{http.StatusInternalServerError, "failed to update user settings"})
	requireRepositoryEqual(t, "private diagnostic withheld", strings.Contains(response.Body.String(), "private settings"), false)
	assertMutationAudit(t, f.app, "user.settings.update", 0)
}
