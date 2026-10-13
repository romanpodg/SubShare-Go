package main

import (
	"testing"
	"time"
)

func seedSubscriptionUser(t *testing.T, app *App, status string) int64 {
	t.Helper()
	result, err := app.db.Exec(`
		INSERT INTO users(
			name, email, token, activation_code, subscription_id, status,
			starts_at, expires_at, max_devices, subscription_name,
			subscription_refresh_hours, subscription_info_url
		) VALUES('Alice', 'alice@example.test', 'legacy-token', 'activation-token',
		         'subscription-token', ?, ?, ?, 1, 'Personal title', 24,
		         'https://user.example/info')
	`, status, time.Now().Add(-time.Hour).UTC(), time.Now().Add(time.Hour).UTC())
	requireFixturef(t, err == nil, "seed user: %v", err)

	id, err := result.LastInsertId()
	requireFixturef(t, err == nil, "fixture LastInsertId: %v", err)

	return id
}
