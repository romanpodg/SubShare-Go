package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func isDeviceRegistrationWrite(query string) bool {
	return strings.HasPrefix(strings.TrimSpace(query), "UPDATE user_devices")
}

func synchronizeDeviceRegistrations(t *testing.T, faults *mutationSQLFaults) *atomic.Int32 {
	t.Helper()
	var ready atomic.Int32
	gate := make(chan struct{})
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	faults.beforeExec = func(query string) {
		if !isDeviceRegistrationWrite(query) {
			return
		}
		if ready.Add(1) == 2 {
			close(gate)
		}
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	return &ready
}

func TestUserDeviceConcurrentTransactionsRetainCapacity(t *testing.T) {
	for _, test := range []struct {
		name, first, second string
		seedExisting        bool
		accepted, count     int
	}{
		{"last slot", "one", "two", true, 1, 2},
		{"same identity", " Device ", "DEVICE", false, 2, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			id := seedSubscriptionUser(t, f.app, "active")
			if test.seedExisting {
				execRepositoryFixtureSQL(t, f.app, "UPDATE users SET max_devices = 2 WHERE id = ?", id)
				allowed, err := f.app.registerHWID(id, "existing", deviceMeta{})
				requireRepositorySuccess(t, err)
				requireRepositoryEqual(t, "existing device occupies first slot", allowed, true)
			}
			ready := synchronizeDeviceRegistrations(t, faults)
			calls := make([]func() mutationConcurrentResult, 0, 2)
			for _, hwid := range []string{test.first, test.second} {
				calls = append(calls, func() mutationConcurrentResult {
					allowed, err := f.app.registerHWID(id, hwid, deviceMeta{AppVersion: hwid})
					return mutationConcurrentResult{allowed: allowed, err: err}
				})
			}
			results := runConcurrentMutations(t, calls)
			requireRepositoryEqual(t, "both transactions reached their first write", ready.Load(), int32(2))
			accepted := 0
			for _, result := range results {
				requireRepositorySuccess(t, result.err)
				if result.allowed {
					accepted++
				}
			}
			requireRepositoryEqual(t, "accepted registration count", accepted, test.accepted)
			requireRepositoryEqual(t, "persisted capacity retained", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices WHERE user_id = ?", id), test.count)
		})
	}
}

type deviceRegistrationSnapshot struct {
	version         string
	created, seenAt sql.NullTime
}

func readDeviceRegistrationSnapshot(t *testing.T, app *App, id int64) deviceRegistrationSnapshot {
	t.Helper()
	var state deviceRegistrationSnapshot
	requireRepositorySuccess(t, app.db.QueryRow("SELECT app_version, created_at, last_seen_at FROM user_devices WHERE user_id = ? AND normalized_hwid = 'device'", id).Scan(&state.version, &state.created, &state.seenAt))
	return state
}

func TestUserDeviceRegistrationFailuresRollback(t *testing.T) {
	for _, phase := range []string{"begin", "update", "confirmation", "insert", "commit"} {
		t.Run(phase, func(t *testing.T) {
			f, faults := newFaultedMutationFixture(t)
			id := seedSubscriptionUser(t, f.app, "active")
			execRepositoryFixtureSQL(t, f.app, `UPDATE users SET max_devices = 2 WHERE id = ?;
				INSERT INTO user_devices(user_id, hwid, normalized_hwid, app_version, created_at, last_seen_at)
				VALUES(?, 'Device', 'device', 'old', '2000-01-01 00:00:00', '2000-01-02 00:00:00')`, id, id)
			before := readDeviceRegistrationSnapshot(t, f.app, id)
			hwid, cause := installDeviceRegistrationFailure(t, f, faults, phase)
			allowed, err := f.app.registerHWID(id, hwid, deviceMeta{AppVersion: "new"})
			requireRepositoryEqual(t, "failed registration denied", allowed, false)
			requireRepositoryEqual(t, "failure cause returned", err != nil, true)
			if cause != nil {
				requireRepositoryEqual(t, "driver cause retained", errors.Is(err, cause), true)
			}
			requireRepositoryEqual(t, "metadata and timestamps rolled back", readDeviceRegistrationSnapshot(t, f.app, id), before)
			requireRepositoryEqual(t, "failed command adds no device", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices WHERE user_id = ?", id), 1)
			faults.beginErr, faults.commitErr, faults.deviceRowsAffectedErr = nil, nil, nil
			execRepositoryFixtureSQL(t, f.app, "DROP TRIGGER IF EXISTS reject_registration")
			allowed, err = f.app.registerHWID(id, hwid, deviceMeta{AppVersion: "new"})
			requireRepositorySuccess(t, err)
			requireRepositoryEqual(t, "failure releases registration transaction", allowed, true)
		})
	}
}

func installDeviceRegistrationFailure(t *testing.T, f userMutationFixture, faults *mutationSQLFaults, phase string) (string, error) {
	t.Helper()
	cause := errors.New("private registration diagnostic")
	switch phase {
	case "begin":
		faults.beginErr = cause
	case "confirmation":
		faults.deviceRowsAffectedErr = cause
	case "commit":
		faults.commitErr = cause
	case "update":
		execRepositoryFixtureSQL(t, f.app, "CREATE TRIGGER reject_registration BEFORE UPDATE ON user_devices BEGIN SELECT RAISE(ABORT, 'private update diagnostic'); END")
		return "device", nil
	case "insert":
		execRepositoryFixtureSQL(t, f.app, "CREATE TRIGGER reject_registration BEFORE INSERT ON user_devices BEGIN SELECT RAISE(ABORT, 'private insert diagnostic'); END")
		return "new", nil
	}
	return "device", cause
}

func TestUserDeviceRegistrationRetainsExplicitIdentity(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedSubscriptionUser(t, f.app, "active")
	for _, raw := range []string{" First ", "Other"} {
		allowed, err := f.app.registerHWID(id, raw, deviceMeta{NormalizedHWID: " Shared ", AppVersion: raw})
		requireRepositorySuccess(t, err)
		requireRepositoryEqual(t, "explicit normalized identity accepted at capacity", allowed, true)
	}
	var raw, normalized, version string
	requireRepositorySuccess(t, f.app.db.QueryRow("SELECT hwid, normalized_hwid, app_version FROM user_devices WHERE user_id = ?", id).Scan(&raw, &normalized, &version))
	requireRepositoryEqual(t, "initial raw identity retained", raw, "First")
	requireRepositoryEqual(t, "explicit identity trimmed and lowercased", normalized, "shared")
	requireRepositoryEqual(t, "existing identity metadata refreshed", version, "Other")
	requireRepositoryEqual(t, "normalized identity does not consume another slot", mutationCount(t, f.app, "SELECT COUNT(*) FROM user_devices WHERE user_id = ?", id), 1)
	allowed, err := (&App{}).registerHWID(999, " \t", deviceMeta{})
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "empty optional identity bypasses persistence", allowed, true)
}
