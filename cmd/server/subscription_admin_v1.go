package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/storage"
	"net/http"
	"strconv"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

var (
	errAssignmentUserNotFound = errors.New("assignment user not found")
	errAssignmentKeyNotFound  = errors.New("assignment key not found")
)

func (a *App) apiV1PatchUserSubscription(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	var input model.PatchSubscriptionRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	updated, err := a.patchUserSubscription(r.Context(), id, input)
	if err != nil {
		writeSubscriptionPatchError(w, r, err)
		return
	}
	a.recordAuditEvent(r, "user.subscription.update", "user", strconv.FormatInt(id, 10), map[string]any{"status": updated.status})
	httpapi.WriteMessage(w, "subscription updated")
}

func (a *App) updateUserKeyAssignment(userID int64, mode string, keyIDs []int64) error {
	mode, ok := model.NormalizeKeyAssignmentMode(mode)
	if !ok {
		return fmt.Errorf("invalid assignment mode")
	}
	normalizedIDs := make([]int64, 0, len(keyIDs))
	seen := make(map[int64]struct{}, len(keyIDs))
	for _, keyID := range keyIDs {
		if keyID <= 0 {
			return fmt.Errorf("invalid key id")
		}
		if _, exists := seen[keyID]; exists {
			continue
		}
		seen[keyID] = struct{}{}
		normalizedIDs = append(normalizedIDs, keyID)
	}

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, userID).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return errAssignmentUserNotFound
	}
	if _, err := tx.Exec(`DELETE FROM user_keys WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if mode == model.KeyAssignmentModeAll {
		if err := storage.AssignAllKeysToUser(context.Background(), tx, userID); err != nil {
			return err
		}
	} else {
		for _, keyID := range normalizedIDs {
			assigned, err := storage.AssignKeyToUser(context.Background(), tx, userID, keyID)
			if err != nil {
				return err
			}
			if !assigned {
				return fmt.Errorf("%w: %d", errAssignmentKeyNotFound, keyID)
			}
		}
	}
	if _, err := tx.Exec(`UPDATE users SET key_assignment_mode = ? WHERE id = ?`, mode, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) apiV1UpdateUserKeyAssignment(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	var input model.UpdateKeyAssignmentRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	mode, valid := model.NormalizeKeyAssignmentMode(input.Mode)
	if !valid {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "assignment_mode_invalid", "mode must be all or selected")
		return
	}
	if mode == model.KeyAssignmentModeAll && len(input.KeyIDs) > 0 {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "assignment_keys_invalid", "key_ids must be empty in all mode")
		return
	}
	err := a.updateUserKeyAssignment(id, mode, input.KeyIDs)
	switch {
	case errors.Is(err, errAssignmentUserNotFound):
		httpapi.WriteV1Error(w, r, http.StatusNotFound, "user_not_found", "user not found")
	case errors.Is(err, errAssignmentKeyNotFound):
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "key_not_found", "one or more keys do not exist")
	case err != nil:
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "assignment_update_failed", "failed to update key assignment")
	default:
		a.recordAuditEvent(r, "user.keys.update", "user", strconv.FormatInt(id, 10), map[string]any{
			"mode": mode, "keys_count": len(input.KeyIDs),
		})
		httpapi.WriteMessage(w, "subscription key assignment updated")
	}
}
