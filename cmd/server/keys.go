package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"net/http"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

// keyHandlers is the key-management object graph, built once per App.
type keyHandlers struct {
	service    *keymanagement.Service
	profiles   *httpapi.KeyProfileHandler
	admin      *httpapi.KeyAdministrationHandler
	queries    *httpapi.KeyQueryHandler
	legacy     *httpapi.LegacyKeyHandler
	categories *httpapi.KeyCategoryHandler
}

func (a *App) keys() *keyHandlers {
	a.keysOnce.Do(func() {
		service := keymanagement.NewService(a.store(), a.buildCapabilitiesMap)
		a.keyHandlers = &keyHandlers{
			service:  service,
			profiles: httpapi.NewKeyProfileHandler(service, a.recordAuditEvent),
			admin: httpapi.NewKeyAdministrationHandler(service, a.recordAuditEvent, httpapi.KeyAdministrationRuntime{
				CheckKey:       a.checkAndPersistKey,
				StartJob:       a.startTrackedJob,
				FinishJob:      a.finishTrackedJob,
				QueueHealthJob: a.queueKeyHealthCheck,
			}),
			queries:    httpapi.NewKeyQueryHandler(service),
			legacy:     httpapi.NewLegacyKeyHandler(service, a.recordAuditEvent),
			categories: httpapi.NewKeyCategoryHandler(service, a.recordAuditEvent),
		}
	})
	return a.keyHandlers
}

func (a *App) keyService() *keymanagement.Service {
	return a.keys().service
}

// registerKeyRoutes keeps the supported key-management surface and its
// retained compatibility adapters together. Full decrypted-key list/export
// routes are intentionally absent; secret access is only available through
// the explicit, revision-aware reveal endpoint.
func (a *App) registerKeyRoutes(mux *http.ServeMux) {
	k := a.keys()
	admin := func(h http.HandlerFunc) http.Handler { return a.requireAdmin(h) }

	// Supported v1 key API.
	mux.Handle("GET /api/v1/keys", admin(k.queries.ListKeys))
	mux.Handle("GET /api/v1/keys/{id}", admin(k.profiles.GetKey))
	mux.Handle("POST /api/v1/keys/{id}/reveal", admin(k.profiles.RevealKey))
	mux.Handle("POST /api/v1/keys/{id}/clone", admin(k.profiles.CloneKey))
	mux.Handle("GET /api/v1/key-editor-schema", admin(k.profiles.GetKeyEditorSchema))
	mux.Handle("POST /api/v1/keys", admin(k.profiles.CreateKeyProfile))
	mux.Handle("PUT /api/v1/keys/{id}", admin(k.profiles.UpdateKeyProfile))
	mux.Handle("DELETE /api/v1/keys/{id}", a.requireAdmin(a.v1Compatibility(http.HandlerFunc(k.legacy.DeleteKey))))
	mux.Handle("POST /api/v1/keys/{id}/check", a.requireAdmin(a.v1Compatibility(http.HandlerFunc(k.admin.CheckKey))))
	mux.Handle("POST /api/v1/keys/check-all", admin(k.admin.QueueHealthCheck))
	mux.Handle("GET /api/v1/key-categories", admin(k.queries.ListCategories))
	mux.Handle("POST /api/v1/key-categories", admin(k.categories.CreateCategory))
	mux.Handle("PUT /api/v1/key-categories", admin(k.categories.UpdateCategory))
	mux.Handle("PUT /api/v1/key-categories/order", admin(k.categories.ReorderCategories))
	mux.Handle("PUT /api/v1/key-categories/rename", admin(k.categories.RenameCategory))
	mux.Handle("POST /api/v1/key-categories/delete", admin(k.categories.DeleteCategory))
	mux.Handle("POST /api/v1/keys/bulk/status", admin(k.admin.BulkUpdateKeys))
	mux.Handle("POST /api/v1/keys/bulk/delete", admin(k.admin.BulkDeleteKeys))
	mux.Handle("PUT /api/v1/keys/order", admin(k.categories.ReorderKeys))

	// Deprecated /api/admin compatibility API over the same handlers.
	mux.Handle("GET /api/admin/key-categories", admin(k.categories.ListCategories))
	mux.Handle("POST /api/admin/key-categories", admin(k.categories.CreateCategory))
	mux.Handle("PUT /api/admin/key-categories", admin(k.categories.UpdateCategory))
	mux.Handle("PUT /api/admin/key-categories/order", admin(k.categories.ReorderCategories))
	mux.Handle("PUT /api/admin/key-categories/rename", admin(k.categories.RenameCategory))
	mux.Handle("POST /api/admin/key-categories/delete", admin(k.categories.DeleteCategory))
	mux.Handle("POST /api/admin/keys", admin(k.legacy.CreateKey))
	mux.Handle("POST /api/admin/keys/bulk/status", admin(k.admin.BulkUpdateKeys))
	mux.Handle("POST /api/admin/keys/bulk/delete", admin(k.admin.BulkDeleteKeys))
	mux.Handle("PUT /api/admin/keys/order", admin(k.categories.ReorderKeys))
	mux.Handle("PUT /api/admin/keys/{id}", admin(k.legacy.UpdateKey))
	mux.Handle("DELETE /api/admin/keys/{id}", admin(k.legacy.DeleteKey))
	mux.Handle("POST /api/admin/keys/{id}/check", admin(k.admin.CheckKey))
	mux.Handle("POST /api/admin/keys/check-all", admin(k.admin.CheckAllKeys))
}

