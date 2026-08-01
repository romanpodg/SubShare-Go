package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/middleware"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

type pageMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func parsePageParams(r *http.Request) (int, int) {
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

func paginate[T any](items []T, page, pageSize int) ([]T, pageMeta) {
	total := len(items)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []T{}, pageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return items[start:end], pageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages}
}

func writeV1Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID, _ := r.Context().Value(middleware.CtxKeyRequestID).(string)
	writeJSON(w, status, map[string]any{
		"error":        message,
		"code":         code,
		"message":      message,
		"field_errors": map[string][]string{},
		"request_id":   requestID,
	})
}

func (a *App) apiV1BuildInfo(w http.ResponseWriter, _ *http.Request) {
	var schemaVersion int
	_ = a.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&schemaVersion)
	writeJSON(w, http.StatusOK, map[string]any{
		"version":        version,
		"commit":         commit,
		"build_time":     buildTime,
		"schema_version": schemaVersion,
	})
}

func (a *App) apiV1Dashboard(w http.ResponseWriter, r *http.Request) {
	var userTotal, userActive, userExpired, userPaused, userBlocked, userLimited int
	if err := a.db.QueryRow(`
		WITH device_counts AS (
			SELECT user_id, COUNT(*) AS connected_devices
			FROM user_devices
			GROUP BY user_id
		),
		effective_users AS (
			SELECT CASE
				WHEN LOWER(TRIM(COALESCE(u.status, 'active'))) = 'blocked' THEN 'blocked'
				WHEN LOWER(TRIM(COALESCE(u.status, 'active'))) = 'paused' THEN 'paused'
				WHEN u.expires_at IS NOT NULL AND datetime(u.expires_at) < CURRENT_TIMESTAMP THEN 'expired'
				WHEN COALESCE(d.connected_devices, 0) >= COALESCE(NULLIF(u.max_devices, 0), 1) THEN 'limited'
				ELSE 'active'
			END AS effective_status
			FROM users u
			LEFT JOIN device_counts d ON d.user_id = u.id
		)
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN effective_status = 'active' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN effective_status = 'expired' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN effective_status = 'paused' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN effective_status = 'blocked' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN effective_status = 'limited' THEN 1 ELSE 0 END), 0)
		FROM effective_users
	`).Scan(&userTotal, &userActive, &userExpired, &userPaused, &userBlocked, &userLimited); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "dashboard_users_failed", "failed to load dashboard users")
		return
	}
	userCounts := map[string]int{
		"total": userTotal, "active": userActive, "expired": userExpired,
		"paused": userPaused, "blocked": userBlocked, "limited": userLimited,
	}

	var keyTotal, keyUp, keyDown, keyUnknown int
	if err := a.db.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN check_status = 'up' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN check_status = 'down' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN check_status IS NULL OR check_status = '' OR check_status = 'unknown' THEN 1 ELSE 0 END), 0)
		FROM vless_keys
	`).Scan(&keyTotal, &keyUp, &keyDown, &keyUnknown); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "dashboard_keys_failed", "failed to load dashboard keys")
		return
	}

	degradedSections := make([]string, 0, 3)
	var sourceTotal, sourceErrors, devices int
	if err := a.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN import_status = 'error' THEN 1 ELSE 0 END), 0) FROM external_subscription_sources`).Scan(&sourceTotal, &sourceErrors); err != nil {
		degradedSections = append(degradedSections, "sources")
	}
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM user_devices`).Scan(&devices); err != nil {
		degradedSections = append(degradedSections, "devices")
	}

	recent, err := a.listAuditEvents(6, 0)
	if err != nil {
		recent = []auditEvent{}
		degradedSections = append(degradedSections, "audit")
	}
	backup := map[string]any{"enabled": false, "status": "disabled", "file": "", "last_modified": "", "size_bytes": int64(0)}
	if backupPath := strings.TrimSpace(os.Getenv("BACKUP_PATH")); backupPath != "" {
		backup["enabled"] = true
		backup["file"] = filepath.Base(backupPath)
		backup["status"] = "pending"
		if info, statErr := os.Stat(backupPath); statErr == nil {
			backup["status"] = "healthy"
			backup["last_modified"] = info.ModTime().UTC().Format(time.RFC3339)
			backup["size_bytes"] = info.Size()
		} else if !os.IsNotExist(statErr) {
			backup["status"] = "error"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": userCounts,
		"keys": map[string]int{
			"total": keyTotal, "up": keyUp, "down": keyDown, "unknown": keyUnknown,
		},
		"sources":             map[string]int{"total": sourceTotal, "errors": sourceErrors},
		"devices":             devices,
		"backup":              backup,
		"recent_audit_events": recent,
		"degraded_sections":   degradedSections,
	})
}

type userSummary struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	Email                string    `json:"email"`
	Status               string    `json:"status"`
	EffectiveStatus      string    `json:"effective_status"`
	StartsAt             string    `json:"starts_at"`
	ExpiresAt            string    `json:"expires_at"`
	MaxDevices           int       `json:"max_devices"`
	ConnectedDeviceCount int       `json:"connected_device_count"`
	CreatedAt            time.Time `json:"created_at"`
}

func (a *App) apiV1ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.listUsers()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "users_list_failed", "failed to load users")
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	items := make([]userSummary, 0, len(users))
	for _, user := range users {
		if query != "" && !strings.Contains(strings.ToLower(user.Name+" "+user.Email), query) {
			continue
		}
		if status != "" && status != "all" && user.EffectiveStatus != status && user.Status != status {
			continue
		}
		items = append(items, userSummary{
			ID: user.ID, Name: user.Name, Email: user.Email, Status: user.Status,
			EffectiveStatus: user.EffectiveStatus, StartsAt: user.StartsAtInput,
			ExpiresAt: user.ExpiresAtInput, MaxDevices: user.MaxDevices,
			ConnectedDeviceCount: user.ConnectedDeviceCount, CreatedAt: user.CreatedAt,
		})
	}
	switch r.URL.Query().Get("sort") {
	case "name_asc":
		sort.SliceStable(items, func(i, j int) bool { return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name) })
	case "expires_asc":
		sort.SliceStable(items, func(i, j int) bool { return items[i].ExpiresAt < items[j].ExpiresAt })
	default:
		sort.SliceStable(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	}
	page, pageSize := parsePageParams(r)
	data, meta := paginate(items, page, pageSize)
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "meta": meta})
}

func (a *App) apiV1GetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	users, err := a.listUsers()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "user_load_failed", "failed to load user")
		return
	}
	for _, user := range users {
		if user.ID == id {
			user.Token = ""
			writeJSON(w, http.StatusOK, map[string]any{"data": user})
			return
		}
	}
	writeV1Error(w, r, http.StatusNotFound, "user_not_found", "user not found")
}

type keySummary struct {
	ID                 int64     `json:"id"`
	Label              string    `json:"label"`
	CategoryID         int64     `json:"category_id"`
	Category           string    `json:"category"`
	Kind               string    `json:"kind"`
	Status             string    `json:"status"`
	CheckStatus        string    `json:"check_status"`
	CheckError         string    `json:"check_error"`
	LastLatencyMS      int64     `json:"last_latency_ms"`
	LastCheckedAt      string    `json:"last_checked_at"`
	ExternalSourceID   int64     `json:"external_source_id"`
	ExternalSourceName string    `json:"external_source_name"`
	CreatedAt          time.Time `json:"created_at"`
}

func (a *App) apiV1ListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := a.listKeys()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "keys_list_failed", "failed to load keys")
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	items := make([]keySummary, 0, len(keys))
	for _, key := range keys {
		if query != "" && !strings.Contains(strings.ToLower(key.Label+" "+key.Category+" "+key.ExternalSourceName), query) {
			continue
		}
		if status != "" && status != "all" && key.Status != status && key.CheckStatus != status {
			continue
		}
		items = append(items, keySummary{
			ID: key.ID, Label: key.Label, CategoryID: key.CategoryID, Category: key.Category, Kind: key.Kind,
			Status: key.Status, CheckStatus: key.CheckStatus, CheckError: key.CheckError,
			LastLatencyMS: key.LastLatencyMS, LastCheckedAt: key.LastCheckedAtText,
			ExternalSourceID: key.ExternalSourceID, ExternalSourceName: key.ExternalSourceName,
			CreatedAt: key.CreatedAt,
		})
	}
	page, pageSize := parsePageParams(r)
	data, meta := paginate(items, page, pageSize)
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "meta": meta})
}

type auditEvent struct {
	ID           int64           `json:"id"`
	ActorAdminID sql.NullInt64   `json:"-"`
	Actor        string          `json:"actor"`
	Action       string          `json:"action"`
	TargetType   string          `json:"target_type"`
	TargetID     string          `json:"target_id"`
	Metadata     json.RawMessage `json:"metadata"`
	RequestID    string          `json:"request_id"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (a *App) listAuditEvents(limit, offset int) ([]auditEvent, error) {
	rows, err := a.db.Query(`
		SELECT ae.id, ae.actor_admin_id, COALESCE(ad.username, 'system'), ae.action,
		       ae.target_type, ae.target_id, ae.metadata_json, ae.request_id, ae.created_at
		FROM audit_events ae
		LEFT JOIN admins ad ON ad.id = ae.actor_admin_id
		ORDER BY ae.id DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]auditEvent, 0, limit)
	for rows.Next() {
		var event auditEvent
		var metadata string
		if err := rows.Scan(&event.ID, &event.ActorAdminID, &event.Actor, &event.Action, &event.TargetType, &event.TargetID, &metadata, &event.RequestID, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.Metadata = json.RawMessage(metadata)
		out = append(out, event)
	}
	return out, rows.Err()
}

func (a *App) apiV1ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	var total int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&total); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "audit_list_failed", "failed to load audit events")
		return
	}
	events, err := a.listAuditEvents(pageSize, (page-1)*pageSize)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "audit_list_failed", "failed to load audit events")
		return
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": events,
		"meta": pageMeta{Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages},
	})
}

func (a *App) recordAuditEvent(r *http.Request, action, targetType, targetID string, metadata map[string]any) {
	session, _, ok := a.adminSessionFromRequest(r)
	if !ok {
		return
	}
	a.recordAuditEventForActor(session.AdminID, requestIDFromRequest(r), action, targetType, targetID, metadata)
}

func requestIDFromRequest(r *http.Request) string {
	requestID, _ := r.Context().Value(middleware.CtxKeyRequestID).(string)
	return requestID
}

func (a *App) recordAuditEventForActor(actorAdminID int64, requestID, action, targetType, targetID string, metadata map[string]any) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		payload = []byte("{}")
	}
	_, _ = a.db.Exec(`
		INSERT INTO audit_events(actor_admin_id, action, target_type, target_id, metadata_json, request_id)
		VALUES(?, ?, ?, ?, ?, ?)
	`, actorAdminID, action, targetType, targetID, string(payload), requestID)
}
