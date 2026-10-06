package main

import (
	"errors"
	"strings"
)

var errActivationCodeInvalid = errors.New("invalid activation code")

func validateActivationCode(raw string) (string, error) {
	code := strings.TrimSpace(raw)
	if code == "" || strings.Contains(code, "/") {
		return "", errActivationCodeInvalid
	}
	return code, nil
}
