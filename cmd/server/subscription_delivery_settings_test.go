package main

import "testing"

func TestValidateSubscriptionDeliverySettings(t *testing.T) {
	t.Parallel()
	input := defaultSubscriptionDeliverySettings()
	input.Announcement = "Maintenance"
	input.ResponseHeaders = []responseHeader{{Key: "X-Provider", Value: "SubShare"}}
	input.Remarks["expired"] = []string{"Subscription expired", "Contact support"}
	if _, err := validateSubscriptionDeliverySettings(input); err != nil {
		t.Fatalf("valid delivery settings rejected: %v", err)
	}

	input.ResponseHeaders = []responseHeader{{Key: "Set-Cookie", Value: "unsafe=true"}}
	if _, err := validateSubscriptionDeliverySettings(input); err == nil {
		t.Fatal("unsafe response header was accepted")
	}
}

func TestRemarkStatusFromReason(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"subscription expired":               "expired",
		"subscription paused":                "paused",
		"subscription blocked":               "blocked",
		"maximum number of devices reached":  "limited",
		"subscription has no available keys": "empty",
	}
	for reason, want := range tests {
		if got := remarkStatusFromReason(reason); got != want {
			t.Fatalf("remarkStatusFromReason(%q) = %q, want %q", reason, got, want)
		}
	}
}
