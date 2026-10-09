package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestResponsePolicyConditionContracts(t *testing.T) {
	header := http.Header{"X-Fixture": {"Alpha", "BETA"}}
	cases := []struct {
		name, operator, value string
		sensitive, want       bool
	}{
		{"equals-folded", "EQUALS", "alpha,beta", false, true},
		{"equals-case", "EQUALS", "alpha,beta", true, false},
		{"not-equals", "NOT_EQUALS", "other", false, true},
		{"contains", "CONTAINS", "BETA", false, true},
		{"not-contains", "NOT_CONTAINS", "other", true, true},
		{"starts", "STARTS_WITH", "Al", true, true},
		{"not-starts", "NOT_STARTS_WITH", "beta", false, true},
		{"ends", "ENDS_WITH", "BETA", true, true},
		{"not-ends", "NOT_ENDS_WITH", "other", false, true},
		{"regex", "REGEX", "^alpha,beta$", false, true},
		{"not-regex", "NOT_REGEX", "^other$", false, true},
		{"invalid-regex-closed", "REGEX", "[", false, false},
		{"invalid-negative-regex-closed", "NOT_REGEX", "[", false, false},
		{"unknown-closed", "UNKNOWN", "alpha", false, false},
		{"raw-lowercase-operator-closed", "equals", "alpha,beta", false, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			condition := responseRuleCondition{"x-fixture", test.operator, test.value, test.sensitive}
			if got := ruleConditionMatches(condition, header); got != test.want {
				t.Fatalf("match = %v, want %v", got, test.want)
			}
		})
	}
}

func TestResponsePolicyAbsentAndRegexCharacterization(t *testing.T) {
	cases := []struct {
		name, operator, value string
		header                http.Header
		want                  bool
	}{
		{"absent-equals", "EQUALS", "value", nil, false},
		{"absent-not-equals", "NOT_EQUALS", "value", nil, true},
		{"absent-contains", "CONTAINS", "value", nil, false},
		{"absent-not-contains", "NOT_CONTAINS", "value", nil, true},
		{"raw-empty-equals", "EQUALS", "", nil, true},
		{"untrimmed-header", "EQUALS", "value", http.Header{"X-Fixture": {" value "}}, false},
		// The existing insensitive path lowercases the regex itself, including escapes.
		{"lowercased-regex-escape", "REGEX", `\D+`, http.Header{"X-Fixture": {"123"}}, true},
		{"negative-lowercased-escape", "NOT_REGEX", `\D+`, http.Header{"X-Fixture": {"123"}}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := ruleConditionMatches(responseRuleCondition{"x-fixture", test.operator, test.value, false}, test.header); got != test.want {
				t.Fatalf("match = %v, want %v", got, test.want)
			}
		})
	}
}

func TestResponsePolicyAggregationContracts(t *testing.T) {
	match := responseRuleCondition{"x-fixture", "EQUALS", "value", true}
	miss := responseRuleCondition{"x-fixture", "EQUALS", "other", true}
	cases := []struct {
		operator   string
		conditions []responseRuleCondition
		want       bool
	}{
		{"AND", nil, true}, {"OR", nil, true}, {"UNKNOWN", nil, true},
		{"AND", []responseRuleCondition{match, match}, true},
		{"AND", []responseRuleCondition{match, miss}, false},
		{"OR", []responseRuleCondition{miss, match}, true},
		{"OR", []responseRuleCondition{miss, miss}, false},
		{"or", []responseRuleCondition{match, miss}, false},
		{"UNKNOWN", []responseRuleCondition{match, miss}, false},
	}
	for index, test := range cases {
		rule := responseRule{Enabled: false, Operator: test.operator, Conditions: test.conditions}
		if got := responseRuleMatches(rule, http.Header{"X-Fixture": {"value"}}); got != test.want {
			t.Fatalf("case %d match = %v, want %v", index, got, test.want)
		}
	}
}

func responsePolicyInput() responseRuleInput {
	return responseRuleInput{
		Name: " Rule ", Description: " Description ", Operator: " and ", ResponseType: " PLAIN ", Priority: 10,
		Conditions: []responseRuleCondition{{" X-Fixture ", " equals ", " value ", false}},
		Headers:    []responseHeader{{" x-cache ", " yes "}},
	}
}

