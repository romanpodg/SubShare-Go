package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

type KeyAdministrationRuntime struct {
	CheckKey       func(ctx context.Context, keyID int64, rawURL string) error
	StartJob       func(kind, targetType, targetID string) int64
	FinishJob      func(jobID int64, err error)
	QueueHealthJob func(r *http.Request) int64
}

type KeyAdministrationHandler struct {
	service *keymanagement.Service
	audit   AuditRecorder
	runtime KeyAdministrationRuntime
}

func NewKeyAdministrationHandler(service *keymanagement.Service, audit AuditRecorder, runtime KeyAdministrationRuntime) *KeyAdministrationHandler {
	return &KeyAdministrationHandler{service: service, audit: audit, runtime: runtime}
}

func (h *KeyAdministrationHandler) BulkUpdateKeys(w http.ResponseWriter, r *http.Request) {
	var req model.BulkUpdateKeyStatusRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.service.BulkUpdateKeys(r.Context(), keymanagement.BulkUpdateKeysParams{
		IDs: req.IDs, Status: req.Status, Category: req.Category,
	})
	if err != nil {
		switch {
		case errors.Is(err, keymanagement.ErrBulkKeyIDsRequired),
			errors.Is(err, keymanagement.ErrInvalidBulkKeyID),
			errors.Is(err, keymanagement.ErrDuplicateBulkKeyIDs):
			WriteError(w, r, http.StatusBadRequest, err.Error())
		case errors.Is(err, keymanagement.ErrInvalidKeyStatus):
			WriteError(w, r, http.StatusBadRequest, "invalid key status")
		case errors.Is(err, keymanagement.ErrBulkCategoryPersistence):
			log.Printf("BulkUpdateKeys category: %v", err)
			WriteError(w, r, http.StatusInternalServerError, "failed to save key category")
		default:
			if id, ok := keymanagement.MissingKeyID(err); ok {
				WriteError(w, r, http.StatusNotFound, fmt.Sprintf("key not found: %d", id))
				return
			}
			log.Printf("BulkUpdateKeys: %v", err)
			WriteError(w, r, http.StatusInternalServerError, "failed to update keys")
		}
		return
	}

	status, _ := model.NormalizeKeyStatus(req.Status)
	if h.audit != nil {
		h.audit(r, "keys.bulk_status", "key", "multiple", map[string]any{"count": updated, "status": status})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "keys updated", "updated": updated})
}

func (h *KeyAdministrationHandler) BulkDeleteKeys(w http.ResponseWriter, r *http.Request) {
	var req model.BulkDeleteKeysRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	deleted, err := h.service.BulkDeleteKeys(r.Context(), req.IDs)
	if err != nil {
		switch {
		case errors.Is(err, keymanagement.ErrBulkKeyIDsRequired),
			errors.Is(err, keymanagement.ErrInvalidBulkKeyID),
			errors.Is(err, keymanagement.ErrDuplicateBulkKeyIDs):
			WriteError(w, r, http.StatusBadRequest, err.Error())
		default:
			if id, ok := keymanagement.MissingKeyID(err); ok {
				WriteError(w, r, http.StatusNotFound, fmt.Sprintf("key not found: %d", id))
				return
			}
			log.Printf("BulkDeleteKeys: %v", err)
			WriteError(w, r, http.StatusInternalServerError, "failed to delete keys")
		}
		return
	}

	if h.audit != nil {
		h.audit(r, "keys.bulk_delete", "key", "multiple", map[string]any{"count": deleted})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "keys deleted", "deleted": deleted})
}

func (h *KeyAdministrationHandler) CheckKey(w http.ResponseWriter, r *http.Request) {
	id, ok := PathID(w, r, "id")
	if !ok {
		return
	}
	target, err := h.service.GetHealthCheckTarget(r.Context(), id)
	switch {
	case errors.Is(err, keymanagement.ErrKeyNotFound):
		WriteError(w, r, http.StatusNotFound, "key not found")
		return
	case errors.Is(err, keymanagement.ErrInformationalHealthCheck):
		WriteError(w, r, http.StatusBadRequest, "informational keys do not require checks")
		return
	case errors.Is(err, keymanagement.ErrCredentialMissing):
		WriteError(w, r, http.StatusInternalServerError, "missing profile key secret")
		return
	case err != nil:
		WriteError(w, r, http.StatusInternalServerError, "failed to decrypt key")
		return
	}

	if h.runtime.CheckKey == nil {
		WriteError(w, r, http.StatusInternalServerError, "failed to check key")
		return
	}
	if err := h.runtime.CheckKey(r.Context(), id, target.URL); err != nil {
		log.Printf("CheckKey: %v", err)
		WriteError(w, r, http.StatusInternalServerError, "failed to check key")
		return
	}
	h.respondKeyCheck(w, r, id)
}

