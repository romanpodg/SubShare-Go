package main

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

// Legacy replacement deliberately has different defaults, byte limits and
// server-local date parsing from PATCH. Do not apply PATCH clearing policy here.
type subscriptionReplacement struct {
	status, blockedReason, name, infoURL, extraURL, extraStatus string
	startsAt, expiresAt                                         sql.NullTime
	refreshHours                                                int
}

func prepareSubscriptionReplacement(input model.UpdateSubscriptionRequest) (subscriptionReplacement, error) {
	var next subscriptionReplacement
	if err := replacementAccess(input, &next); err != nil {
		return subscriptionReplacement{}, err
	}
	if err := replacementMetadata(input, &next); err != nil {
		return subscriptionReplacement{}, err
	}
	if err := replacementDelivery(input, &next); err != nil {
		return subscriptionReplacement{}, err
	}
	return next, nil
}

func replacementAccess(input model.UpdateSubscriptionRequest, next *subscriptionReplacement) error {
	status, ok := model.UserStatus(input.Status).Normalize()
	if !ok {
		return errors.New("invalid subscription status")
	}
	next.status = status
	starts, err := parseOptionalDateTimeLocal(input.StartsAt)
	if err != nil {
		return errors.New("invalid starts_at datetime")
	}
	expires, err := parseOptionalDateTimeLocal(input.ExpiresAt)
	if err != nil {
		return errors.New("invalid expires_at datetime")
	}
	next.startsAt, next.expiresAt = starts, expires
	if next.invalidDateRange() {
		return errors.New("starts_at must be before expires_at")
	}
	return nil
}

func (next subscriptionReplacement) invalidDateRange() bool {
	if !next.startsAt.Valid || !next.expiresAt.Valid {
		return false
	}
	return next.startsAt.Time.After(next.expiresAt.Time)
}

func replacementMetadata(input model.UpdateSubscriptionRequest, next *subscriptionReplacement) error {
	next.blockedReason = strings.TrimSpace(input.BlockedReason)
	if next.status != model.UserStatusBlocked {
		next.blockedReason = ""
	}
	next.name = strings.TrimSpace(input.SubscriptionName)
	if len(next.name) > 120 {
		return errors.New("subscription_name is too long (max 120 characters)")
	}
	return nil
}

func replacementDelivery(input model.UpdateSubscriptionRequest, next *subscriptionReplacement) error {
	next.refreshHours = input.SubscriptionRefreshHours
	if next.refreshHours <= 0 {
		next.refreshHours = 12
	}
	if next.refreshHours > 720 {
		return errors.New("subscription_refresh_hours must be between 1 and 720")
	}
	infoURL, err := normalizeAbsoluteHTTPURL(input.SubscriptionInfoURL, "subscription_info_url")
	if err != nil {
		return err
	}
	extraURL, err := normalizeAbsoluteHTTPURL(input.SubscriptionExtraURL, "subscription_extra_url")
	if err != nil {
		return err
	}
	next.infoURL, next.extraURL = infoURL, extraURL
	next.extraStatus = strings.TrimSpace(input.SubscriptionExtraStatus)
	if len(next.extraStatus) > 255 {
		return errors.New("subscription_extra_status is too long (max 255 characters)")
	}
	return nil
}