// buildCapabilitiesMap projects the delivery capability matrix onto one parsed
// profile for the key detail response.
func (a *App) buildCapabilitiesMap(parsed *profiles.Profile) map[string]map[string]any {
	caps := make(map[string]map[string]any)
	if parsed == nil {
		return caps
	}
	matrix := delivery.CapabilityMatrix()
	for _, item := range matrix {
		if strings.EqualFold(item.Protocol, string(parsed.Protocol)) {
			for fmtName, outCap := range item.Outputs {
				payload := map[string]any{
					"status": string(outCap.Status),
				}
				if outCap.ReasonCode != "" {
					payload["reason_code"] = outCap.ReasonCode
				}
				if outCap.TargetVersion != "" {
					payload["target_version"] = outCap.TargetVersion
				}
				caps[fmtName] = payload
			}

			// Apply TUIC v4 overrides
			if parsed.Protocol == profiles.ProtocolTUIC && parsed.Data != nil {
				if tuicData, ok := parsed.Data.(profiles.TUICData); ok && tuicData.Generation == 4 {
					caps["mihomo"] = map[string]any{"status": string(delivery.CapabilityUnsupported), "reason_code": delivery.ReasonCompatibility}
					caps["sing-box"] = map[string]any{"status": string(delivery.CapabilityUnsupported), "reason_code": delivery.ReasonCompatibility}
					caps["xray-json"] = map[string]any{"status": string(delivery.CapabilityUnsupported), "reason_code": delivery.ReasonCompatibility}
					caps["plain"] = map[string]any{"status": string(delivery.CapabilityCompatibility), "reason_code": "raw_delivery_only"}
					caps["base64"] = map[string]any{"status": string(delivery.CapabilityCompatibility), "reason_code": "raw_delivery_only"}
				}
			}
			break
		}
	}
	if len(caps) == 0 {
		caps["plain"] = map[string]any{"status": "supported"}
		caps["base64"] = map[string]any{"status": "supported"}
	}
	return caps
}

func (a *App) fetchKeyByID(id int64) (*model.VLESSKey, string, error) {
	key, decURI, err := a.keyService().GetRawByID(context.Background(), id)
	if errors.Is(err, keymanagement.ErrKeyNotFound) {
		return nil, "", sql.ErrNoRows
	}
	return key, decURI, err
}
