// Package delivery renders a subscription body from stored profile entries.
// It has no database, keyring or HTTP dependencies: callers select the entries
// and pass them in; the module owns every output format and its exclusion
// policy.
package delivery

import (
	"encoding/base64"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func EffectiveFormat(responseType string, settings model.SubscriptionSettings, defaultEncoding string) string {
	if responseType != "" {
		return responseType
	}
	if settings.SubscriptionFormat == model.SubscriptionFormatXrayJSON {
		return "xray-json"
	}
	if defaultEncoding == "base64" {
		return "base64"
	}
	return "plain"
}

func IsStructuredFormat(format string) bool {
	switch format {
	case "mihomo", "sing-box", "xray-json":
		return true
	default:
		return false
	}
}

// Render produces the subscription body for responseType. Informational
// entries are expanded with tpl; subscriptionFormat only matters for the
// plain/base64 path.
func Render(responseType string, entries []Entry, subscriptionFormat string, tpl TemplateData) (Generated, error) {
	switch responseType {
	case "mihomo":
		return RenderMihomo(MaterializeInformational(entries, tpl, false))
	case "sing-box":
		return RenderSingBox(MaterializeInformational(entries, tpl, false))
	case "xray-json":
		return RenderXray(MaterializeInformational(entries, tpl, true))
	default:
		return RenderPlain(entries, subscriptionFormat, tpl)
	}
}

func EncodeBase64(body string) string {
	return base64.StdEncoding.EncodeToString([]byte(body))
}
