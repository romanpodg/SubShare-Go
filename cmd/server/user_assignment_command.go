package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

var (
	errAssignmentUserNotFound = errors.New("assignment user not found")
	errAssignmentKeyNotFound  = errors.New("assignment key not found")
)

func (a *App) updateUserKeyAssignment(userID int64, mode string, keyIDs []int64) error {
	input, err := normalizeUserAssignment(mode, keyIDs)
	if err != nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := clearUserAssignments(tx, userID); err != nil {
		return err
	}
	if err := writeUserAssignments(tx, userID, input); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE users SET key_assignment_mode = ? WHERE id = ?`, input.mode, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func clearUserAssignments(tx *sql.Tx, userID int64) error {
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, userID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return errAssignmentUserNotFound
	}
	_, err := tx.Exec(`DELETE FROM user_keys WHERE user_id = ?`, userID)
	return err
}

func writeUserAssignments(tx *sql.Tx, userID int64, input userAssignmentInput) error {
	if input.mode == model.KeyAssignmentModeAll {
		return storage.AssignAllKeysToUser(context.Background(), tx, userID)
	}
	for _, id := range input.keyIDs {
		assigned, err := storage.AssignKeyToUser(context.Background(), tx, userID, id)
		if err != nil {
			return err
		}
		if !assigned {
			return fmt.Errorf("%w: %d", errAssignmentKeyNotFound, id)
		}
	}
	return nil
}
