package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
)

func writeSubscriptionReplacementError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *subscriptionReplacementValidationError
	if errors.As(err, &invalid) {
		httpapi.WriteError(w, r, http.StatusBadRequest, invalid.Error())
		return
	}
	log.Printf("apiUpdateUserSubscription: %v", err)
	httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update subscription")
}
