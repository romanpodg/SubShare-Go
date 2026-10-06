package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestUserCreationRegisteredRoutesPreserveDefaults(t *testing.T) {
	for _, path := range []string{"/api/admin/users", "/api/v1/users"} {
		t.Run(path, func(t *testing.T) {
			f := newUserMutationFixture(t)
			profile := seedStartupEncryptedProfile(t, f.app.db, f.app.profileKeyring)
			before := time.Now().UTC()
			response := f.request(http.MethodPost, path, `{"name":" Alice ","email":" alice@example.test ","activation_code":" custom-code ","blocked_reason":"unused"}`)
			assertMutationResponse(t, response, path, mutationResponseWant{status: http.StatusOK, message: "user created"})
			assertCreatedMutationUser(t, f.app, createdMutationUserWant{days: 30, status: "active", reason: ""}, before)
			requireRepositoryEqual(t, "initial profile assigned", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_keys WHERE key_id = ?", profile.id), 1)
			assertMutationAudit(t, f.app, "user.create", 1)
		})
	}
}

type createdMutationUserWant struct {
	days           int
	status, reason string
}

func assertCreatedMutationUser(t *testing.T, app *App, want createdMutationUserWant, before time.Time) {
	t.Helper()
	var name, email, activation, token, subscription, actualStatus, actualReason, assignment string
	var starts, expires time.Time
	var devices int
	err := app.db.QueryRow(`SELECT name, email, activation_code, token, subscription_id, status,
		COALESCE(blocked_reason, ''), starts_at, expires_at, max_devices, key_assignment_mode FROM users`).Scan(
		&name, &email, &activation, &token, &subscription, &actualStatus, &actualReason, &starts, &expires, &devices, &assignment)
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "created name trimmed", name, "Alice")
	requireRepositoryEqual(t, "created email trimmed", email, "alice@example.test")
	requireRepositoryEqual(t, "custom activation code trimmed", activation, "custom-code")
	requireRepositoryEqual(t, "created status", actualStatus, want.status)
	requireRepositoryEqual(t, "blocked reason normalization", actualReason, want.reason)
	requireRepositoryEqual(t, "independent generated identities", token != "" && subscription != "" && token != subscription, true)
	requireRepositoryEqual(t, "created device limit", devices, 1)
	requireRepositoryEqual(t, "created assignment mode", assignment, "all")
	requireRepositoryEqual(t, "created start within request", !starts.Before(before) && !starts.After(time.Now().UTC()), true)
	requireRepositoryEqual(t, "created subscription duration", expires.Equal(starts.AddDate(0, 0, want.days)), true)
}

func assertMutationAudit(t *testing.T, app *App, action string, count int) {
	t.Helper()
	requireRepositoryEqual(t, "mutation audit count", mutationCount(t, app, "SELECT COUNT(*) FROM audit_events WHERE action = ?", action), count)
}

func TestUserCreationDurationAndStatusBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, status, reason string
		days, wantDays       int
	}{
		{"zero defaults", "active", "", 0, 30},
		{"negative defaults", "paused", "", -1, 30},
		{"minimum", "active", "", 1, 1},
		{"maximum blocked", "blocked", "billing hold", 3650, 3650},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			before := time.Now().UTC()
			body := mutationJSON(t, model.CreateUserRequest{Name: "Alice", Email: "alice@example.test", ActivationCode: "custom-code", Status: test.status, IssueDays: test.days, BlockedReason: " " + test.reason + " "})
			assertMutationResponse(t, f.request(http.MethodPost, "/api/v1/users", body), "/api/v1/users", mutationResponseWant{status: http.StatusOK, message: "user created"})
			assertCreatedMutationUser(t, f.app, createdMutationUserWant{days: test.wantDays, status: test.status, reason: test.reason}, before)
		})
	}
}

