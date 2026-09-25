package sources

import (
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

const maxExternalSourceNameCodePoints = 64

// ValidateName normalizes and validates names supplied through
// either source API. Database writes must use the returned value.
func ValidateName(raw string) (string, error) {
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

// NormalizeRemoteName is intentionally separate from user-input
// validation: remote metadata can be shortened, while user input is rejected.
func NormalizeRemoteName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) <= maxExternalSourceNameCodePoints {
		return name, false
	}
	runes := []rune(name)
	return string(runes[:maxExternalSourceNameCodePoints-1]) + "…", true
}

func NormalizeRemoteProfileTitle(raw string) (string, []string) {
	name, shortened := NormalizeRemoteName(raw)
	if !shortened {
		return name, nil
	}
	return name, []string{"Remote profile title exceeded 64 characters and was shortened."}
}

func SuggestName(sourceURL string, meta Metadata) string {
	if strings.TrimSpace(meta.Title) != "" {
		return strings.TrimSpace(meta.Title)
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "Сторонняя подписка"
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "Сторонняя подписка"
	}
	return host
}

func NormalizeCategory(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "Общее"
	}
	if len(value) > 24 {
		value = value[:24]
	}
	return value
}

func NormalizeKeyInsertMode(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "top", "bottom":
		return value
	default:
		return "bottom"
	}
}
