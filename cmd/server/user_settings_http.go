package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) apiUpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	var input model.UpdateUserSettingsRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}
	next, err := a.updateUserSettings(id, input)
	if err != nil {
		writeUserSettingsError(w, r, err)
		return
	}
	a.recordAuditEvent(r, "user.settings.update", "user", strconv.FormatInt(id, 10), map[string]any{
		"time_zone": next.timeZone, "language": next.language,
	})
	httpapi.WriteMessage(w, "user settings updated")
}

func writeUserSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *userSettingsValidationError
	if errors.As(err, &invalid) {
		httpapi.WriteError(w, r, http.StatusBadRequest, invalid.Error())
		return
	}
	if errors.Is(err, errUserSettingsNotFound) {
		httpapi.WriteError(w, r, http.StatusNotFound, "user not found")
		return
	}
	log.Printf("apiUpdateUserSettings: %v", err)
	httpapi.WriteError(w, r, http.StatusInternalServerError, "failed to update user settings")
}
