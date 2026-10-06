package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
)

func writeCreateUserCommandError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errCreateUserToken) {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to generate user token")
		return
	}
	// Retry-generation failures also carry the insert marker: retain their
	// original token-generation response before classifying other inserts.
	if errors.Is(err, errCreateSubscriptionToken) {
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to generate subscription token")
		return
	}
	log.Printf("apiCreateUser: %v", err)
	if errors.Is(err, errCreateUserInsert) {
		httpapi.WriteError(w, r, http.StatusConflict, "failed to create user (check activation code uniqueness)")
		return
	}
	httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to create user")
}

func writeDeleteUserCommandError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errDeleteUserNotFound) {
		httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
		return
	}
	log.Printf("apiDeleteUser: %v", err)
	httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to delete user")
}
