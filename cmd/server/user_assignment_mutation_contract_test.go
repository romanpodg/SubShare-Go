package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

type userAssignmentFixture struct {
	userMutationFixture
	faults *mutationSQLFaults
	id     int64
	keys   [2]int64
}

type userAssignmentFixtureState struct {
	mode string
	keys []int64
}

func newUserAssignmentFixture(t *testing.T) userAssignmentFixture {
	t.Helper()
	f, faults := newFaultedMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	execRepositoryFixtureSQL(t, f.app, "UPDATE users SET key_assignment_mode = 'selected' WHERE id = ?", id)
	fixture := userAssignmentFixture{userMutationFixture: f, faults: faults, id: id}
	repository := storage.NewRepository(f.app.db, f.app.profileKeyring)
	for i := range fixture.keys {
		key, _, err := repository.CreateLocal(context.Background(), keymanagement.CreateProfileParams{
			Label: fmt.Sprintf("Assignment%d", i), Status: "active", Kind: "real", Protocol: "trojan",
			BuiltURI: fmt.Sprintf("trojan://fixture-%d@assignment.example:443#Fixture%d", i, i),
		})
		requireRepositorySuccess(t, err)
		fixture.keys[i] = key.ID
	}
	requireRepositorySuccess(t, f.app.updateUserKeyAssignment(id, model.KeyAssignmentModeSelected, []int64{fixture.keys[0]}))
	return fixture
}

func userAssignmentState(t *testing.T, f userAssignmentFixture) userAssignmentFixtureState {
	t.Helper()
	var state userAssignmentFixtureState
	requireRepositorySuccess(t, f.app.db.QueryRow("SELECT key_assignment_mode FROM users WHERE id = ?", f.id).Scan(&state.mode))
	rows, err := f.app.db.Query("SELECT key_id FROM user_keys WHERE user_id = ? ORDER BY key_id", f.id)
	requireRepositorySuccess(t, err)
	defer rows.Close()
	for rows.Next() {
		var id int64
		requireRepositorySuccess(t, rows.Scan(&id))
		state.keys = append(state.keys, id)
	}
	requireRepositorySuccess(t, rows.Err())
	return state
}

func TestUserAssignmentMutationRegisteredContracts(t *testing.T) {
	for _, test := range []struct {
		name, prefix, suffix, mode string
		indices                    []int
	}{
		{"all", "/api/v1/users/", "/key-assignment", "all", []int{0, 1}},
		{"selected duplicates", "/api/v1/users/", "/key-assignment", "selected", []int{1, 1}},
		{"selected empty", "/api/v1/users/", "/key-assignment", "selected", nil},
		{"v1 keys alias", "/api/v1/users/", "/keys", "selected", []int{1, 1}},
		{"legacy keys alias", "/api/admin/users/", "/keys", "selected", []int{1, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserAssignmentFixture(t)
			requestIDs := []int64{}
			for _, i := range test.indices {
				requestIDs = append(requestIDs, f.keys[i])
			}
			if test.mode == "all" {
				requestIDs = nil
			}
			body := map[string]any{"key_ids": requestIDs}
			if test.suffix == "/key-assignment" {
				body["mode"] = test.mode
			}
			path := test.prefix + strconv.FormatInt(f.id, 10) + test.suffix
			message := "subscription key assignment updated"
			if test.suffix == "/keys" {
				message = "subscription keys updated"
			}
			assertMutationResponse(t, f.request(http.MethodPut, path, mutationJSON(t, body)), path, mutationResponseWant{http.StatusOK, message})
			state := userAssignmentState(t, f)
			requireRepositoryEqual(t, "assignment mode retained", state.mode, test.mode)
			want := []int64{f.keys[1]}
			if test.mode == "all" {
				want = f.keys[:]
			}
			if len(test.indices) == 0 {
				want = nil
			}
			requireRepositoryEqual(t, "complete deduplicated assignment", state.keys, want)
			assertMutationAudit(t, f.app, "user.keys.update", 1)
		})
	}
}

