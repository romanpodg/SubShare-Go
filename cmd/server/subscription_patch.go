package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

const subscriptionPatchAttempts = 5

type subscriptionPatchError struct {
	status        int
	code, message string
	cause         error
}

func (e *subscriptionPatchError) Error() string { return e.message }
func (e *subscriptionPatchError) Unwrap() error { return e.cause }

// Each conditional write checks the values used for validation. A stale patch
// is reapplied to a fresh row so unrelated changes and dependent policies survive.
func (a *App) patchUserSubscription(ctx context.Context, id int64, input model.PatchSubscriptionRequest) (subscriptionPatchState, error) {
	for attempt := 0; attempt < subscriptionPatchAttempts; attempt++ {
		snapshot, err := loadSubscriptionPatchSnapshot(ctx, a.db, id)
		if err != nil {
			return subscriptionPatchState{}, subscriptionPatchLoadError(err)
		}
		next, err := applySubscriptionPatch(input, snapshot.state)
		if err != nil {
			return subscriptionPatchState{}, err
		}
		written, err := snapshot.store(ctx, a.db, id, next)
		if err != nil {
			return subscriptionPatchState{}, &subscriptionPatchError{
				status: http.StatusInternalServerError, code: "subscription_update_failed",
				message: "failed to update subscription", cause: err,
			}
		}
		if written {
			return next, nil
		}
	}
	return subscriptionPatchState{}, &subscriptionPatchError{
		status: http.StatusConflict, code: "subscription_conflict",
		message: "subscription changed; retry the request",
	}
}

func subscriptionPatchLoadError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return &subscriptionPatchError{status: http.StatusNotFound, code: "user_not_found", message: "user not found", cause: err}
	}
	return &subscriptionPatchError{
		status: http.StatusInternalServerError, code: "subscription_load_failed",
		message: "failed to load subscription", cause: err,
	}
}

func invalidSubscriptionPatch(code string, err error) error {
	return &subscriptionPatchError{status: http.StatusBadRequest, code: code, message: err.Error(), cause: err}
}

func writeSubscriptionPatchError(w http.ResponseWriter, r *http.Request, err error) {
	var failure *subscriptionPatchError
	if errors.As(err, &failure) {
		httpapi.WriteV1Error(w, r, failure.status, failure.code, failure.message)
		return
	}
	httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "subscription_update_failed", "failed to update subscription")
}
