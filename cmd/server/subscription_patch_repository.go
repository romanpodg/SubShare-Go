package main

import (
	"context"
	"database/sql"
)

type subscriptionPatchSnapshot struct {
	state subscriptionPatchState
	// Compare the original stored text, including NULL and date precision,
	// rather than a timestamp reformatted after database/sql parsed it.
	rawStartsAt, rawExpiresAt, rawTimeZone sql.NullString
}

func loadSubscriptionPatchSnapshot(ctx context.Context, db *sql.DB, id int64) (subscriptionPatchSnapshot, error) {
	var snapshot subscriptionPatchSnapshot
	state := &snapshot.state
	err := db.QueryRowContext(ctx, `
		SELECT status, starts_at, expires_at, blocked_reason, subscription_name,
		       subscription_refresh_hours, subscription_info_url,
		       subscription_extra_url, subscription_extra_status,
		       COALESCE(NULLIF(TRIM(time_zone), ''), 'UTC'),
		       CAST(starts_at AS TEXT), CAST(expires_at AS TEXT), time_zone
		FROM users WHERE id = ?
	`, id).Scan(
		&state.status, &state.startsAt, &state.expiresAt, &state.blockedReason, &state.name,
		&state.refreshHours, &state.infoURL, &state.extraURL, &state.extraStatus, &state.timeZone,
		&snapshot.rawStartsAt, &snapshot.rawExpiresAt, &snapshot.rawTimeZone,
	)
	return snapshot, err
}

// The comparison and write are one SQLite statement, protecting against other
// PATCH/PUT writers and settings changes across connections and server processes.
func (snapshot subscriptionPatchSnapshot) store(ctx context.Context, db *sql.DB, id int64, next subscriptionPatchState) (bool, error) {
	previous := snapshot.state
	result, err := db.ExecContext(ctx, `
		UPDATE users
		SET status = ?, starts_at = ?, expires_at = ?, blocked_reason = ?,
		    subscription_name = ?, subscription_refresh_hours = ?,
		    subscription_info_url = ?, subscription_extra_url = ?,
		    subscription_extra_status = ?
		WHERE id = ? AND status IS ?
		  AND CAST(starts_at AS TEXT) IS ? AND CAST(expires_at AS TEXT) IS ?
		  AND blocked_reason IS ? AND subscription_name IS ?
		  AND subscription_refresh_hours IS ? AND subscription_info_url IS ?
		  AND subscription_extra_url IS ? AND subscription_extra_status IS ?
		  AND time_zone IS ?
	`, next.status, nullTimeValue(next.startsAt), nullTimeValue(next.expiresAt), nullStringValue(next.blockedReason.String),
		nullStringValue(next.name.String), next.refreshHours, nullStringValue(next.infoURL.String),
		nullStringValue(next.extraURL.String), nullStringValue(next.extraStatus.String), id,
		previous.status, snapshot.rawStartsAt, snapshot.rawExpiresAt, previous.blockedReason, previous.name,
		previous.refreshHours, previous.infoURL, previous.extraURL, previous.extraStatus, snapshot.rawTimeZone)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows != 0, err
}
