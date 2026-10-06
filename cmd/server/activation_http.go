package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
)

func writeActivationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errActivationCodeInvalid):
		httpapi.WriteError(w, r, http.StatusBadRequest, "Введите корректный ключ активации")
	case errors.Is(err, errActivationNotFound):
		httpapi.WriteError(w, r, http.StatusNotFound, "Подписка не найдена")
	case errors.Is(err, errActivationAlreadyUsed):
		httpapi.WriteError(w, r, http.StatusForbidden, "Ключ уже активирован")
	default:
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to activate subscription")
	}
}

func (a *App) activationSubscriptionURL(r *http.Request, subscriptionID string) string {
	subscriptionURL := fmt.Sprintf("%s/sub/%s", a.resolveBaseURL(r), subscriptionID)
	if strings.TrimSpace(a.happCryptoAPIURL) == "" {
		return subscriptionURL
	}
	encryptedURL, err := a.encryptSubscriptionURL(subscriptionURL)
	if err != nil {
		log.Printf("apiActivateSubscription: failed to encrypt url via configured Happ API: %v", err)
		return subscriptionURL
	}
	if strings.TrimSpace(encryptedURL) != "" {
		return encryptedURL
	}
	return subscriptionURL
}