func TestResponsePolicyValidationNormalizationAndAliasing(t *testing.T) {
	input := responsePolicyInput()
	got, err := validateResponseRuleInput(input)
	if err != nil {
		t.Fatal(err)
	}
	if [4]string{got.Name, got.Description, got.Operator, got.ResponseType} != [4]string{"Rule", "Description", "AND", "plain"} {
		t.Fatal("normalized scalar contract changed")
	}
	if input.Conditions[0] != (responseRuleCondition{"x-fixture", "EQUALS", "value", false}) {
		t.Fatal("condition normalization must retain existing slice alias behavior")
	}
	if input.Headers[0] != (responseHeader{"X-Cache", "yes"}) {
		t.Fatal("header normalization must retain existing slice alias behavior")
	}
}

func TestResponsePolicyValidationBoundaries(t *testing.T) {
	cases := []struct {
		name, message string
		change        func(*responseRuleInput)
	}{
		{"empty-name", "name must contain 1..80 characters", func(v *responseRuleInput) { v.Name = " " }},
		{"unicode-name-too-long", "name must contain 1..80 characters", func(v *responseRuleInput) { v.Name = strings.Repeat("名", 81) }},
		{"description-too-long", "description is too long", func(v *responseRuleInput) { v.Description = strings.Repeat("名", 251) }},
		{"operator", "operator must be AND or OR", func(v *responseRuleInput) { v.Operator = "XOR" }},
		{"response", "unsupported response type", func(v *responseRuleInput) { v.ResponseType = "unknown" }},
		{"negative-priority", "priority must be between 0 and 100000", func(v *responseRuleInput) { v.Priority = -1 }},
		{"high-priority", "priority must be between 0 and 100000", func(v *responseRuleInput) { v.Priority = 100001 }},
		{"condition-count", "too many conditions", func(v *responseRuleInput) { v.Conditions = make([]responseRuleCondition, 21) }},
		{"condition-name-bytes", "invalid condition header name", func(v *responseRuleInput) { v.Conditions[0].HeaderName = strings.Repeat("名", 27) }},
		{"condition-value-empty", "condition value must contain 1..255 characters", func(v *responseRuleInput) { v.Conditions[0].Value = " " }},
		{"invalid-regex", "invalid condition regex", func(v *responseRuleInput) { v.Conditions[0].Operator = "REGEX"; v.Conditions[0].Value = "[" }},
		{"header-count", "too many response headers", func(v *responseRuleInput) { v.Headers = make([]responseHeader, 31) }},
		{"reserved-header", `unsafe response header "Profile-Title"`, func(v *responseRuleInput) { v.Headers[0].Key = " profile-title " }},
		{"header-injection", `unsafe response header "X-Cache"`, func(v *responseRuleInput) { v.Headers[0].Value = "value\r\nInjected: x" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := responsePolicyInput()
			test.change(&input)
			_, err := validateResponseRuleInput(input)
			if err == nil {
				t.Fatal("invalid input accepted")
			}
			if err.Error() != test.message {
				t.Fatalf("error = %q, want %q", err.Error(), test.message)
			}
		})
	}
}

func TestResponseTemplatePolicyContracts(t *testing.T) {
	cases := []struct {
		format, content, message string
	}{
		{"plain", "without marker", ""}, {"base64", "without marker", ""},
		{"mihomo", " \n\t", ""}, {"sing-box", "{{subscription}}", ""}, {"xray-json", "{{subscription}}", ""},
		{"mihomo", "without marker", "custom template must contain {{subscription}}"},
		{"sing-box", "without marker", "custom template must contain {{subscription}}"},
		{"xray-json", "without marker", "custom template must contain {{subscription}}"},
		{"unknown", "", "unsupported template format"},
		{"plain", strings.Repeat("x", (1<<20)+1), "template content is too large"},
	}
	for _, test := range cases {
		t.Run(test.format+test.message, func(t *testing.T) {
			_, err := validateTemplateInput(templateInput{Name: " Template ", Format: test.format, Content: test.content})
			message := ""
			if err != nil {
				message = err.Error()
			}
			if message != test.message {
				t.Fatalf("error = %q, want %q", message, test.message)
			}
		})
	}
}
