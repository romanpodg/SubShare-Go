package storage

import "testing"

// Assertions keep independent expected values visible in the contract tests.
func requireFixture(t *testing.T, condition bool, values ...any) {
	t.Helper()
	if !condition {
		t.Fatal(values...)
	}
}

func requireFixturef(t *testing.T, condition bool, format string, values ...any) {
	t.Helper()
	if !condition {
		t.Fatalf(format, values...)
	}
}