func TestUserCreationRejectsInvalidInputsWithoutWrites(t *testing.T) {
	for _, test := range []struct{ name, body, message string }{
		{"status precedes duration", `{"status":"invalid","issue_days":3651}`, "invalid subscription status"},
		{"duration precedes required fields", `{"issue_days":3651}`, "issue days must be between 1 and 3650"},
		{"malformed", "{", "invalid request body"},
		{"unknown field", `{"name":"Alice","activation_code":"code","unknown":true}`, "invalid request body"},
		{"trailing JSON", `{"name":"Alice","activation_code":"code"}{}`, "invalid request body"},
		{"oversized body", `{"name":"` + strings.Repeat("x", 1<<20) + `"}`, "invalid request body"},
		{"blank name", `{"name":" ","activation_code":"code"}`, "name is required"},
		{"missing code", `{"name":"Alice"}`, "activation code is required"},
		{"slash code", `{"name":"Alice","activation_code":"bad/code"}`, "activation code is required"},
		{"invalid status", `{"name":"Alice","activation_code":"code","status":"unknown"}`, "invalid subscription status"},
		{"too many days", `{"name":"Alice","activation_code":"code","issue_days":3651}`, "issue days must be between 1 and 3650"},
		{"long name", mutationJSON(t, model.CreateUserRequest{Name: strings.Repeat("x", 256), ActivationCode: "code"}), "name is too long (max 255 characters)"},
		{"long email", mutationJSON(t, model.CreateUserRequest{Name: "Alice", Email: strings.Repeat("x", 256), ActivationCode: "code"}), "email is too long (max 255 characters)"},
		{"long code", mutationJSON(t, model.CreateUserRequest{Name: "Alice", ActivationCode: strings.Repeat("x", 129)}), "activation code is too long (max 128 characters)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			for _, path := range []string{"/api/admin/users", "/api/v1/users"} {
				assertMutationResponse(t, f.request(http.MethodPost, path, test.body), path, mutationResponseWant{status: http.StatusBadRequest, message: test.message})
			}
			assertMutationUserCounts(t, f.app, 0, 0)
			assertMutationAudit(t, f.app, "user.create", 0)
		})
	}
}

func assertMutationUserCounts(t *testing.T, app *App, users, assignments int) {
	t.Helper()
	requireRepositoryEqual(t, "persisted user count", mutationCount(t, app, "SELECT COUNT(*) FROM users"), users)
	requireRepositoryEqual(t, "persisted assignment count", mutationCount(t, app, "SELECT COUNT(*) FROM user_keys"), assignments)
}

func TestUserCreationDuplicateCodeRetainsOriginal(t *testing.T) {
	f := newUserMutationFixture(t)
	userID := seedSubscriptionUser(t, f.app, "active")
	for _, path := range []string{"/api/admin/users", "/api/v1/users"} {
		response := f.request(http.MethodPost, path, `{"name":"replacement","activation_code":"activation-token"}`)
		assertMutationResponse(t, response, path, mutationResponseWant{status: http.StatusConflict, message: "failed to create user (check activation code uniqueness)"})
	}
	assertMutationUserCounts(t, f.app, 1, 0)
	var name string
	requireRepositorySuccess(t, f.app.db.QueryRow("SELECT name FROM users WHERE id = ?", userID).Scan(&name))
	requireRepositoryEqual(t, "duplicate create retains original user", name, "Alice")
	assertMutationAudit(t, f.app, "user.create", 0)
}

func TestUserCreationFieldLengthBoundaries(t *testing.T) {
	for _, test := range []struct {
		name    string
		input   model.CreateUserRequest
		status  int
		message string
	}{
		{"maximum ASCII", model.CreateUserRequest{Name: strings.Repeat("x", 255), Email: strings.Repeat("x", 255), ActivationCode: strings.Repeat("x", 128)}, http.StatusOK, "user created"},
		{"Unicode byte boundary", model.CreateUserRequest{Name: strings.Repeat("界", 85), ActivationCode: "code"}, http.StatusOK, "user created"},
		{"Unicode above byte boundary", model.CreateUserRequest{Name: strings.Repeat("界", 86), ActivationCode: "code"}, http.StatusBadRequest, "name is too long (max 255 characters)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			path := "/api/v1/users"
			assertMutationResponse(t, f.request(http.MethodPost, path, mutationJSON(t, test.input)), path, mutationResponseWant{test.status, test.message})
			wantCount := 0
			if test.status == http.StatusOK {
				wantCount = 1
			}
			assertMutationUserCounts(t, f.app, wantCount, 0)
		})
	}
}