func (h *KeyAdministrationHandler) CheckAllKeys(w http.ResponseWriter, r *http.Request) {
	targets, err := h.service.ListHealthCheckTargets(r.Context())
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "failed to load keys")
		return
	}

	jobID := int64(0)
	if h.runtime.StartJob != nil {
		jobID = h.runtime.StartJob("keys_health_check", "key", "all")
	}
	eligibleTargets := make([]keymanagement.HealthCheckTarget, 0, len(targets))
	for _, target := range targets {
		status, _ := model.NormalizeKeyStatus(target.Status)
		kind, _ := model.NormalizeKeyKind(target.Kind)
		if status != model.KeyStatusActive || kind == model.KeyKindInformational || target.Unreadable || target.URL == "" {
			continue
		}
		eligibleTargets = append(eligibleTargets, target)
	}
	var waitGroup sync.WaitGroup
	semaphore := make(chan struct{}, 10)
	for _, target := range eligibleTargets {
		waitGroup.Add(1)
		semaphore <- struct{}{}
		go func(item keymanagement.HealthCheckTarget) {
			defer waitGroup.Done()
			defer func() { <-semaphore }()
			if h.runtime.CheckKey != nil {
				if checkErr := h.runtime.CheckKey(r.Context(), item.ID, item.URL); checkErr != nil {
					log.Printf("CheckAllKeys key_id=%d: %v", item.ID, checkErr)
				}
			}
		}(target)
	}
	waitGroup.Wait()

	results, err := h.service.ListHealthCheckResults(r.Context())
	if err != nil {
		h.finishJob(jobID, err)
		WriteError(w, r, http.StatusInternalServerError, "failed to load key checks")
		return
	}

	payloads := make([]map[string]any, 0, len(targets))
	for _, result := range results {
		payloads = append(payloads, healthCheckPayload(result))
	}
	h.finishJob(jobID, nil)
	if h.audit != nil {
		h.audit(r, "keys.health_check", "key", "all", map[string]any{"checked": len(eligibleTargets)})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"checked": len(eligibleTargets), "keys": payloads})
}

func (h *KeyAdministrationHandler) QueueHealthCheck(w http.ResponseWriter, r *http.Request) {
	jobID := int64(0)
	if h.runtime.QueueHealthJob != nil {
		jobID = h.runtime.QueueHealthJob(r)
	}
	if jobID == 0 {
		WriteV1Error(w, r, http.StatusInternalServerError, "job_queue_failed", "failed to queue key health check")
		return
	}
	WriteJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
}

func (h *KeyAdministrationHandler) respondKeyCheck(w http.ResponseWriter, r *http.Request, keyID int64) {
	result, err := h.service.GetHealthCheckResult(r.Context(), keyID)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "failed to load key check")
		return
	}
	WriteJSON(w, http.StatusOK, healthCheckPayload(result))
}

func (h *KeyAdministrationHandler) finishJob(jobID int64, err error) {
	if h.runtime.FinishJob != nil {
		h.runtime.FinishJob(jobID, err)
	}
}

func healthCheckPayload(result keymanagement.HealthCheckResult) map[string]any {
	payload := map[string]any{
		"id":                 result.ID,
		"check_status":       result.Status,
		"check_status_label": model.CheckStatusLabel(result.Status),
		"check_error":        result.Error,
		"last_checked_at":    "",
		"last_latency_ms":    result.Latency,
	}
	if result.LastCheckedAt != nil {
		payload["last_checked_at"] = result.LastCheckedAt.Local().Format("2006-01-02 15:04:05")
	}
	return payload
}
