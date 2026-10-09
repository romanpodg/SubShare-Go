package main

import (
	"fmt"
	"regexp"
	"strings"
)

var templateSlugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeTemplateFormat(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "base64", "plain", "xray-json", "mihomo", "sing-box":
		return value, true
	default:
		return "", false
	}
}

func templateSlug(name string) string {
	slug := templateSlugSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "template"
	}
	return slug
}

func validateTemplateInput(input templateInput) (templateInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return input, fmt.Errorf("name must contain 1..80 characters")
	}
	format, ok := normalizeTemplateFormat(input.Format)
	if !ok {
		return input, fmt.Errorf("unsupported template format")
	}
	input.Format = format
	return input, validateTemplateContent(format, input.Content)
}

func validateTemplateContent(format, content string) error {
	if len(content) > 1<<20 {
		return fmt.Errorf("template content is too large")
	}
	if !structuredTemplateFormat(format) {
		return nil
	}
	if strings.TrimSpace(content) == "" {
		return nil
	}
	if !strings.Contains(content, "{{subscription}}") {
		return fmt.Errorf("custom template must contain {{subscription}}")
	}
	return nil
}

func structuredTemplateFormat(format string) bool {
	switch format {
	case "mihomo", "sing-box", "xray-json":
		return true
	default:
		return false
	}
}

type responseTemplatePolicyError struct {
	Code    string
	Message string
}

func validateResponseTemplateReference(responseType, format string, enabled bool) *responseTemplatePolicyError {
	if !enabled {
		return &responseTemplatePolicyError{"template_invalid", "selected template does not exist or is disabled"}
	}
	if format != responseType {
		return &responseTemplatePolicyError{"template_format_mismatch", "selected template format must match the rule response type"}
	}
	return nil
}
