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

func TestAPITokenAllowsDeniesPrivilegeEscalation(t *testing.T) {
	t.Parallel()
	// Creating administrators and minting tokens is ownership itself. These
	// paths previously fell through to the permissive default, so any token
	// holding settings:write could mint an owner account or a "*" token.
	for _, path := range []string{
		"/api/v1/admins",
		"/api/v1/admins/1",
		"/api/admin/admins",
		"/api/v1/api-tokens",
		"/api/v1/api-tokens/1",
	} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			for _, scopes := range [][]string{
				{"settings:write"},
				{"read", "settings:write"},
				{"users:write", "keys:write", "settings:write"},
			} {
				if apiTokenAllows(scopes, method, path) {
					t.Errorf("scopes %v must not allow %s %s", scopes, method, path)
				}
			}
			if !apiTokenAllows([]string{"*"}, method, path) {
				t.Errorf("unrestricted token should allow %s %s", method, path)
			}
		}
	}
}

func TestAPITokenAllowsDeniesUnknownWritePaths(t *testing.T) {
	t.Parallel()
	// An unrecognized write endpoint is far more likely to be newly added and
	// privileged than to be safe, so it is denied until mapped explicitly.
	for _, scopes := range [][]string{
		{"settings:write"},
		{"users:write"},
		{"keys:write"},
		{"read"},
	} {
		if apiTokenAllows(scopes, http.MethodPost, "/api/v1/some-future-endpoint") {
			t.Errorf("scopes %v must not allow an unmapped write path", scopes)
		}
	}
}

func TestAPITokenKeyScopeCoversSupportedAndCompatibilityRoutes(t *testing.T) {
	t.Parallel()
	keyScopes := []string{"keys:write"}
	for _, operation := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/keys"},
		{http.MethodPut, "/api/v1/keys/1"},
		{http.MethodDelete, "/api/v1/keys/1"},
		{http.MethodPost, "/api/v1/keys/1/reveal"},
		{http.MethodPost, "/api/v1/keys/1/clone"},
		{http.MethodPost, "/api/v1/keys/1/check"},
		{http.MethodPost, "/api/v1/keys/check-all"},
		{http.MethodPost, "/api/v1/keys/bulk/status"},
		{http.MethodPost, "/api/v1/keys/bulk/delete"},
		{http.MethodPut, "/api/v1/keys/order"},
		{http.MethodPost, "/api/v1/key-categories"},
		{http.MethodPut, "/api/v1/key-categories/order"},
		{http.MethodPost, "/api/v1/key-categories/delete"},
		{http.MethodPost, "/api/v1/sources"},
		{http.MethodPut, "/api/v1/sources/1"},
		{http.MethodDelete, "/api/v1/sources/1"},
		{http.MethodPost, "/api/v1/sources/1/sync"},
		{http.MethodPost, "/api/admin/keys"},
		{http.MethodPut, "/api/admin/keys/1"},
		{http.MethodDelete, "/api/admin/keys/1"},
		{http.MethodPost, "/api/admin/keys/1/check"},
		{http.MethodPost, "/api/admin/key-categories"},
		{http.MethodPost, "/api/admin/external-sources/import"},
		{http.MethodPost, "/api/admin/external-sources/1/sync"},
	} {
		if !apiTokenAllows(keyScopes, operation.method, operation.path) {
			t.Errorf("keys:write should allow %s %s", operation.method, operation.path)
		}
		if apiTokenAllows([]string{"read"}, operation.method, operation.path) {
			t.Errorf("read should not allow %s %s", operation.method, operation.path)
		}
		if apiTokenAllows([]string{"settings:write"}, operation.method, operation.path) {
			t.Errorf("settings:write should not allow %s %s", operation.method, operation.path)
		}
	}

	for _, path := range []string{"/api/v1/keys", "/api/v1/keys/1", "/api/v1/key-categories", "/api/v1/sources"} {
		if !apiTokenAllows([]string{"read"}, http.MethodGet, path) {
			t.Errorf("read should allow GET %s", path)
		}
	}
}
