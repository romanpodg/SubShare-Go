package main

import (
	"net/http"
	"strings"
)

type adminAuthDenial struct {
	status        int
	code, message string
}
type adminRoleRequest struct{ role, method string }
type adminRolePolicy func(adminRoleRequest) *adminAuthDenial

func rejectViewerMutation(request adminRoleRequest) *adminAuthDenial {
	if isUnsafeHTTPMethod(request.method) && request.role == "viewer" {
		return &adminAuthDenial{http.StatusForbidden, "viewer_read_only", "viewer role is read-only"}
	}
	return nil
}
func rejectNonOwner(request adminRoleRequest) *adminAuthDenial {
	if !isOwnerRole(request.role) {
		return &adminAuthDenial{http.StatusForbidden, "owner_required", "owner role is required"}
	}
	return nil
}

// Preserve role, token-scope and CSRF denial order. Handlers retain their own
// principal queries; no request-context principal cache is introduced.
func (a *App) requireAdministrativeAccess(next http.Handler, rolePolicy adminRolePolicy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _, ok := a.adminSessionFromRequest(r)
		if !ok {
			writeAdminAuthError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if denial := rolePolicy(adminRoleRequest{session.Role, r.Method}); denial != nil {
			writeAdminAuthError(w, r, denial.status, denial.code, denial.message)
			return
		}
		if session.AuthKind == "api_token" && !apiTokenAllows(session.Scopes, r.Method, r.URL.Path) {
			writeAdminAuthError(w, r, http.StatusForbidden, "token_scope_forbidden", "API token scope does not allow this action")
			return
		}
		if isUnsafeHTTPMethod(r.Method) && session.AuthKind != "api_token" {
			csrfToken := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
			if !secureEqual(csrfToken, session.CSRFToken) {
				writeAdminAuthError(w, r, http.StatusForbidden, "csrf_invalid", "invalid CSRF token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
