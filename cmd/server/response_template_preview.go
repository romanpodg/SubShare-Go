package main

import (
	"encoding/base64"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
)

func renderTemplatePreview(input templateInput) (string, string, error) {
	const sampleVLESS = "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&sni=edge.example.com&type=ws&path=%2Fws#Example"
	body := sampleVLESS
	contentType := "text/plain; charset=utf-8"
	var err error
	switch input.Format {
	case "xray-json":
		body = `[{"outbounds":[{"protocol":"vless","tag":"Example"}]}]`
		contentType = "application/json; charset=utf-8"
	case "mihomo":
		body, err = renderMihomoSubscription(sampleVLESS)
		contentType = "application/yaml; charset=utf-8"
	case "sing-box":
		body, err = renderSingBoxSubscription(sampleVLESS)
		contentType = "application/json; charset=utf-8"
	}
	if err != nil {
		return "", "", err
	}
	body = applyTemplateContent(input.Content, body, "SubShare Preview")
	if input.Format == "base64" {
		body = base64.StdEncoding.EncodeToString([]byte(body))
	}
	if err := delivery.ValidateStructuredBody(input.Format, body); err != nil {
		return "", "", fmt.Errorf("rendered preview is not valid %s", input.Format)
	}
	return body, contentType, nil
}
