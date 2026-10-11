package main

import (
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"net/http"
)

func writeSourceCreateCommandError(w http.ResponseWriter, r *http.Request, err error) {
	failure := errSourceCreate
	if !errors.As(err, &failure) {
		failure = errSourceCreate
	}
	if failure.field != "" {
		httpapi.WriteFieldError(w, r, http.StatusConflict, failure.code, failure.message, failure.field)
		return
	}
	httpapi.WriteV1Error(w, r, http.StatusInternalServerError, failure.code, failure.message)
}
