package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func (a *App) keyProfileHTTPHandler() *httpapi.KeyProfileHandler {
	return httpapi.NewKeyProfileHandler(a.keyService(), a.recordAuditEvent)
}

func (a *App) keyAdministrationHTTPHandler() *httpapi.KeyAdministrationHandler {
	return httpapi.NewKeyAdministrationHandler(a.keyService(), a.recordAuditEvent, httpapi.KeyAdministrationRuntime{
		CheckKey:       a.checkAndPersistKey,
		StartJob:       a.startTrackedJob,
		FinishJob:      a.finishTrackedJob,
		QueueHealthJob: a.queueKeyHealthCheck,
	})
}

func (a *App) keyQueryHTTPHandler() *httpapi.KeyQueryHandler {
	return httpapi.NewKeyQueryHandler(a.keyService())
}

func (a *App) apiV1GetKey(w http.ResponseWriter, r *http.Request) {
	a.keyProfileHTTPHandler().GetKey(w, r)
}

func (a *App) apiV1RevealKey(w http.ResponseWriter, r *http.Request) {
	a.keyProfileHTTPHandler().RevealKey(w, r)
}

func (a *App) apiV1CloneKey(w http.ResponseWriter, r *http.Request) {
	a.keyProfileHTTPHandler().CloneKey(w, r)
}

func (a *App) apiV1GetKeyEditorSchema(w http.ResponseWriter, r *http.Request) {
	a.keyProfileHTTPHandler().GetKeyEditorSchema(w, r)
}

func (a *App) apiV1CreateKeyProfile(w http.ResponseWriter, r *http.Request) {
	a.keyProfileHTTPHandler().CreateKeyProfile(w, r)
}

func (a *App) apiV1UpdateKeyProfile(w http.ResponseWriter, r *http.Request) {
	a.keyProfileHTTPHandler().UpdateKeyProfile(w, r)
}

func sanitizeCheckError(raw string) model.SanitizedCheckError {
	return keymanagement.SanitizeCheckError(raw)
}

func (a *App) buildSafeStructuredProfile(parsed *profiles.Profile) *model.SafeStructuredProfile {
	return keymanagement.BuildSafeStructuredProfile(parsed)
}

func (a *App) buildCapabilitiesMap(parsed *profiles.Profile) map[string]map[string]any {
	caps := make(map[string]map[string]any)
	if parsed == nil {
		return caps
	}
	matrix := subscriptionCapabilityMatrix()
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
					caps["mihomo"] = map[string]any{"status": string(capabilityUnsupported), "reason_code": generationReasonCompatibility}
					caps["sing-box"] = map[string]any{"status": string(capabilityUnsupported), "reason_code": generationReasonCompatibility}
					caps["xray-json"] = map[string]any{"status": string(capabilityUnsupported), "reason_code": generationReasonCompatibility}
					caps["plain"] = map[string]any{"status": string(capabilityCompatibility), "reason_code": "raw_delivery_only"}
					caps["base64"] = map[string]any{"status": string(capabilityCompatibility), "reason_code": "raw_delivery_only"}
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

func (a *App) buildKeyProfileDetailResponse(key model.VLESSKey, decryptedURI string) model.KeyProfileDetailResponse {
	return keymanagement.BuildKeyProfileDetailResponse(key, decryptedURI, a.buildCapabilitiesMap)
}

func (a *App) fetchKeyByID(id int64) (*model.VLESSKey, string, error) {
	key, decURI, err := a.keyService().GetRawByID(context.Background(), id)
	if errors.Is(err, profilepersistence.ErrProfileNotFound) || errors.Is(err, keymanagement.ErrKeyNotFound) {
		return nil, "", sql.ErrNoRows
	}
	return key, decURI, err
}

func (a *App) keyService() *keymanagement.Service {
	profileRepo := storage.NewProfileRepository(a.db, a.profileKeyring)
	keyRepo := storage.NewKeyRepository(a.db, a.profileKeyring)
	return keymanagement.NewService(profileRepo, keyRepo, a.buildCapabilitiesMap)
}

func (a *App) buildURIFromStructuredCreate(proto, label string, patch *model.StructuredProfilePatch) (string, error) {
	return keymanagement.BuildURIFromStructuredCreate(proto, label, patch)
}

func (a *App) applyStructuredPatchToURI(protocol, currentURI, label string, patch *model.StructuredProfilePatch) (string, error) {
	return keymanagement.ApplyStructuredPatchToURI(protocol, currentURI, label, patch)
}
