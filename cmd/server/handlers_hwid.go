package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) apiUpdateUserHWID(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}

	var req model.UpdateHWIDRequest
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MaxDevices < 0 || req.MaxDevices > 32 {
		httpapi.WriteError(w, r, http.StatusBadRequest, "max_devices must be between 0 and 32 (0 means unlimited)")
		return
	}

	if _, err := a.db.Exec(`UPDATE users SET max_devices = ? WHERE id = ?`, req.MaxDevices, id); err != nil {
		log.Printf("apiUpdateUserHWID: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update hwid settings")
		return
	}
	a.recordAuditEvent(r, "user.hwid.update", "user", strconv.FormatInt(id, 10), map[string]any{"max_devices": req.MaxDevices})
	httpapi.WriteMessage(w, "hwid settings updated")
}

func (a *App) apiDeleteUserHWID(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	hwid := r.PathValue("hwid")
	if strings.TrimSpace(hwid) == "" {
		httpapi.WriteError(w, r, http.StatusBadRequest, "hwid is required")
		return
	}

	res, err := a.db.Exec(`DELETE FROM user_devices WHERE user_id = ? AND hwid = ?`, id, hwid)
	if err != nil {
		log.Printf("apiDeleteUserHWID: %v", err)
		httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to delete hwid")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		httpapi.WriteError(w, r, http.StatusNotFound, "hwid not found")
		return
	}
	a.recordAuditEvent(r, "user.hwid.delete", "user", strconv.FormatInt(id, 10), nil)
	httpapi.WriteMessage(w, "hwid removed")
}
