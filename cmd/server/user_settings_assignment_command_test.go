package main

import (
	"errors"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestUserSettingsCommandKeepsAuditInHTTP(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	before := readMutationSubscription(t, f.app, id)
	next, err := f.app.updateUserSettings(id, model.UpdateUserSettingsRequest{})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "normalized command result", next, userSettingsInput{timeZone: "Europe/Moscow", language: "ru"})
	requireRepositoryEqual(t, "settings persisted", userSettingsFixtureState(t, f.app, id), [2]string{"Europe/Moscow", "ru"})
	requireRepositoryEqual(t, "subscription state retained", readMutationSubscription(t, f.app, id), before)
	_, err = f.app.updateUserSettings(999999, model.UpdateUserSettingsRequest{})
	requireRepositoryEqual(t, "confirmed missing target", errors.Is(err, errUserSettingsNotFound), true)
	assertMutationAudit(t, f.app, "user.settings.update", 0)
}

func TestUserSettingsCommandValidationRetainsState(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	before := userSettingsFixtureState(t, f.app, id)
	next, err := f.app.updateUserSettings(id, model.UpdateUserSettingsRequest{TimeZone: "not-a-zone", Language: "bad"})
	var invalid *userSettingsValidationError
	requireRepositoryEqual(t, "validation cause classified", errors.As(err, &invalid), true)
	requireRepositoryEqual(t, "timezone validation precedes language", err.Error(), "time_zone must be a valid IANA timezone")
	requireRepositoryEqual(t, "no provisional result", next, userSettingsInput{})
	requireRepositoryEqual(t, "invalid input retains settings", userSettingsFixtureState(t, f.app, id), before)
	assertMutationAudit(t, f.app, "user.settings.update", 0)
}

func TestUserSettingsCommandRetainsReportingCauseAndPerformedWrite(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	cause := errors.New("private command reporting fixture")
	faults.settingsRowsAffectedErr = cause
	next, err := f.app.updateUserSettings(id, model.UpdateUserSettingsRequest{TimeZone: "UTC", Language: "en"})
	requireRepositoryEqual(t, "reporting cause retained", errors.Is(err, cause), true)
	requireRepositoryEqual(t, "unknown outcome is not confirmed absence", errors.Is(err, errUserSettingsNotFound), false)
	requireRepositoryEqual(t, "no successful command result", next, userSettingsInput{})
	requireRepositoryEqual(t, "performed write not claimed rolled back", userSettingsFixtureState(t, f.app, id), [2]string{"UTC", "en"})
	assertMutationAudit(t, f.app, "user.settings.update", 0)
}

func TestUserAssignmentCommandKeepsAuditInHTTPAndValidatesBeforeWrite(t *testing.T) {
	f := newUserAssignmentFixture(t)
	requireRepositorySuccess(t, f.app.updateUserKeyAssignment(f.id, "selected", []int64{f.keys[1], f.keys[1]}))
	want := userAssignmentFixtureState{mode: "selected", keys: []int64{f.keys[1]}}
	requireRepositoryEqual(t, "deduplicated command assignment", userAssignmentState(t, f), want)
	err := f.app.updateUserKeyAssignment(f.id, "selected", []int64{0})
	requireRepositoryEqual(t, "invalid ID rejected", err != nil, true)
	requireRepositoryEqual(t, "validation retains prior assignment", userAssignmentState(t, f), want)
	assertMutationAudit(t, f.app, "user.keys.update", 0)
}

func TestUserAssignmentCommandCommitFailureRollsBackAndRetainsCause(t *testing.T) {
	f := newUserAssignmentFixture(t)
	before := userAssignmentState(t, f)
	cause := errors.New("private assignment command commit")
	f.faults.commitErr = cause
	err := f.app.updateUserKeyAssignment(f.id, "all", nil)
	requireRepositoryEqual(t, "commit cause retained", errors.Is(err, cause), true)
	requireRepositoryEqual(t, "transaction rolled back mode and assignments", userAssignmentState(t, f), before)
	assertMutationAudit(t, f.app, "user.keys.update", 0)
}
