package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	errActivationNotFound    = errors.New("activation not found")
	errActivationAlreadyUsed = errors.New("activation already used")
)

// redeemActivationCode returns an identity only after a confirmed atomic claim.
// Redemption consumes a code; subscription status/date access policy is separate.
func (a *App) redeemActivationCode(code string) (string, error) {
	record, err := a.loadActivationRecord(code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errActivationNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load activation: %w", err)
	}
	if record.usedAt.Valid {
		return "", errActivationAlreadyUsed
	}
	subscriptionID, err := activationSubscriptionID(record.subscriptionID.String)
	if err != nil {
		return "", fmt.Errorf("prepare activation identity: %w", err)
	}
	claimed, err := a.claimActivation(record.userID, subscriptionID)
	if err != nil {
		return "", fmt.Errorf("claim activation: %w", err)
	}
	if !claimed {
		return "", errActivationAlreadyUsed
	}
	return subscriptionID, nil
}

func activationSubscriptionID(stored string) (string, error) {
	if id := strings.TrimSpace(stored); id != "" {
		return id, nil
	}
	return generateToken(24)
}
