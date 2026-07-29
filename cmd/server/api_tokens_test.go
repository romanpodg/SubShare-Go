package main

import (
	"net/http"
	"testing"
)

func TestNormalizeAPITokenScopes(t *testing.T) {
	t.Parallel()
	scopes, ok := normalizeAPITokenScopes([]string{"read", "users:write", "read"})
	if !ok {
		t.Fatal("valid scopes were rejected")
	}
	if len(scopes) != 2 {
		t.Fatalf("scope count = %d, want 2", len(scopes))
	}
	if _, ok := normalizeAPITokenScopes([]string{"admin"}); ok {
		t.Fatal("unknown scope was accepted")
	}
}

func TestAPITokenAllows(t *testing.T) {
	t.Parallel()
	scopes := []string{"read", "users:write"}
	if !apiTokenAllows(scopes, http.MethodGet, "/api/v1/dashboard") {
		t.Fatal("read scope should allow GET")
	}
	if !apiTokenAllows(scopes, http.MethodPost, "/api/v1/users") {
		t.Fatal("users:write should allow user mutation")
	}
	if apiTokenAllows(scopes, http.MethodPost, "/api/v1/templates") {
		t.Fatal("users:write should not allow settings mutation")
	}
}
