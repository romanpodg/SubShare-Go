package keymanagement

import (
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

// validateStoredConfiguration keeps legacy XRAY-JSON support inside the
// profile-aware API without teaching the URI registry to own JSON documents.
func validateStoredConfiguration(raw string) (string, *profiles.Profile, error) {
	trimmed := strings.TrimSpace(raw)
	if profileconfig.SupportedConfigScheme(trimmed) == "xray-json" {
		if err := profileconfig.ValidateXrayJSONConfiguration(trimmed); err != nil {
			return "", nil, err
		}
		return "xray-json", nil, nil
	}

	parsed, err := profiles.Parse(trimmed)
	if err != nil {
		return "", nil, err
	}
	return string(parsed.Protocol), parsed, nil
}