func TestUserAssignmentMutationValidationRetainsState(t *testing.T) {
	for _, test := range []struct {
		name, body, message string
		status              int
	}{
		{"mode", `{"mode":"bad"}`, "mode must be all or selected", http.StatusBadRequest},
		{"all IDs", `{"mode":"all","key_ids":[-1]}`, "key_ids must be empty in all mode", http.StatusBadRequest},
		{"nonpositive selected ID", `{"mode":"selected","key_ids":[0]}`, "failed to update key assignment", http.StatusInternalServerError},
		{"missing key", `{"mode":"selected","key_ids":[999999]}`, "one or more keys do not exist", http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserAssignmentFixture(t)
			before := userAssignmentState(t, f)
			path := "/api/v1/users/" + strconv.FormatInt(f.id, 10) + "/key-assignment"
			assertMutationResponse(t, f.request(http.MethodPut, path, test.body), path, mutationResponseWant{test.status, test.message})
			requireRepositoryEqual(t, "invalid assignment retains all state", userAssignmentState(t, f), before)
			assertMutationAudit(t, f.app, "user.keys.update", 0)
		})
	}
}

func installUserAssignmentFailure(t *testing.T, f userAssignmentFixture, phase string) {
	t.Helper()
	switch phase {
	case "begin":
		f.faults.beginErr = errors.New("private assignment begin")
	case "user query":
		f.faults.queryErr = func(query string) error {
			if strings.Contains(query, "SELECT COUNT(*) FROM users") {
				return errors.New("private assignment query")
			}
			return nil
		}
	case "delete":
		execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER reject_assignment BEFORE DELETE ON user_keys BEGIN SELECT RAISE(ABORT, 'private assignment delete'); END`)
	case "insert":
		execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER reject_assignment BEFORE INSERT ON user_keys BEGIN SELECT RAISE(ABORT, 'private assignment insert'); END`)
	case "mode":
		execRepositoryFixtureSQL(t, f.app, `CREATE TRIGGER reject_assignment BEFORE UPDATE OF key_assignment_mode ON users BEGIN SELECT RAISE(ABORT, 'private assignment mode'); END`)
	case "commit":
		f.faults.commitErr = errors.New("private assignment commit")
	default:
		t.Fatalf("unknown phase %s", phase)
	}
}

func TestUserAssignmentMutationFailureRollsBack(t *testing.T) {
	for _, phase := range []string{"begin", "user query", "delete", "insert", "mode", "commit"} {
		t.Run(phase, func(t *testing.T) {
			f := newUserAssignmentFixture(t)
			before := userAssignmentState(t, f)
			installUserAssignmentFailure(t, f, phase)
			path := "/api/v1/users/" + strconv.FormatInt(f.id, 10) + "/key-assignment"
			body := mutationJSON(t, map[string]any{"mode": "all", "key_ids": []int64{}})
			assertMutationResponse(t, f.request(http.MethodPut, path, body), path, mutationResponseWant{http.StatusInternalServerError, "failed to update key assignment"})
			f.faults.queryErr = nil
			requireRepositoryEqual(t, "failed transaction retains assignments and mode", userAssignmentState(t, f), before)
			assertMutationAudit(t, f.app, "user.keys.update", 0)
		})
	}
}

func TestUserAssignmentMutationMissingUserRetainsOtherUser(t *testing.T) {
	f := newUserAssignmentFixture(t)
	before := userAssignmentState(t, f)
	path := "/api/v1/users/999999/key-assignment"
	assertMutationResponse(t, f.request(http.MethodPut, path, `{"mode":"all"}`), path, mutationResponseWant{http.StatusNotFound, "user not found"})
	requireRepositoryEqual(t, "missing user leaves other assignments", userAssignmentState(t, f), before)
	assertMutationAudit(t, f.app, "user.keys.update", 0)
}
