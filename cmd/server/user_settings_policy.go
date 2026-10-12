package main

import (
	"errors"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type userSettingsInput struct{ timeZone, language string }

func prepareUserSettings(input model.UpdateUserSettingsRequest) (userSettingsInput, error) {
	zone, err := normalizeUserTimeZone(input.TimeZone)
	if err != nil {
		return userSettingsInput{}, err
	}
	language := strings.ToLower(strings.TrimSpace(input.Language))
	if language == "" {
		language = "ru"
	}
	switch language {
	case "ru", "en":
	default:
		return userSettingsInput{}, errors.New("language must be one of: ru, en")
	}
	return userSettingsInput{timeZone: zone, language: language}, nil
}

func normalizeUserTimeZone(raw string) (string, error) {
	zone := strings.TrimSpace(raw)
	if zone == "" {
		zone = "Europe/Moscow"
	}
	if len(zone) > 64 {
		return "", errors.New("time_zone is too long (max 64 characters)")
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return "", errors.New("time_zone must be a valid IANA timezone")
	}
	return zone, nil
}
