package main

import (
	"errors"
	"net/http"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
)

func writeSubscriptionPatchError(w http.ResponseWriter, r *http.Request, err error) {
	var failure *subscriptionPatchError
	if errors.As(err, &failure) {
		httpapi.WriteV1Error(w, r, failure.status, failure.code, failure.message)
		return
	}
	httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "subscription_update_failed", "failed to update subscription")
}
