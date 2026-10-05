package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

func (a *App) redeemActivationCode(code string) (string, int, string, error) {
	var userID int64
	var usedAt sql.NullTime
	var subscriptionID sql.NullString
	err := a.db.QueryRow(
		`SELECT id, activation_used_at, subscription_id FROM users WHERE activation_code = ?`,
		code,
	).Scan(&userID, &usedAt, &subscriptionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", http.StatusNotFound, "Подписка не найдена", nil
	}
	if err != nil {
		return "", 0, "", err
	}

	if usedAt.Valid {
		return "", http.StatusForbidden, "Ключ уже активирован", nil
	}

	generatedSubscriptionID, err := activationSubscriptionID(subscriptionID.String)
	if err != nil {
		return "", 0, "", err
	}
	claimed, err := a.claimActivation(userID, generatedSubscriptionID)
	if err != nil {
		return "", 0, "", err
	}
	if !claimed {
		return "", http.StatusForbidden, "Ключ уже активирован", nil
	}
	return generatedSubscriptionID, http.StatusOK, "", nil
}

func activationSubscriptionID(stored string) (string, error) {
	if id := strings.TrimSpace(stored); id != "" {
		return id, nil
	}
	return generateToken(24)
}

// claimActivation succeeds only for the first redemption, even if another
// request activated the user after the initial SELECT.
func (a *App) claimActivation(userID int64, subscriptionID string) (bool, error) {
	res, err := a.db.Exec(
		`UPDATE users
		 SET subscription_id = ?, activation_used_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND activation_used_at IS NULL`,
		subscriptionID,
		userID,
	)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows != 0, nil
}
