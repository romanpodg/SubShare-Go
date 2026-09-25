package main

import (
	"context"
	"net/http"
	"regexp"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
)

var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{8}$`)

func applySensitiveResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

func normalizeKeyCategory(raw string) string {
	return keymanagement.NormalizeKeyCategory(raw)
}

func (a *App) upsertKeyCategory(category string) error {
	_, err := a.keyService().EnsureCategory(context.Background(), category)
	return err
}
