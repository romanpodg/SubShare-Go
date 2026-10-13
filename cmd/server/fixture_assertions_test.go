package main

import "testing"

// Keep each expected field and guard visible at its call site. The helpers only
// report failures; they do not compute expected results from production code.
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

func checkFixturef(t *testing.T, condition bool, format string, values ...any) {
	t.Helper()
	if !condition {
		t.Errorf(format, values...)
	}
}
