package main

import (
	"net/http"
	"regexp"
	"strings"
)

type responseConditionValues struct {
	actual   string
	expected string
}

func ruleConditionMatches(condition responseRuleCondition, header http.Header) bool {
	values := responseConditionValues{strings.Join(header.Values(condition.HeaderName), ","), condition.Value}
	if !condition.CaseSensitive {
		values.actual = strings.ToLower(values.actual)
		values.expected = strings.ToLower(values.expected)
	}
	switch condition.Operator {
	case "REGEX", "NOT_REGEX":
		return responseRegexMatches(condition.Operator, values)
	default:
		return responseTextMatches(condition.Operator, values)
	}
}

func responseTextMatches(operator string, values responseConditionValues) bool {
	var matched bool
	switch strings.TrimPrefix(operator, "NOT_") {
	case "EQUALS":
		matched = values.actual == values.expected
	case "CONTAINS":
		matched = strings.Contains(values.actual, values.expected)
	case "STARTS_WITH":
		matched = strings.HasPrefix(values.actual, values.expected)
	case "ENDS_WITH":
		matched = strings.HasSuffix(values.actual, values.expected)
	default:
		return false
	}
	return matched != strings.HasPrefix(operator, "NOT_")
}

func responseRegexMatches(operator string, values responseConditionValues) bool {
	re, err := regexp.Compile(values.expected)
	if err != nil {
		return false
	}
	matched := re.MatchString(values.actual)
	if operator == "NOT_REGEX" {
		return !matched
	}
	return matched
}

func responseRuleMatches(rule responseRule, header http.Header) bool {
	if len(rule.Conditions) == 0 {
		return true
	}
	if rule.Operator == "OR" {
		return anyResponseConditionMatches(rule.Conditions, header)
	}
	return allResponseConditionsMatch(rule.Conditions, header)
}

func anyResponseConditionMatches(conditions []responseRuleCondition, header http.Header) bool {
	for _, condition := range conditions {
		if ruleConditionMatches(condition, header) {
			return true
		}
	}
	return false
}

func allResponseConditionsMatch(conditions []responseRuleCondition, header http.Header) bool {
	for _, condition := range conditions {
		if !ruleConditionMatches(condition, header) {
			return false
		}
	}
	return true
}

// firstMatchingResponseRule consumes the existing priority/id ordered projection.
func firstMatchingResponseRule(rules []responseRule, header http.Header) *responseRule {
	for index := range rules {
		if rules[index].Enabled && responseRuleMatches(rules[index], header) {
			return &rules[index]
		}
	}
	return nil
}
