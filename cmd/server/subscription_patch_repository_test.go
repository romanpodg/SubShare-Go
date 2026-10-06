package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func TestSubscriptionPatchSnapshotPreservesNullAndPreciseDates(t *testing.T) {
	for _, test := range []struct {
		name                  string
		starts, expires, zone any
	}{
		{"NULL dates", nil, nil, ""},
		{"literal precision", "2026-01-01T03:04:05.123456789+03:00", "2027-01-01T00:00:00Z", " UTC "},
		{"driver precision", time.Date(2026, 1, 1, 0, 0, 0, 987654321, time.UTC), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), "UTC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newUserMutationFixture(t)
			id := seedMutationSubscription(t, f.app)
			execRepositoryFixtureSQL(t, f.app, `UPDATE users SET starts_at = ?, expires_at = ?, time_zone = ?,
				blocked_reason = NULL, subscription_info_url = NULL, subscription_extra_url = NULL,
				subscription_extra_status = NULL WHERE id = ?`, test.starts, test.expires, test.zone, id)
			before := readSubscriptionPatchTimes(t, f.app, id)
			response := f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"subscription_name":"changed title"}`)
			assertMutationResponse(t, response, "/api/v1/users/1/subscription", mutationResponseWant{http.StatusOK, "subscription updated"})
			assertSubscriptionPatchTimesEqual(t, readSubscriptionPatchTimes(t, f.app, id), before)
			requireRepositoryEqual(t, "NULL overrides retained", mutationCount(t, f.app, `SELECT COUNT(*) FROM users WHERE id = ?
				AND blocked_reason IS NULL AND subscription_info_url IS NULL AND subscription_extra_url IS NULL AND subscription_extra_status IS NULL`, id), 1)
		})
	}
}

func readSubscriptionPatchTimes(t *testing.T, app *App, id int64) [2]sql.NullTime {
	t.Helper()
	var values [2]sql.NullTime
	requireRepositorySuccess(t, app.db.QueryRow("SELECT starts_at, expires_at FROM users WHERE id = ?", id).Scan(&values[0], &values[1]))
	return values
}

func assertSubscriptionPatchTimesEqual(t *testing.T, got, want [2]sql.NullTime) {
	t.Helper()
	for index := range got {
		requireRepositoryEqual(t, "nullable date presence retained", got[index].Valid, want[index].Valid)
		requireRepositoryEqual(t, "date instant and precision retained", got[index].Time.Equal(want[index].Time), true)
	}
}

func TestSubscriptionPatchSnapshotDetectsIndependentDatabaseWriter(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	snapshot, err := loadSubscriptionPatchSnapshot(t.Context(), f.app.db, id)
	requireRepositorySuccess(t, err)
	var seq int
	var name, path string
	requireRepositorySuccess(t, f.app.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path))
	other, err := sql.Open("sqlite", filepath.ToSlash(path)+"?_pragma=busy_timeout%3D5000")
	requireRepositorySuccess(t, err)
	t.Cleanup(func() { _ = other.Close() })
	_, err = other.Exec("UPDATE users SET status = 'paused', blocked_reason = NULL WHERE id = ?", id)
	requireRepositorySuccess(t, err)
	next := snapshot.state
	next.name = sql.NullString{String: "changed title", Valid: true}
	written, err := snapshot.store(t.Context(), f.app.db, id, next)
	requireRepositorySuccess(t, err)
	requireRepositoryEqual(t, "stale snapshot rejected across database handles", written, false)
	state := readMutationSubscription(t, f.app, id)
	requireRepositoryEqual(t, "independent writer status retained", state.status, "paused")
	requireRepositoryEqual(t, "stale name not published", state.name, "Personal title")
}

func TestSubscriptionPatchRowsAffectedFailureDoesNotReportSuccess(t *testing.T) {
	f, faults := newFaultedMutationFixture(t)
	seedMutationSubscription(t, f.app)
	faults.patchRowsAffectedErr = errors.New("injected private rows-affected diagnostic")
	response := f.request(http.MethodPatch, "/api/v1/users/1/subscription", `{"subscription_name":"changed title"}`)
	assertMutationResponse(t, response, "/api/v1/users/1/subscription", mutationResponseWant{http.StatusInternalServerError, "failed to update subscription"})
	requireRepositoryEqual(t, "rows-affected failure error code", decodeJSONMap(t, response)["code"], "subscription_update_failed")
	assertMutationAudit(t, f.app, "user.subscription.update", 0)
}

func TestSubscriptionPatchRespectsCancelledContext(t *testing.T) {
	f := newUserMutationFixture(t)
	id := seedMutationSubscription(t, f.app)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := f.app.patchUserSubscription(ctx, id, model.PatchSubscriptionRequest{})
	requireRepositoryEqual(t, "cancelled query cause retained", errors.Is(err, context.Canceled), true)
	assertMutationAudit(t, f.app, "user.subscription.update", 0)
}
