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
	requireFixturef(t, recorder.Code == http.StatusUnprocessableEntity, "status=%d, want 422", recorder.Code)

	var payload delivery.Failure
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &payload)
		requireFixturef(t, err == nil, "decode safe all-excluded response: %v", err)
	}

	requireFixturef(t, payload.ErrorCode == delivery.ReasonAllExcluded, "unexpected safe all-excluded summary: code=%q eligible=%d excluded=%d", payload.ErrorCode, payload.EligibleCount, payload.ExcludedCount)
	requireFixturef(t, payload.EligibleCount == 1, "unexpected safe all-excluded summary: code=%q eligible=%d excluded=%d", payload.ErrorCode, payload.EligibleCount, payload.ExcludedCount)
	requireFixturef(t, payload.ExcludedCount == 1, "unexpected safe all-excluded summary: code=%q eligible=%d excluded=%d", payload.ErrorCode, payload.EligibleCount, payload.ExcludedCount)
	requireFixturef(t, payload.ExclusionCounts[reason] == 1, "unexpected safe all-excluded summary: code=%q eligible=%d excluded=%d", payload.ErrorCode, payload.EligibleCount, payload.ExcludedCount)

	requireFixture(t, recorder.Header().Get("SubShare-Excluded-Count") == "1", "all-excluded headers do not match the response summary")
	requireFixture(t, recorder.Header().Get("SubShare-Exclusion-Codes") == reason, "all-excluded headers do not match the response summary")
	requireFixture(t, recorder.Header().Get("SubShare-Exclusion-Counts") == reason+"=1", "all-excluded headers do not match the response summary")

	diagnostics := recorder.Body.String() + recorder.Header().Get("SubShare-Exclusion-Codes") + recorder.Header().Get("SubShare-Exclusion-Counts")
	for _, forbidden := range forbidden {
		requireFixture(t, !(strings.Contains(diagnostics, forbidden)), "all-excluded response leaked per-profile material")

	}
}
