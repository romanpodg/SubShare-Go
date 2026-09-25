package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

// --- Users API ---

func (a *App) apiListUsers(w http.ResponseWriter, r *http.Request) {
	applySensitiveResponseHeaders(w)
	users, err := a.listUsers()
	if err != nil {
		log.Printf("apiListUsers: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	// The legacy token column is retained for database compatibility, but it is
	// not used by current subscription delivery or the administration frontend.
	// Do not return that access material from bulk user projections.
	for i := range users {
		users[i].Token = ""
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"users": users})
}

// createUserInput is the validated, normalized form of a CreateUserRequest.
type createUserInput struct {
	name           string
	email          string
	activationCode string
	status         string
	blockedReason  string
	issueDays      int
}

// validateCreateUserRequest normalizes the request and returns a 400 message
// when it is invalid.
func validateCreateUserRequest(req model.CreateUserRequest) (createUserInput, string) {
	in := createUserInput{
		name:           strings.TrimSpace(req.Name),
		email:          strings.TrimSpace(req.Email),
		activationCode: strings.TrimSpace(req.ActivationCode),
		issueDays:      req.IssueDays,
	}
	status, ok := model.NormalizeUserStatus(req.Status)
	if !ok {
		return in, "invalid subscription status"
	}
	in.status = status

	if in.issueDays <= 0 {
		in.issueDays = 30
	}
	if in.issueDays > 3650 {
		return in, "issue days must be between 1 and 3650"
	}

	in.blockedReason = strings.TrimSpace(req.BlockedReason)
	if in.status != model.UserStatusBlocked {
		in.blockedReason = ""
	}

	if in.name == "" {
		return in, "name is required"
	}
	if in.activationCode == "" || strings.Contains(in.activationCode, "/") {
		return in, "activation code is required"
	}
	if len(in.name) > 255 {
		return in, "name is too long (max 255 characters)"
	}
	if len(in.email) > 255 {
		return in, "email is too long (max 255 characters)"
	}
	if len(in.activationCode) > 128 {
		return in, "activation code is too long (max 128 characters)"
	}
	return in, ""
}

func (a *App) apiCreateUser(w http.ResponseWriter, r *http.Request) {
	var req model.CreateUserRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	in, msg := validateCreateUserRequest(req)
	if msg != "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, msg)
		return
	}
	name, email, activationCode, status, blockedReason, issueDays := in.name, in.email, in.activationCode, in.status, in.blockedReason, in.issueDays

	legacyToken, err := generateToken(24)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to generate user token")
		return
	}

	subscriptionID, err := generateToken(24)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to generate subscription token")
		return
	}

	now := time.Now().UTC()
	expiresAt := now.AddDate(0, 0, issueDays)

	tx, err := a.db.Begin()
	if err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create user")
		return
	}
	defer tx.Rollback()

	var userID int64
	for attempt := 0; attempt < 5; attempt++ {
		var res sql.Result
		res, err = tx.Exec(
			`INSERT INTO users(name, email, token, activation_code, subscription_id, status, starts_at, expires_at, blocked_reason, max_devices) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			name, email, legacyToken, activationCode, subscriptionID, status, now, expiresAt, blockedReason,
		)
		if err == nil {
			userID, _ = res.LastInsertId()
			break
		}

		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "users.subscription_id") || strings.Contains(errText, "idx_users_subscription_id") {
			subscriptionID, err = generateToken(24)
			if err != nil {
				httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to generate subscription token")
				return
			}
			continue
		}
		break
	}
	if err != nil {
		log.Printf("apiCreateUser: %v", err)
		httpapi.WriteError(w, r, http.StatusConflict, "failed to create user (check activation code uniqueness)")
		return
	}

	if err := storage.AssignAllKeysToUser(context.Background(), tx, userID); err != nil {
		log.Printf("apiCreateUser: failed to assign keys to user %d: %v", userID, err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create user")
		return
	}
	if err := tx.Commit(); err != nil {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create user")
		return
	}

	a.recordAuditEvent(r, "user.create", "user", strconv.FormatInt(userID, 10), map[string]any{"name": name})
	httpapi.WriteMessage(w, "user created")
}

func (a *App) apiDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	res, err := a.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		log.Printf("apiDeleteUser: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to delete user")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
		return
	}
	a.recordAuditEvent(r, "user.delete", "user", strconv.FormatInt(id, 10), nil)
	httpapi.WriteMessage(w, "user deleted")
}

func (a *App) apiUpdateUserKeys(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateUserKeysRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := a.updateUserKeyAssignment(id, model.KeyAssignmentModeSelected, req.KeyIDs); err != nil {
		if errors.Is(err, errAssignmentUserNotFound) {
			httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, errAssignmentKeyNotFound) {
			httpapi.WriteError(w, r, http.StatusBadRequest, "one or more keys do not exist")
			return
		}
		log.Printf("apiUpdateUserKeys: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update user keys")
		return
	}
	a.recordAuditEvent(r, "user.keys.update", "user", strconv.FormatInt(id, 10), map[string]any{
		"mode": model.KeyAssignmentModeSelected, "keys_count": len(req.KeyIDs),
	})
	httpapi.WriteMessage(w, "subscription keys updated")
}

func (a *App) apiUpdateUserSubscription(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateSubscriptionRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	status, ok := model.NormalizeUserStatus(req.Status)
	if !ok {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid subscription status")
		return
	}

	startsAt, err := parseOptionalDateTimeLocal(req.StartsAt)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid starts_at datetime")
		return
	}
	expiresAt, err := parseOptionalDateTimeLocal(req.ExpiresAt)
	if err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid expires_at datetime")
		return
	}

	if startsAt.Valid && expiresAt.Valid && startsAt.Time.After(expiresAt.Time) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "starts_at must be before expires_at")
		return
	}

	blockedReason := strings.TrimSpace(req.BlockedReason)
	if status != model.UserStatusBlocked {
		blockedReason = ""
	}

	subscriptionName := strings.TrimSpace(req.SubscriptionName)
	if len(subscriptionName) > 120 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription_name is too long (max 120 characters)")
		return
	}

	subscriptionRefreshHours := req.SubscriptionRefreshHours
	if subscriptionRefreshHours <= 0 {
		subscriptionRefreshHours = 12
	}
	if subscriptionRefreshHours > 720 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription_refresh_hours must be between 1 and 720")
		return
	}

	normalizeURL := func(raw string, field string) (string, bool) {
		normalized, err := normalizeAbsoluteHTTPURL(raw, field)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, err.Error())
			return "", false
		}
		return normalized, true
	}

	subscriptionInfoURL, ok := normalizeURL(req.SubscriptionInfoURL, "subscription_info_url")
	if !ok {
		return
	}
	subscriptionExtraURL, ok := normalizeURL(req.SubscriptionExtraURL, "subscription_extra_url")
	if !ok {
		return
	}

	subscriptionExtraStatus := strings.TrimSpace(req.SubscriptionExtraStatus)
	if len(subscriptionExtraStatus) > 255 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription_extra_status is too long (max 255 characters)")
		return
	}

	_, err = a.db.Exec(
		`UPDATE users
		 SET status = ?,
		     starts_at = ?,
		     expires_at = ?,
		     blocked_reason = ?,
		     subscription_name = ?,
		     subscription_refresh_hours = ?,
		     subscription_info_url = ?,
		     subscription_extra_url = ?,
		     subscription_extra_status = ?
		 WHERE id = ?`,
		status,
		nullTimeValue(startsAt),
		nullTimeValue(expiresAt),
		nullStringValue(blockedReason),
		nullStringValue(subscriptionName),
		subscriptionRefreshHours,
		nullStringValue(subscriptionInfoURL),
		nullStringValue(subscriptionExtraURL),
		nullStringValue(subscriptionExtraStatus),
		id,
	)
	if err != nil {
		log.Printf("apiUpdateUserSubscription: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update subscription")
		return
	}
	a.recordAuditEvent(r, "user.subscription.update", "user", strconv.FormatInt(id, 10), map[string]any{"status": status})
	httpapi.WriteMessage(w, "subscription updated")
}

func (a *App) apiGetUserSubscriptionURLs(w http.ResponseWriter, r *http.Request) {
	applySensitiveResponseHeaders(w)
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var subscriptionID sql.NullString
	if err := a.db.QueryRow(`SELECT subscription_id FROM users WHERE id = ?`, id).Scan(&subscriptionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("apiGetUserSubscriptionURLs: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to load user subscription")
		return
	}

	subID := strings.TrimSpace(subscriptionID.String)
	if subID == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "subscription id is empty")
		return
	}

	plainURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subID)
	encryptedURL := ""
	if encrypted, err := a.encryptSubscriptionURL(plainURL); err == nil {
		encryptedURL = strings.TrimSpace(encrypted)
	} else {
		log.Printf("apiGetUserSubscriptionURLs: encrypt failed for user_id=%d: %v", id, err)
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"plain_url":     plainURL,
		"encrypted_url": encryptedURL,
	})
}

func (a *App) apiUpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateUserSettingsRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	timeZone := strings.TrimSpace(req.TimeZone)
	if timeZone == "" {
		timeZone = "Europe/Moscow"
	}
	if len(timeZone) > 64 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "time_zone is too long (max 64 characters)")
		return
	}
	if _, err := time.LoadLocation(timeZone); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "time_zone must be a valid IANA timezone")
		return
	}

	language := strings.ToLower(strings.TrimSpace(req.Language))
	if language == "" {
		language = "ru"
	}
	switch language {
	case "ru", "en":
	default:
		httpapi.WriteError(w, r, http.StatusBadRequest, "language must be one of: ru, en")
		return
	}

	res, err := a.db.Exec(`UPDATE users SET time_zone = ?, language = ? WHERE id = ?`, timeZone, language, id)
	if err != nil {
		log.Printf("apiUpdateUserSettings: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update user settings")
		return
	}
	updated, _ := res.RowsAffected()
	if updated == 0 {
		httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
		return
	}

	a.recordAuditEvent(r, "user.settings.update", "user", strconv.FormatInt(id, 10), map[string]any{
		"time_zone": timeZone,
		"language":  language,
	})
	httpapi.WriteMessage(w, "user settings updated")
}
