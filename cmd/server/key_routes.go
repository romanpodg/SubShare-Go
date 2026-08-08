package main

import (
	"net/http"
)

// registerKeyRoutes keeps the supported key-management surface and its
// retained compatibility adapters together. Full decrypted-key list/export
// routes are intentionally absent; secret access is only available through
// the explicit, revision-aware reveal endpoint.
func (a *App) registerKeyRoutes(mux *http.ServeMux) {
	// Supported v1 key API.
	mux.Handle("GET /api/v1/keys", a.requireAdmin(http.HandlerFunc(a.apiV1ListKeys)))
	mux.Handle("GET /api/v1/keys/{id}", a.requireAdmin(http.HandlerFunc(a.apiV1GetKey)))
	mux.Handle("POST /api/v1/keys/{id}/reveal", a.requireAdmin(http.HandlerFunc(a.apiV1RevealKey)))
	mux.Handle("POST /api/v1/keys/{id}/clone", a.requireAdmin(http.HandlerFunc(a.apiV1CloneKey)))
	mux.Handle("GET /api/v1/key-editor-schema", a.requireAdmin(http.HandlerFunc(a.apiV1GetKeyEditorSchema)))
	mux.Handle("POST /api/v1/keys", a.requireAdmin(http.HandlerFunc(a.apiV1CreateKeyProfile)))
	mux.Handle("PUT /api/v1/keys/{id}", a.requireAdmin(http.HandlerFunc(a.apiV1UpdateKeyProfile)))
	mux.Handle("DELETE /api/v1/keys/{id}", a.requireAdmin(a.v1Compatibility(http.HandlerFunc(a.apiDeleteKey))))
	mux.Handle("POST /api/v1/keys/{id}/check", a.requireAdmin(a.v1Compatibility(http.HandlerFunc(a.apiCheckKey))))
	mux.Handle("POST /api/v1/keys/check-all", a.requireAdmin(http.HandlerFunc(a.apiV1QueueKeyHealthCheck)))
	mux.Handle("GET /api/v1/key-categories", a.requireAdmin(http.HandlerFunc(a.apiV1ListKeyCategories)))
	mux.Handle("POST /api/v1/key-categories", a.requireAdmin(http.HandlerFunc(a.apiCreateKeyCategory)))
	mux.Handle("PUT /api/v1/key-categories", a.requireAdmin(http.HandlerFunc(a.apiUpdateKeyCategory)))
	mux.Handle("PUT /api/v1/key-categories/order", a.requireAdmin(http.HandlerFunc(a.apiReorderKeyCategories)))
	mux.Handle("PUT /api/v1/key-categories/rename", a.requireAdmin(http.HandlerFunc(a.apiRenameKeyCategory)))
	mux.Handle("POST /api/v1/key-categories/delete", a.requireAdmin(http.HandlerFunc(a.apiDeleteKeyCategory)))
	mux.Handle("POST /api/v1/keys/bulk/status", a.requireAdmin(http.HandlerFunc(a.apiBulkUpdateKeyStatus)))
	mux.Handle("POST /api/v1/keys/bulk/delete", a.requireAdmin(http.HandlerFunc(a.apiBulkDeleteKeys)))
	mux.Handle("PUT /api/v1/keys/order", a.requireAdmin(http.HandlerFunc(a.apiReorderKeys)))

	// Deprecated /api/admin compatibility API. These handlers are transport
	// adapters over the same key-management application service used by v1.
	mux.Handle("GET /api/admin/key-categories", a.requireAdmin(http.HandlerFunc(a.apiListKeyCategories)))
	mux.Handle("POST /api/admin/key-categories", a.requireAdmin(http.HandlerFunc(a.apiCreateKeyCategory)))
	mux.Handle("PUT /api/admin/key-categories", a.requireAdmin(http.HandlerFunc(a.apiUpdateKeyCategory)))
	mux.Handle("PUT /api/admin/key-categories/order", a.requireAdmin(http.HandlerFunc(a.apiReorderKeyCategories)))
	mux.Handle("PUT /api/admin/key-categories/rename", a.requireAdmin(http.HandlerFunc(a.apiRenameKeyCategory)))
	mux.Handle("POST /api/admin/key-categories/delete", a.requireAdmin(http.HandlerFunc(a.apiDeleteKeyCategory)))
	mux.Handle("POST /api/admin/keys", a.requireAdmin(http.HandlerFunc(a.apiCreateKey)))
	mux.Handle("POST /api/admin/keys/bulk/status", a.requireAdmin(http.HandlerFunc(a.apiBulkUpdateKeyStatus)))
	mux.Handle("POST /api/admin/keys/bulk/delete", a.requireAdmin(http.HandlerFunc(a.apiBulkDeleteKeys)))
	mux.Handle("PUT /api/admin/keys/order", a.requireAdmin(http.HandlerFunc(a.apiReorderKeys)))
	mux.Handle("PUT /api/admin/keys/{id}", a.requireAdmin(http.HandlerFunc(a.apiUpdateKey)))
	mux.Handle("DELETE /api/admin/keys/{id}", a.requireAdmin(http.HandlerFunc(a.apiDeleteKey)))
	mux.Handle("POST /api/admin/keys/{id}/check", a.requireAdmin(http.HandlerFunc(a.apiCheckKey)))
	mux.Handle("POST /api/admin/keys/check-all", a.requireAdmin(http.HandlerFunc(a.apiCheckAllKeys)))
}
