package main

import "database/sql"

type activationRecord struct {
	userID         int64
	usedAt         sql.NullTime
	subscriptionID sql.NullString
}

func (a *App) loadActivationRecord(code string) (activationRecord, error) {
	var record activationRecord
	err := a.db.QueryRow(
		`SELECT id, activation_used_at, subscription_id FROM users WHERE activation_code = ?`,
		code,
	).Scan(&record.userID, &record.usedAt, &record.subscriptionID)
	return record, err
}

// claimActivation atomically stores the subscription identity and marks the code
// used only while it remains unclaimed. The preceding read is not a reservation.
// A result-reporting error leaves the outcome unknown; it cannot imply rollback.
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
