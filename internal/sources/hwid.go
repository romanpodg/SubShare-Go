package sources

import (
	"fmt"
	"net/http"
	"strings"
)

type HWIDProfile struct {
	PassHWID  bool
	Version   string
	ModelName string
	HWID      string
}

func clampExternalHWIDField(raw string, max int) string {
	value := strings.TrimSpace(raw)
	if max <= 0 || value == "" {
		return value
	}
	if len(value) > max {
		value = value[:max]
	}
	return value
}

func NormalizeHWIDProfile(pass bool, version, modelName, hwid string) HWIDProfile {
	if !pass {
		return HWIDProfile{}
	}
	return HWIDProfile{
		PassHWID:  true,
		Version:   clampExternalHWIDField(version, 64),
		ModelName: clampExternalHWIDField(modelName, 128),
		HWID:      clampExternalHWIDField(hwid, 128),
	}
}

func applyExternalHWIDHeaders(req *http.Request, profile HWIDProfile) {
	req.Header.Set("User-Agent", externalUserAgent(profile))
	if !profile.PassHWID {
		return
	}
	setNonEmptyHeaders(req.Header, map[string]string{
		"X-HWID":           profile.HWID,
		"X-Device-ID":      profile.HWID,
		"X-Device-Model":   profile.ModelName,
		"X-App-Version":    profile.Version,
		"X-Client-Version": profile.Version,
	})
	if profile.ModelName != "" || profile.Version != "" {
		req.Header.Set("X-Device-Info", strings.TrimSpace(fmt.Sprintf("model=%s;version=%s", profile.ModelName, profile.Version)))
	}
}

// externalUserAgent impersonates the Happ client only when a version is
// passed through; otherwise the request identifies itself as SubShare.
func externalUserAgent(profile HWIDProfile) string {
	switch {
	case !profile.PassHWID || profile.Version == "":
		return "subshare/1.0 (+external-import)"
	case profile.ModelName != "":
		return fmt.Sprintf("Happ/%s (%s)", profile.Version, profile.ModelName)
	default:
		return fmt.Sprintf("Happ/%s", profile.Version)
	}
}

func setNonEmptyHeaders(header http.Header, values map[string]string) {
	for name, value := range values {
		if value != "" {
			header.Set(name, value)
		}
	}
}
