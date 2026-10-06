package main

import (
	"database/sql"
	"errors"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type subscriptionPatchState struct {
	status, timeZone                                    string
	startsAt, expiresAt                                 sql.NullTime
	blockedReason, name, infoURL, extraURL, extraStatus sql.NullString
	refreshHours                                        int
}

func applySubscriptionPatch(input model.PatchSubscriptionRequest, current subscriptionPatchState) (subscriptionPatchState, error) {
	next := current
	if err := patchSubscriptionAccess(input, &next); err != nil {
		return subscriptionPatchState{}, err
	}
	if err := patchSubscriptionMetadata(input, &next); err != nil {
		return subscriptionPatchState{}, err
	}
	if err := patchSubscriptionDelivery(input, &next); err != nil {
		return subscriptionPatchState{}, err
	}
	if next.status != model.UserStatusBlocked {
		next.blockedReason = sql.NullString{}
	}
	return next, nil
}

func patchSubscriptionAccess(input model.PatchSubscriptionRequest, next *subscriptionPatchState) error {
	if err := applyPatchStatus(input.Status, &next.status); err != nil {
		return invalidSubscriptionPatch("status_invalid", err)
	}
	location, err := time.LoadLocation(next.timeZone)
	if err != nil {
		location = time.UTC
	}
	if err := applyPatchTime(input.StartsAt, &next.startsAt, "starts_at", location); err != nil {
		return invalidSubscriptionPatch("starts_at_invalid", err)
	}
	if err := applyPatchTime(input.ExpiresAt, &next.expiresAt, "expires_at", location); err != nil {
		return invalidSubscriptionPatch("expires_at_invalid", err)
	}
	if next.invalidDateRange() {
		return invalidSubscriptionPatch("date_range_invalid", errors.New("starts_at must be before expires_at"))
	}
	return nil
}

func (state subscriptionPatchState) invalidDateRange() bool {
	if !state.startsAt.Valid || !state.expiresAt.Valid {
		return false
	}
	return state.startsAt.Time.After(state.expiresAt.Time)
}

func patchSubscriptionMetadata(input model.PatchSubscriptionRequest, next *subscriptionPatchState) error {
	if err := applyPatchString(input.BlockedReason, &next.blockedReason, 255, "blocked_reason"); err != nil {
		return invalidSubscriptionPatch("blocked_reason_invalid", err)
	}
	if err := applyPatchString(input.SubscriptionName, &next.name, 120, "subscription_name"); err != nil {
		return invalidSubscriptionPatch("subscription_name_invalid", err)
	}
	if err := applyPatchString(input.SubscriptionExtraStatus, &next.extraStatus, 255, "subscription_extra_status"); err != nil {
		return invalidSubscriptionPatch("subscription_extra_status_invalid", err)
	}
	return nil
}

func patchSubscriptionDelivery(input model.PatchSubscriptionRequest, next *subscriptionPatchState) error {
	if err := applyPatchURL(input.SubscriptionInfoURL, &next.infoURL, "subscription_info_url"); err != nil {
		return invalidSubscriptionPatch("subscription_info_url_invalid", err)
	}
	if err := applyPatchURL(input.SubscriptionExtraURL, &next.extraURL, "subscription_extra_url"); err != nil {
		return invalidSubscriptionPatch("subscription_extra_url_invalid", err)
	}
	if err := applyPatchRefreshHours(input.SubscriptionRefreshHours, &next.refreshHours); err != nil {
		return invalidSubscriptionPatch("subscription_refresh_invalid", err)
	}
	return nil
}
