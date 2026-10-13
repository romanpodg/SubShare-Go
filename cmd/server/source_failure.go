package main

import (
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"net/http"
)

type sourceFailure struct {
	status               int
	code, message, field string
}

func (e sourceFailure) Error() string { return e.message }

func writeSourceFailure(w http.ResponseWriter, r *http.Request, err error) {
	failure := errSourceCreate
	if !errors.As(err, &failure) {
		failure = errSourceCreate
	}
	httpapi.WriteFieldError(w, r, failure.status, failure.code, failure.message, failure.field)
}
