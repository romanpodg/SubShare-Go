package main

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) subscriptionAccessAllowed(subscriptionID string) (bool, int64, int, string, error) {
	var row subscriptionAccessRow
	err := a.db.QueryRow(
		`SELECT id, status, starts_at, expires_at, blocked_reason FROM users WHERE subscription_id = ?`,
		subscriptionID,
	).Scan(&row.userID, &row.status, &row.startsAt, &row.expiresAt, &row.blockedReason)
	if errors.Is(err, sql.ErrNoRows) {
		return false, 0, http.StatusNotFound, "subscription not found", nil
	}
	if err != nil {
		return false, 0, 0, "", err
	}
	code, reason := row.accessDecision(time.Now().UTC())
	return code == http.StatusOK, row.userID, code, reason, nil
}

type subscriptionAccessRow struct {
	userID                int64
	status, blockedReason sql.NullString
	startsAt, expiresAt   sql.NullTime
}

// accessDecision preserves denial precedence: status, start time, expiration.
func (row subscriptionAccessRow) accessDecision(now time.Time) (int, string) {
	switch model.NormalizeStoredStatus(row.status.String) {
	case model.UserStatusBlocked:
		return http.StatusForbidden, firstNonEmpty(row.blockedReason.String, "subscription blocked")
	case model.UserStatusPaused:
		return http.StatusForbidden, "subscription paused"
	}
	if row.startsAt.Valid && now.Before(row.startsAt.Time.UTC()) {
		return http.StatusForbidden, "subscription is not active yet"
	}
	if row.expiresAt.Valid && now.After(row.expiresAt.Time.UTC()) {
		return http.StatusGone, "subscription expired"
	}
	return http.StatusOK, ""
}
