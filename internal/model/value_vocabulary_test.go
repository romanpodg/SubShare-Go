package model

import "testing"

type normalizableValue interface {
	Normalize() (string, bool)
}

func TestModelValueVocabularies(t *testing.T) {
	tests := []struct {
		name  string
		input normalizableValue
		want  string
		valid bool
	}{
		{"default format", SubscriptionFormat(""), "links", true},
		{"normalized format", SubscriptionFormat(" XRAY-JSON "), "xray-json", true},
		{"invalid format", SubscriptionFormat("json"), "", false},
		{"default kind", KeyKind("\t"), "real", true},
		{"normalized kind", KeyKind(" Informational "), "informational", true},
		{"invalid kind", KeyKind("active"), "", false},
		{"assignment requires value", KeyAssignmentMode(""), "", false},
		{"all assignment", KeyAssignmentMode(" ALL "), "all", true},
		{"selected assignment", KeyAssignmentMode(" Selected "), "selected", true},
		{"invalid assignment", KeyAssignmentMode("real"), "", false},
		{"user cannot use key status", UserStatus("non-active"), "", false},
		{"key cannot use user status", KeyStatus("paused"), "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := test.input.Normalize()
			if got != test.want {
				t.Errorf("value = %q, want %q", got, test.want)
			}
			if valid != test.valid {
				t.Errorf("valid = %v, want %v", valid, test.valid)
			}
		})
	}
}
