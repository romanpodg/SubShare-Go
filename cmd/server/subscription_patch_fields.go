package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func normalizeAbsoluteHTTPURL(raw, field string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || !subscriptionHTTPURLAllowed(parsed) {
		return "", fmt.Errorf("%s must be an absolute http(s) URL", field)
	}
	return parsed.String(), nil
}

func subscriptionHTTPURLAllowed(parsed *url.URL) bool {
	if parsed.Host == "" {
		return false
	}
	if parsed.User != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// The applyPatch* helpers implement PATCH semantics for one field each: an
// unset field leaves the target untouched, null clears it, a value replaces it.

func applyPatchStatus(field model.OptionalString, target *string) error {
	if !field.Set {
		return nil
	}
	if field.Null {
		return errors.New("status cannot be null")
	}
	normalized, valid := model.NormalizeUserStatus(field.Value)
	if !valid {
		return errors.New("invalid subscription status")
	}
	*target = normalized
	return nil
}

func applyPatchTime(field model.OptionalString, target *sql.NullTime, fieldName string, location *time.Location) error {
	if !field.Set {
		return nil
	}
	if field.Null || strings.TrimSpace(field.Value) == "" {
		*target = sql.NullTime{}
		return nil
	}
	parsed, parseErr := parseOptionalDateTimeInLocation(field.Value, location)
	if parseErr != nil {
		return fmt.Errorf("invalid %s datetime", fieldName)
	}
	*target = parsed
	return nil
}

func applyPatchString(field model.OptionalString, target *sql.NullString, maxRunes int, fieldName string) error {
	if !field.Set {
		return nil
	}
	if field.Null {
		*target = sql.NullString{}
		return nil
	}
	value := strings.TrimSpace(field.Value)
	if len([]rune(value)) > maxRunes {
		return fmt.Errorf("%s is too long", fieldName)
	}
	*target = sql.NullString{String: value, Valid: value != ""}
	return nil
}

func applyPatchURL(field model.OptionalString, target *sql.NullString, fieldName string) error {
	if !field.Set {
		return nil
	}
	if field.Null || strings.TrimSpace(field.Value) == "" {
		*target = sql.NullString{}
		return nil
	}
	value, normalizeErr := normalizeAbsoluteHTTPURL(field.Value, fieldName)
	if normalizeErr != nil {
		return normalizeErr
	}
	*target = sql.NullString{String: value, Valid: true}
	return nil
}

func applyPatchRefreshHours(field model.OptionalInt, target *int) error {
	if !field.Set {
		return nil
	}
	if field.Null || field.Value == 0 {
		*target = 0
		return nil
	}
	if field.Value < 1 || field.Value > 720 {
		return errors.New("subscription_refresh_hours must be between 1 and 720")
	}
	*target = field.Value
	return nil
}
