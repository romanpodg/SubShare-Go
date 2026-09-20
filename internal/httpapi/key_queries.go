package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
)

type KeyQueryHandler struct {
	service *keymanagement.Service
}

func NewKeyQueryHandler(service *keymanagement.Service) *KeyQueryHandler {
	return &KeyQueryHandler{service: service}
}

type keySummary struct {
	ID                   int64     `json:"id"`
	Label                string    `json:"label"`
	ClientDisplayName    string    `json:"client_display_name"`
	CategoryID           int64     `json:"category_id"`
	Category             string    `json:"category"`
	Kind                 string    `json:"kind"`
	TemplateText         string    `json:"template_text"`
	Status               string    `json:"status"`
	CheckStatus          string    `json:"check_status"`
	CheckError           string    `json:"check_error"`
	LastLatencyMS        int64     `json:"last_latency_ms"`
	LastCheckedAt        string    `json:"last_checked_at"`
	ExternalSourceID     int64     `json:"external_source_id"`
	ExternalSourceName   string    `json:"external_source_name"`
	Protocol             string    `json:"protocol"`
	ProfileSchemaVersion int       `json:"profile_schema_version"`
	ProfileCompatibility string    `json:"profile_compatibility"`
	ProfileWarnings      []string  `json:"profile_warnings"`
	CreatedAt            time.Time `json:"created_at"`
}

type keyPageMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func (h *KeyQueryHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.service.ListLegacy(r.Context())
	if err != nil {
		WriteV1Error(w, r, http.StatusInternalServerError, "keys_list_failed", "failed to load keys")
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	items := make([]keySummary, 0, len(keys))
	for _, key := range keys {
		if query != "" && !strings.Contains(strings.ToLower(key.Label+" "+key.ClientDisplayName+" "+key.Category+" "+key.ExternalSourceName), query) {
			continue
		}
		if status != "" && status != "all" && key.Status != status && key.CheckStatus != status {
			continue
		}
		items = append(items, keySummary{
			ID: key.ID, Label: key.Label, ClientDisplayName: key.ClientDisplayName, CategoryID: key.CategoryID, Category: key.Category, Kind: key.Kind,
			TemplateText: key.TemplateText,
			Status:       key.Status, CheckStatus: key.CheckStatus, CheckError: key.CheckError,
			LastLatencyMS: key.LastLatencyMS, LastCheckedAt: key.LastCheckedAtText,
			ExternalSourceID: key.ExternalSourceID, ExternalSourceName: key.ExternalSourceName,
			Protocol: key.Protocol, ProfileSchemaVersion: key.ProfileSchemaVersion,
			ProfileCompatibility: key.ProfileCompatibility, ProfileWarnings: key.ProfileWarnings,
			CreatedAt: key.CreatedAt,
		})
	}
	page, pageSize := parseKeyPageParams(r)
	data, meta := paginateKeySummaries(items, page, pageSize)
	WriteJSON(w, http.StatusOK, map[string]any{"data": data, "meta": meta})
}

func (h *KeyQueryHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.service.ListCategories(r.Context())
	if err != nil {
		WriteV1Error(w, r, http.StatusInternalServerError, "key_categories_failed", "failed to load key categories")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"data": categories})
}

func parseKeyPageParams(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func paginateKeySummaries(items []keySummary, page, pageSize int) ([]keySummary, keyPageMeta) {
	total := len(items)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	meta := keyPageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
	start := (page - 1) * pageSize
	if start >= total {
		return []keySummary{}, meta
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return items[start:end], meta
}
