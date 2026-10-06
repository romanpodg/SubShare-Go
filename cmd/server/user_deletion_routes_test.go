package main

import (
	"net/http"
	"strconv"
	"testing"
)

func seedMutationDeletionState(t *testing.T, app *App) int64 {
	t.Helper()
	userID := seedSubscriptionUser(t, app, "active")
	profile := seedStartupEncryptedProfile(t, app.db, app.profileKeyring)
	execRepositoryFixtureSQL(t, app, `INSERT INTO users(name, token, subscription_id) VALUES('other-user', 'other-token', 'other-subscription');
		INSERT INTO user_keys SELECT id, ? FROM users;
		INSERT INTO user_devices(user_id, hwid, normalized_hwid) SELECT id, 'device', 'device' FROM users`, profile.id)
	return userID
}

func TestUserDeletionRegisteredRoutesCascadeOnlyTarget(t *testing.T) {
	for _, prefix := range []string{"/api/admin/users/", "/api/v1/users/"} {
		t.Run(prefix, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationDeletionState(t, f.app)
			path := prefix + strconv.FormatInt(id, 10)
			assertMutationResponse(t, f.request(http.MethodDelete, path, ""), path, mutationResponseWant{status: http.StatusOK, message: "user deleted"})
			assertMutationUserCounts(t, f.app, 1, 1)
			requireRepositoryEqual(t, "target devices cascaded", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices WHERE user_id = ?", id), 0)
			requireRepositoryEqual(t, "other device retained", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices"), 1)
			requireRepositoryEqual(t, "other user retained", mutationCount(t, f.app, "SELECT COUNT(*) FROM users WHERE name = 'other-user'"), 1)
			requireRepositoryEqual(t, "profile retained", mutationCount(t, f.app, "SELECT COUNT(*) FROM vless_keys"), 1)
			requireRepositoryEqual(t, "encrypted secret retained", mutationCount(t, f.app, "SELECT COUNT(*) FROM vless_key_secrets"), 1)
			assertMutationAudit(t, f.app, "user.delete", 1)
			assertMutationResponse(t, f.request(http.MethodDelete, path, ""), path, mutationResponseWant{status: http.StatusNotFound, message: "user not found"})
			assertMutationAudit(t, f.app, "user.delete", 1)
		})
	}
}

func TestUserDeletionRejectsInvalidAndMissingIDs(t *testing.T) {
	f := newUserMutationFixture(t)
	seedMutationDeletionState(t, f.app)
	for _, prefix := range []string{"/api/admin/users/", "/api/v1/users/"} {
		for _, id := range []string{"0", "-1", "invalid", "99999"} {
			status, message := http.StatusBadRequest, "invalid id"
			if id == "99999" {
				status, message = http.StatusNotFound, "user not found"
			}
			path := prefix + id
			assertMutationResponse(t, f.request(http.MethodDelete, path, ""), path, mutationResponseWant{status: status, message: message})
		}
	}
	assertMutationUserCounts(t, f.app, 2, 2)
	assertMutationAudit(t, f.app, "user.delete", 0)
}

func TestUserDeletionCascadeFailureRollsBack(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationDeletionState(t, f.app)
	execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER reject_device_delete BEFORE DELETE ON user_devices
		BEGIN SELECT RAISE(ABORT, 'injected cascade failure'); END`)
	path := "/api/v1/users/" + strconv.FormatInt(id, 10)
	assertMutationResponse(t, f.request(http.MethodDelete, path, ""), path, mutationResponseWant{status: http.StatusInternalServerError, message: "failed to delete user"})
	assertMutationUserCounts(t, f.app, 2, 2)
	requireRepositoryEqual(t, "failed cascade retains devices", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices"), 2)
	assertMutationAudit(t, f.app, "user.delete", 0)
	execRepositoryFixtureSQL(t, f.app, "DROP TRIGGER reject_device_delete")
	assertMutationResponse(t, f.request(http.MethodDelete, path, ""), path, mutationResponseWant{status: http.StatusOK, message: "user deleted"})
}
