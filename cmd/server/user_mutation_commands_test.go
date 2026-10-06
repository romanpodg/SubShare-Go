package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

func userCommandInput(t *testing.T) createUserInput {
	t.Helper()
	input, message := validateCreateUserRequest(model.CreateUserRequest{
		Name: "Alice", Email: "alice@example.test", ActivationCode: "custom-code",
	})
	requireRepositoryEqual(t, "command fixture validated", message, "")
	return input
}

func TestUserCommandsKeepAuditInHTTP(t *testing.T) {
	f := newUserMutationFixture(t)
	profile := seedStartupEncryptedProfile(t, f.app.db, f.app.profileKeyring)
	id, err := f.app.createUser(userCommandInput(t))
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "committed identity returned", id > 0, true)
	assertMutationUserCounts(t, f.app, 1, 1)
	requireRepositoryEqual(t, "returned identity owns assigned profile", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_keys WHERE user_id = ? AND key_id = ?", id, profile.id), 1)
	execRepositoryFixtureSQL(t, f.app, "INSERT INTO user_devices(user_id, hwid) VALUES(?, 'device')", id)
	assertMutationAudit(t, f.app, "user.create", 0)
	requireRepositorySuccess(t, f.app.deleteUser(id))
	assertMutationUserCounts(t, f.app, 0, 0)
	requireRepositoryEqual(t, "command deletion cascades devices", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices"), 0)
	requireRepositoryEqual(t, "shared profile retained", mutationCount(t, f.app, "SELECT COUNT(*) FROM vless_keys"), 1)
	assertMutationAudit(t, f.app, "user.delete", 0)
	requireRepositoryEqual(t, "missing user command result", errors.Is(f.app.deleteUser(id), errDeleteUserNotFound), true)
}

func TestUserCreateCommandDoesNotReturnProvisionalIdentity(t *testing.T) {
	for _, phase := range []string{"begin", "insert", "assignment", "commit"} {
		t.Run(phase, func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			seedStartupEncryptedProfile(t, f.app.db, f.app.profileKeyring)
			installUserCommandFailure(t, f, faults, phase)
			id, err := f.app.createUser(userCommandInput(t))
			requireRepositoryEqual(t, "failed command reports error", err != nil, true)
			requireRepositoryEqual(t, "provisional identity withheld", id, int64(0))
			assertMutationUserCounts(t, f.app, 0, 0)
			assertMutationAudit(t, f.app, "user.create", 0)
		})
	}
}

func installUserCommandFailure(t *testing.T, f userMutationFixture, faults *mutationSQLFaults, phase string) {
	t.Helper()
	switch phase {
	case "begin":
		faults.beginErr = errors.New("injected begin")
	case "insert":
		execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER reject_command BEFORE INSERT ON users BEGIN SELECT RAISE(ABORT, 'injected insert'); END`)
	case "assignment":
		execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER reject_command BEFORE INSERT ON user_keys BEGIN SELECT RAISE(ABORT, 'injected assignment'); END`)
	case "commit":
		faults.commitErr = errors.New("injected commit")
	}
}

func TestUserCommandWrappedTokenFailureRetainsResponse(t *testing.T) {
	err := fmt.Errorf("%w: %w", errCreateUserInsert,
		fmt.Errorf("%w: %w", errCreateSubscriptionToken, errors.New("private entropy diagnostic")))
	handler := httpapi.V1Envelope(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCreateUserCommandError(w, r, err)
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/users", nil))
	requireRepositoryEqual(t, "wrapped generation failure remains server error", recorder.Code, http.StatusInternalServerError)
	payload := decodeJSONMap(t, recorder)
	requireRepositoryEqual(t, "generation error priority retained", payload["error"], "failed to generate subscription token")
	requireRepositoryEqual(t, "private command cause not serialized", strings.Contains(recorder.Body.String(), "private"), false)
}