func TestUserCreationSQLFailuresRollBack(t *testing.T) {
	for _, test := range []struct {
		name, trigger string
		status        int
	}{
		{"user insert", `CREATE TRIGGER reject_create BEFORE INSERT ON users BEGIN SELECT RAISE(ABORT, 'injected user insert'); END`, http.StatusConflict},
		{"assignment insert", `CREATE TRIGGER reject_create BEFORE INSERT ON user_keys BEGIN SELECT RAISE(ABORT, 'injected assignment insert'); END`, http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			seedStartupEncryptedProfile(t, f.app.db, f.app.profileKeyring)
			execRepositoryFixtureSQL(t, f.app, test.trigger)
			message := "failed to create user"
			if test.status == http.StatusConflict {
				message += " (check activation code uniqueness)"
			}
			assertMutationResponse(t, f.request(http.MethodPost, "/api/v1/users", `{"name":"Alice","activation_code":"custom-code"}`), "/api/v1/users", mutationResponseWant{status: test.status, message: message})
			assertMutationUserCounts(t, f.app, 0, 0)
			assertMutationAudit(t, f.app, "user.create", 0)
			execRepositoryFixtureSQL(t, f.app, "DROP TRIGGER reject_create")
			assertMutationResponse(t, f.request(http.MethodPost, "/api/v1/users", `{"name":"Alice","activation_code":"custom-code"}`), "/api/v1/users", mutationResponseWant{status: http.StatusOK, message: "user created"})
			assertMutationUserCounts(t, f.app, 1, 1)
		})
	}
}

func TestUserCreationTransactionFailuresDoNotPublish(t *testing.T) {
	for _, phase := range []string{"begin", "commit"} {
		t.Run(phase, func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			seedStartupEncryptedProfile(t, f.app.db, f.app.profileKeyring)
			if phase == "begin" {
				faults.beginErr = errors.New("injected begin")
			} else {
				faults.commitErr = errors.New("injected commit")
			}
			assertMutationResponse(t, f.request(http.MethodPost, "/api/v1/users", `{"name":"Alice","activation_code":"custom-code"}`), "/api/v1/users", mutationResponseWant{status: http.StatusInternalServerError, message: "failed to create user"})
			assertMutationUserCounts(t, f.app, 0, 0)
			assertMutationAudit(t, f.app, "user.create", 0)
			faults.beginErr, faults.commitErr = nil, nil
			assertMutationResponse(t, f.request(http.MethodPost, "/api/v1/users", `{"name":"Alice","activation_code":"custom-code"}`), "/api/v1/users", mutationResponseWant{status: http.StatusOK, message: "user created"})
			assertMutationUserCounts(t, f.app, 1, 1)
		})
	}
}

func installMutationSubscriptionCollisions(t *testing.T, app *App, failures int) {
	t.Helper()
	// FAIL preserves the trigger counter inside the transaction, allowing real
	// SQLite errors on the first N attempts without changing the token source.
	execRepositoryFixtureSQL(t, app, fmt.Sprintf(`CREATE TABLE create_attempts(count INTEGER NOT NULL);
		INSERT INTO create_attempts VALUES(0);
		CREATE TRIGGER create_collisions BEFORE INSERT ON users BEGIN
		UPDATE create_attempts SET count = count + 1;
		SELECT RAISE(FAIL, 'UNIQUE constraint failed: users.subscription_id') FROM create_attempts WHERE count <= %d;
		END`, failures))
}

func TestUserCreationRetriesSubscriptionCollision(t *testing.T) {
	f := newUserMutationFixture(t)
	installMutationSubscriptionCollisions(t, f.app, 4)
	assertMutationResponse(t, f.request(http.MethodPost, "/api/v1/users", `{"name":"Alice","activation_code":"custom-code"}`), "/api/v1/users", mutationResponseWant{status: http.StatusOK, message: "user created"})
	assertMutationUserCounts(t, f.app, 1, 0)
	requireRepositoryEqual(t, "bounded create retries", mutationCount(t, f.app, "SELECT count FROM create_attempts"), 5)
}

func TestUserCreationExhaustedRetriesReturnConflict(t *testing.T) {
	for _, withKeys := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing_keys=%t", withKeys), func(t *testing.T) {
			f := newUserMutationFixture(t)
			if withKeys {
				seedStartupEncryptedProfile(t, f.app.db, f.app.profileKeyring)
			}
			installMutationSubscriptionCollisions(t, f.app, 5)
			response := f.request(http.MethodPost, "/api/v1/users", `{"name":"Alice","activation_code":"custom-code"}`)
			assertMutationResponse(t, response, "/api/v1/users", mutationResponseWant{status: http.StatusConflict, message: "failed to create user (check activation code uniqueness)"})
			assertMutationUserCounts(t, f.app, 0, 0)
			assertMutationAudit(t, f.app, "user.create", 0)
			requireRepositoryEqual(t, "failed retry side effects rolled back", mutationCount(t, f.app, "SELECT count FROM create_attempts"), 0)
		})
	}
}
