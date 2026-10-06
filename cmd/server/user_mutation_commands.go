package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/romanpodg/SubShare-Go/internal/storage"
)

var (
	errCreateUserToken    = errors.New("generate user token")
	errCreateUserInsert   = errors.New("insert user")
	errDeleteUserNotFound = errors.New("user not found")
)

// createUser publishes an identity only after its user row and initial profile
// assignments have committed together. HTTP response/audit work stays outside.
func (a *App) createUser(input createUserInput) (int64, error) {
	record, err := prepareCreateUserRecord(input)
	if err != nil {
		return 0, err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin create user: %w", err)
	}
	defer tx.Rollback()
	id, err := insertUserWithSubscriptionRetries(tx, record)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errCreateUserInsert, err)
	}
	if err := storage.AssignAllKeysToUser(context.Background(), tx, id); err != nil {
		return 0, fmt.Errorf("assign profiles to user %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit create user: %w", err)
	}
	return id, nil
}

// One DELETE statement retains SQLite's existing atomic foreign-key cascades.
// Do not infer a missing user when the driver cannot report affected rows.
func (a *App) deleteUser(id int64) error {
	result, err := a.db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("confirm deleted user: %w", err)
	}
	if rows == 0 {
		return errDeleteUserNotFound
	}
	return nil
}
