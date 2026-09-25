package delivery

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type TemplateData struct {
	UserName       string
	Telegram       string
	SubscriptionID string
	ExpiryDate     string
	ExpiryDateTime string
	RealKeysCount  int
}

func RenderInfoTemplate(template string, data TemplateData) string {
	text := strings.TrimSpace(template)
	if text == "" {
		return ""
	}
	replacements := map[string]string{
		"{user_name}":       data.UserName,
		"{telegram}":        data.Telegram,
		"{subscription_id}": data.SubscriptionID,
		"{expires_date}":    data.ExpiryDate,
		"{expires_at}":      data.ExpiryDateTime,
		"{real_keys_count}": fmt.Sprintf("%d", data.RealKeysCount),
	}
	for key, value := range replacements {
		text = strings.ReplaceAll(text, key, strings.TrimSpace(value))
	}
	return strings.TrimSpace(text)
}

// informationalText renders an informational entry's template, falling back
// to its label; empty means the entry renders to nothing.
func informationalText(entry Entry, templateData TemplateData) string {
	textTemplate := strings.TrimSpace(entry.TemplateText)
	if textTemplate == "" {
		textTemplate = strings.TrimSpace(entry.Label)
	}
	return RenderInfoTemplate(textTemplate, templateData)
}

// MaterializeInformational turns informational entries into synthetic real
// entries rendered from tpl, dropping those that render to nothing.
func MaterializeInformational(entries []Entry, templateData TemplateData, xray bool) []Entry {
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Kind != model.KeyKindInformational {
			result = append(result, entry)
			continue
		}
		if synthetic, ok := materializeEntry(entry, templateData, xray); ok {
			result = append(result, synthetic)
		}
	}
	return result
}

func materializeEntry(entry Entry, templateData TemplateData, xray bool) (Entry, bool) {
	rendered := informationalText(entry, templateData)
	if rendered == "" {
		return Entry{}, false
	}
	entry.Kind = model.KeyKindReal
	if xray {
		entry.Raw, entry.StoredProtocol = InformationalXrayJSON(rendered), "xray-json"
	} else {
		entry.Raw, entry.StoredProtocol = InformationalVLESSURL(rendered), "vless"
	}
	return entry, true
}

func plainInformationalLines(entry Entry, format string, templateData TemplateData) []string {
	rendered := informationalText(entry, templateData)
	if rendered == "" {
		return nil
	}
	if format != model.SubscriptionFormatXrayJSON {
		return []string{InformationalVLESSURL(rendered)}
	}
	info := InformationalXrayJSON(rendered)
	if strings.TrimSpace(info) == "" {
		return nil
	}
	return []string{info}
}

func InformationalVLESSURL(displayText string) string {
	displayText = strings.TrimSpace(displayText)
	if displayText == "" {
		displayText = "Info"
	}
	return "vless://00000000-0000-0000-0000-000000000000@info.invalid:443?type=tcp&security=none#" + url.QueryEscape(displayText)
}

func InformationalXrayJSON(displayText string) string {
	displayText = strings.TrimSpace(displayText)
	if displayText == "" {
		displayText = "Info"
	}

	description := displayText
	if newline := strings.Index(description, "\n"); newline >= 0 {
		description = strings.TrimSpace(description[:newline])
	}
	if description == "" {
		description = "Informational key"
	}

	payload := map[string]any{
		"remarks": displayText,
		"meta": map[string]any{
			"serverDescription": description,
			"informational":     true,
		},
		"log": map[string]any{
			"loglevel": "warning",
		},
		"inbounds": []any{},
		"outbounds": []any{
			map[string]any{
				"tag":      "proxy",
				"protocol": "vless",
				"settings": map[string]any{
					"vnext": []any{
						map[string]any{
							"address": "info.invalid",
							"port":    443,
							"users": []any{
								map[string]any{
									"id":         "00000000-0000-0000-0000-000000000000",
									"encryption": "none",
								},
							},
						},
					},
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "none",
				},
			},
		},
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}
