package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
)

var errCreateSubscriptionToken = errors.New("generate subscription token")

type createUserRecord struct {
	input                       createUserInput
	legacyToken, subscriptionID string
	startsAt, expiresAt         time.Time
}

// The caller owns the transaction, including key assignment and commit. Keep
// the last INSERT failure intact when retrying; a newly generated token is not
// evidence that a user was created.
func insertUserWithSubscriptionRetries(tx *sql.Tx, record createUserRecord) (int64, error) {
	var insertErr error
	for attempt := 0; attempt < 5; attempt++ {
		var result sql.Result
		result, insertErr = tx.Exec(
			`INSERT INTO users(name, email, token, activation_code, subscription_id, status, starts_at, expires_at, blocked_reason, max_devices) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			record.input.name, record.input.email, record.legacyToken, record.input.activationCode,
			record.subscriptionID, record.input.status, record.startsAt, record.expiresAt, record.input.blockedReason,
		)
		if insertErr == nil {
			return result.LastInsertId()
		}
		if !isCreateSubscriptionCollision(insertErr) {
			break
		}
		if attempt == 4 {
			break
		}
		nextID, err := generateToken(24)
		if err != nil {
			return 0, fmt.Errorf("%w: %w", errCreateSubscriptionToken, err)
		}
		record.subscriptionID = nextID
	}
	return 0, insertErr
}

func isCreateSubscriptionCollision(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "users.subscription_id") || strings.Contains(message, "idx_users_subscription_id")
}

func writeCreateUserInsertError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errCreateSubscriptionToken) {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to generate subscription token")
		return
	}
	log.Printf("apiCreateUser: %v", err)
	httpapi.WriteError(w, r, http.StatusConflict, "failed to create user (check activation code uniqueness)")
}
