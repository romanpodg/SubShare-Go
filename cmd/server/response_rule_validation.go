package main

import (
	"fmt"
	"strings"
)

func normalizeResponseType(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "browser", "base64", "plain", "xray-json", "mihomo", "sing-box", "block", "not-found":
		return value, true
	default:
		return "", false
	}
}

func validateResponseRuleInput(input responseRuleInput) (responseRuleInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if err := validateResponseRuleMetadata(input); err != nil {
		return input, err
	}
	if err := normalizeResponseRuleResponse(&input); err != nil {
		return input, err
	}
	if err := normalizeResponseRuleConditions(input.Conditions); err != nil {
		return input, err
	}
	return input, normalizeResponseRuleHeaders(input.Headers)
}

func validateResponseRuleMetadata(input responseRuleInput) error {
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return fmt.Errorf("name must contain 1..80 characters")
	}
	if len([]rune(input.Description)) > 250 {
		return fmt.Errorf("description is too long")
	}
	return nil
}

func normalizeResponseRuleResponse(input *responseRuleInput) error {
	input.Operator = strings.ToUpper(strings.TrimSpace(input.Operator))
	if input.Operator != "AND" && input.Operator != "OR" {
		return fmt.Errorf("operator must be AND or OR")
	}
	responseType, ok := normalizeResponseType(input.ResponseType)
	if !ok {
		return fmt.Errorf("unsupported response type")
	}
	input.ResponseType = responseType
	if input.Priority < 0 || input.Priority > 100000 {
		return fmt.Errorf("priority must be between 0 and 100000")
	}
	return nil
}
