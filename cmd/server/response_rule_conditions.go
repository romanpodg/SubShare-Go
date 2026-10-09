package main

import (
	"fmt"
	"regexp"
	"strings"
)

// normalizeResponseRuleCondition trims and upper/lower-cases the condition in
// place and validates its operator and value.
func normalizeResponseRuleCondition(condition *responseRuleCondition) error {
	condition.HeaderName = strings.ToLower(strings.TrimSpace(condition.HeaderName))
	condition.Operator = strings.ToUpper(strings.TrimSpace(condition.Operator))
	condition.Value = strings.TrimSpace(condition.Value)
	if err := validateResponseConditionHeader(condition.HeaderName); err != nil {
		return err
	}
	if !supportedResponseConditionOperator(condition.Operator) {
		return fmt.Errorf("unsupported condition operator")
	}
	if condition.Value == "" || len(condition.Value) > 255 {
		return fmt.Errorf("condition value must contain 1..255 characters")
	}
	return validateResponseConditionRegex(condition.Operator, condition.Value)
}

func validateResponseConditionHeader(name string) error {
	if name == "" || len(name) > 80 {
		return fmt.Errorf("invalid condition header name")
	}
	if strings.ContainsAny(name, "\r\n:") {
		return fmt.Errorf("invalid condition header name")
	}
	return nil
}

func supportedResponseConditionOperator(operator string) bool {
	switch operator {
	case "EQUALS", "NOT_EQUALS", "CONTAINS", "NOT_CONTAINS", "STARTS_WITH", "NOT_STARTS_WITH", "ENDS_WITH", "NOT_ENDS_WITH", "REGEX", "NOT_REGEX":
		return true
	default:
		return false
	}
}

func validateResponseConditionRegex(operator, value string) error {
	if operator != "REGEX" && operator != "NOT_REGEX" {
		return nil
	}
	if _, err := regexp.Compile(value); err != nil {
		return fmt.Errorf("invalid condition regex")
	}
	return nil
}

func normalizeResponseRuleConditions(conditions []responseRuleCondition) error {
	if len(conditions) > 20 {
		return fmt.Errorf("too many conditions")
	}
	for index := range conditions {
		if err := normalizeResponseRuleCondition(&conditions[index]); err != nil {
			return err
		}
	}
	return nil
}
