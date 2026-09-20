package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/storage"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

var (
	errAssignmentUserNotFound = errors.New("assignment user not found")
	errAssignmentKeyNotFound  = errors.New("assignment key not found")
)

func normalizeAbsoluteHTTPURL(raw, field string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("%s must be an absolute http(s) URL", field)
	}
	return parsed.String(), nil
}

// The applyPatch* helpers implement PATCH semantics for one field each: an
// unset field leaves the target untouched, null clears it, a value replaces it.

func applyPatchStatus(field model.OptionalString, target *string) error {
	if !field.Set {
		return nil
	}
	if field.Null {
		return errors.New("status cannot be null")
	}
	normalized, valid := model.NormalizeUserStatus(field.Value)
	if !valid {
		return errors.New("invalid subscription status")
	}
	*target = normalized
	return nil
}

func applyPatchTime(field model.OptionalString, target *sql.NullTime, fieldName string, location *time.Location) error {
	if !field.Set {
		return nil
	}
	if field.Null || strings.TrimSpace(field.Value) == "" {
		*target = sql.NullTime{}
		return nil
	}
	parsed, parseErr := parseOptionalDateTimeInLocation(field.Value, location)
	if parseErr != nil {
		return fmt.Errorf("invalid %s datetime", fieldName)
	}
	*target = parsed
	return nil
}

func applyPatchString(field model.OptionalString, target *sql.NullString, maxRunes int, fieldName string) error {
	if !field.Set {
		return nil
	}
	if field.Null {
		*target = sql.NullString{}
		return nil
	}
	value := strings.TrimSpace(field.Value)
	if len([]rune(value)) > maxRunes {
		return fmt.Errorf("%s is too long", fieldName)
	}
	*target = sql.NullString{String: value, Valid: value != ""}
	return nil
}

func applyPatchURL(field model.OptionalString, target *sql.NullString, fieldName string) error {
	if !field.Set {
		return nil
	}
	if field.Null || strings.TrimSpace(field.Value) == "" {
		*target = sql.NullString{}
		return nil
	}
	value, normalizeErr := normalizeAbsoluteHTTPURL(field.Value, fieldName)
	if normalizeErr != nil {
		return normalizeErr
	}
	*target = sql.NullString{String: value, Valid: true}
	return nil
}

func applyPatchRefreshHours(field model.OptionalInt, target *int) error {
	if !field.Set {
		return nil
	}
	if field.Null || field.Value == 0 {
		*target = 0
		return nil
	}
	if field.Value < 1 || field.Value > 720 {
		return errors.New("subscription_refresh_hours must be between 1 and 720")
	}
	*target = field.Value
	return nil
}

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

	var status string
	var startsAt, expiresAt sql.NullTime
	var blockedReason, name, infoURL, extraURL, extraStatus sql.NullString
	var timeZone string
	var refreshHours int
	err := a.db.QueryRow(`
		SELECT status, starts_at, expires_at, blocked_reason, subscription_name,
		       subscription_refresh_hours, subscription_info_url,
		       subscription_extra_url, subscription_extra_status,
		       COALESCE(NULLIF(TRIM(time_zone), ''), 'UTC')
		FROM users WHERE id = ?
	`, id).Scan(
		&status, &startsAt, &expiresAt, &blockedReason, &name, &refreshHours,
		&infoURL, &extraURL, &extraStatus, &timeZone,
	)
	if errors.Is(err, sql.ErrNoRows) {
		httpapi.WriteV1Error(w, r, http.StatusNotFound, "user_not_found", "user not found")
		return
	}
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "subscription_load_failed", "failed to load subscription")
		return
	}

	if err := applyPatchStatus(input.Status, &status); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "status_invalid", err.Error())
		return
	}
	location, locationErr := time.LoadLocation(timeZone)
	if locationErr != nil {
		location = time.UTC
	}
	if err := applyPatchTime(input.StartsAt, &startsAt, "starts_at", location); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "starts_at_invalid", err.Error())
		return
	}
	if err := applyPatchTime(input.ExpiresAt, &expiresAt, "expires_at", location); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "expires_at_invalid", err.Error())
		return
	}
	if startsAt.Valid && expiresAt.Valid && startsAt.Time.After(expiresAt.Time) {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "date_range_invalid", "starts_at must be before expires_at")
		return
	}

	if err := applyPatchString(input.BlockedReason, &blockedReason, 255, "blocked_reason"); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "blocked_reason_invalid", err.Error())
		return
	}
	if err := applyPatchString(input.SubscriptionName, &name, 120, "subscription_name"); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "subscription_name_invalid", err.Error())
		return
	}
	if err := applyPatchString(input.SubscriptionExtraStatus, &extraStatus, 255, "subscription_extra_status"); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "subscription_extra_status_invalid", err.Error())
		return
	}
	if err := applyPatchURL(input.SubscriptionInfoURL, &infoURL, "subscription_info_url"); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "subscription_info_url_invalid", err.Error())
		return
	}
	if err := applyPatchURL(input.SubscriptionExtraURL, &extraURL, "subscription_extra_url"); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "subscription_extra_url_invalid", err.Error())
		return
	}
	if err := applyPatchRefreshHours(input.SubscriptionRefreshHours, &refreshHours); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "subscription_refresh_invalid", err.Error())
		return
	}
	if status != model.UserStatusBlocked {
		blockedReason = sql.NullString{}
	}

	_, err = a.db.Exec(`
		UPDATE users
		SET status = ?, starts_at = ?, expires_at = ?, blocked_reason = ?,
		    subscription_name = ?, subscription_refresh_hours = ?,
		    subscription_info_url = ?, subscription_extra_url = ?,
		    subscription_extra_status = ?
		WHERE id = ?
	`, status, nullTimeValue(startsAt), nullTimeValue(expiresAt), nullStringValue(blockedReason.String),
		nullStringValue(name.String), refreshHours, nullStringValue(infoURL.String),
		nullStringValue(extraURL.String), nullStringValue(extraStatus.String), id)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "subscription_update_failed", "failed to update subscription")
		return
	}
	a.recordAuditEvent(r, "user.subscription.update", "user", strconv.FormatInt(id, 10), map[string]any{"status": status})
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
