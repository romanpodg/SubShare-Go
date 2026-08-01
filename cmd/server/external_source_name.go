package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxExternalSourceNameCodePoints = 64

// validateExternalSourceName normalizes and validates names supplied through
// either source API. Database writes must use the returned value.
func validateExternalSourceName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("name must be valid UTF-8")
	}
	if utf8.RuneCountInString(name) > maxExternalSourceNameCodePoints {
		return "", fmt.Errorf("name is too long (max %d characters)", maxExternalSourceNameCodePoints)
	}
	return name, nil
}

// normalizeRemoteExternalSourceName is intentionally separate from user-input
// validation: remote metadata can be shortened, while user input is rejected.
func normalizeRemoteExternalSourceName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) <= maxExternalSourceNameCodePoints {
		return name, false
	}
	runes := []rune(name)
	return string(runes[:maxExternalSourceNameCodePoints-1]) + "…", true
}

func normalizeRemoteProfileTitle(raw string) (string, []string) {
	name, shortened := normalizeRemoteExternalSourceName(raw)
	if !shortened {
		return name, nil
	}
	return name, []string{"Remote profile title exceeded 64 characters and was shortened."}
}
