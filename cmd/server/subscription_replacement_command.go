package main

import "github.com/romanpodg/SubShare-Go/internal/model"

type subscriptionReplacementValidationError struct{ cause error }

func (err *subscriptionReplacementValidationError) Error() string { return err.cause.Error() }
func (err *subscriptionReplacementValidationError) Unwrap() error { return err.cause }

// Keep legacy one-statement replacement and its missing-target success contract.
// It uses the original background-context Exec; auditing belongs to HTTP.
func (a *App) replaceUserSubscription(id int64, input model.UpdateSubscriptionRequest) (subscriptionReplacement, error) {
	next, err := prepareSubscriptionReplacement(input)
	if err != nil {
		return subscriptionReplacement{}, &subscriptionReplacementValidationError{cause: err}
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
		next.status,
		nullTimeValue(next.startsAt),
		nullTimeValue(next.expiresAt),
		nullStringValue(next.blockedReason),
		nullStringValue(next.name),
		next.refreshHours,
		nullStringValue(next.infoURL),
		nullStringValue(next.extraURL),
		nullStringValue(next.extraStatus),
		id,
	)
	if err != nil {
		return subscriptionReplacement{}, err
	}
	return next, nil
}
