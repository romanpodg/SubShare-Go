package main

import (
	"encoding/json"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"testing"
)

func decodeObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode generated JSON: %v", err)
	}
	return result
}

func exclusionCodes(exclusions []delivery.Exclusion) map[string]bool {
	result := make(map[string]bool)
	for _, exclusion := range exclusions {
		result[exclusion.Reason] = true
	}
	return result
}
