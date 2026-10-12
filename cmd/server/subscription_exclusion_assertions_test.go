package main

import (
	"encoding/json"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func assertAllExcludedDeliveryResponse(t *testing.T, recorder *httptest.ResponseRecorder, reason string, forbidden []string) {
	t.Helper()
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, want 422", recorder.Code)
	}
	var payload delivery.Failure
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode safe all-excluded response: %v", err)
	}
	if payload.ErrorCode != delivery.ReasonAllExcluded || payload.EligibleCount != 1 || payload.ExcludedCount != 1 || payload.ExclusionCounts[reason] != 1 {
		t.Fatalf("unexpected safe all-excluded summary: code=%q eligible=%d excluded=%d", payload.ErrorCode, payload.EligibleCount, payload.ExcludedCount)
	}
	if recorder.Header().Get("SubShare-Excluded-Count") != "1" || recorder.Header().Get("SubShare-Exclusion-Codes") != reason || recorder.Header().Get("SubShare-Exclusion-Counts") != reason+"=1" {
		t.Fatal("all-excluded headers do not match the response summary")
	}
	diagnostics := recorder.Body.String() + recorder.Header().Get("SubShare-Exclusion-Codes") + recorder.Header().Get("SubShare-Exclusion-Counts")
	for _, forbidden := range forbidden {
		if strings.Contains(diagnostics, forbidden) {
			t.Fatal("all-excluded response leaked per-profile material")
		}
	}
}
