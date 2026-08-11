package model

import (
	"testing"
	"time"
)

func TestEffectiveUserStatus(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		status     string
		expiresAt  time.Time
		hasExpiry  bool
		devices    int
		maxDevices int
		want       string
	}{
		{name: "active", status: "active", want: "active"},
		{name: "blocked wins", status: "blocked", expiresAt: now.Add(-time.Hour), hasExpiry: true, want: "blocked"},
		{name: "paused wins", status: "paused", expiresAt: now.Add(-time.Hour), hasExpiry: true, want: "paused"},
		{name: "expired", status: "active", expiresAt: now.Add(-time.Second), hasExpiry: true, want: "expired"},
		{name: "limited", status: "active", devices: 2, maxDevices: 2, want: "limited"},
		{name: "unlimited with connected devices", status: "active", devices: 20, maxDevices: 0, want: "active"},
		{name: "future expiry", status: "active", expiresAt: now.Add(time.Hour), hasExpiry: true, want: "active"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			got := EffectiveUserStatus(test.status, test.expiresAt, test.hasExpiry, test.devices, test.maxDevices, now)
			if got != test.want {
				t.Fatalf("EffectiveUserStatus() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeUserStatus(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStatus string
		wantOK     bool
	}{
		{"empty string defaults to active", "", "active", true},
		{"active lowercase", "active", "active", true},
		{"active uppercase", "ACTIVE", "active", true},
		{"active with surrounding spaces", " Active ", "active", true},
		{"paused", "paused", "paused", true},
		{"blocked", "blocked", "blocked", true},
		{"paused uppercase", "PAUSED", "paused", true},
		{"blocked mixed case", "Blocked", "blocked", true},
		{"invalid status", "invalid", "", false},
		{"deleted status", "deleted", "", false},
		{"random string", "foobar", "", false},
		{"whitespace only defaults to active", "   ", "active", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotOK := NormalizeUserStatus(tt.input)
			if gotStatus != tt.wantStatus {
				t.Errorf("NormalizeUserStatus(%q) status = %q, want %q", tt.input, gotStatus, tt.wantStatus)
			}
			if gotOK != tt.wantOK {
				t.Errorf("NormalizeUserStatus(%q) ok = %v, want %v", tt.input, gotOK, tt.wantOK)
			}
		})
	}
}

func TestNormalizeStoredStatus(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string defaults to active", "", "active"},
		{"blocked stays blocked", "blocked", "blocked"},
		{"paused stays paused", "paused", "paused"},
		{"active stays active", "active", "active"},
		{"invalid falls back to active", "invalid", "active"},
		{"deleted falls back to active", "deleted", "active"},
		{"uppercase ACTIVE normalizes", "ACTIVE", "active"},
		{"random garbage falls back to active", "xyz", "active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeStoredStatus(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeStoredStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeKeyStatus(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStatus string
		wantOK     bool
	}{
		{"empty defaults to active", "", "active", true},
		{"active stays active", "active", "active", true},
		{"non-active stays non-active", "non-active", "non-active", true},
		{"blocked converts to non-active", "blocked", "non-active", true},
		{"BLOCKED uppercase converts to non-active", "BLOCKED", "non-active", true},
		{"ACTIVE uppercase normalizes", "ACTIVE", "active", true},
		{"invalid status", "invalid", "", false},
		{"paused is not valid for keys", "paused", "", false},
		{"whitespace only defaults to active", "   ", "active", true},
		{"blocked with spaces", " blocked ", "non-active", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotOK := NormalizeKeyStatus(tt.input)
			if gotStatus != tt.wantStatus {
				t.Errorf("NormalizeKeyStatus(%q) status = %q, want %q", tt.input, gotStatus, tt.wantStatus)
			}
			if gotOK != tt.wantOK {
				t.Errorf("NormalizeKeyStatus(%q) ok = %v, want %v", tt.input, gotOK, tt.wantOK)
			}
		})
	}
}

func TestKeyStatusLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"active label", "active", "active"},
		{"non-active label", "non-active", "non-active"},
		{"unknown defaults to non-active", "unknown", "non-active"},
		{"empty defaults to non-active", "", "non-active"},
		{"random string defaults to non-active", "foobar", "non-active"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KeyStatusLabel(tt.input)
			if got != tt.want {
				t.Errorf("KeyStatusLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeCheckStatus(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"up stays up", "up", "up"},
		{"down stays down", "down", "down"},
		{"unknown stays unknown", "unknown", "unknown"},
		{"unsupported check stays explicit", "unsupported_check", "unsupported_check"},
		{"empty defaults to unknown", "", "unknown"},
		{"invalid defaults to unknown", "invalid", "unknown"},
		{"UP uppercase normalizes to up", "UP", "up"},
		{"DOWN uppercase normalizes to down", "DOWN", "down"},
		{"whitespace only defaults to unknown", "   ", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeCheckStatus(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeCheckStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCheckStatusLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"up returns Доступен", "up", "Доступен"},
		{"down returns Недоступен", "down", "Недоступен"},
		{"unsupported returns neutral label", "unsupported_check", "Проверка не поддерживается"},
		{"unknown returns Не проверен", "unknown", "Не проверен"},
		{"empty returns Не проверен", "", "Не проверен"},
		{"arbitrary string returns Не проверен", "foobar", "Не проверен"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckStatusLabel(tt.input)
			if got != tt.want {
				t.Errorf("CheckStatusLabel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
