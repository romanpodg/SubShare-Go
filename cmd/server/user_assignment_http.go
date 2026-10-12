package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) apiV1UpdateUserKeyAssignment(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	var input model.UpdateKeyAssignmentRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	mode, err := prepareUserAssignmentRequest(input)
	if err != nil {
		writeUserAssignmentError(w, r, err)
		return
	}
	if err := a.updateUserKeyAssignment(id, mode, input.KeyIDs); err != nil {
		writeUserAssignmentError(w, r, err)
		return
	}
	a.recordAuditEvent(r, "user.keys.update", "user", strconv.FormatInt(id, 10), map[string]any{
		"mode": mode, "keys_count": len(input.KeyIDs),
	})
	httpapi.WriteMessage(w, "subscription key assignment updated")
}

func writeUserAssignmentError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *userAssignmentRequestError
	if errors.As(err, &invalid) {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, invalid.code, invalid.message)
		return
	}
	switch {
	case errors.Is(err, errAssignmentUserNotFound):
		httpapi.WriteV1Error(w, r, http.StatusNotFound, "user_not_found", "user not found")
	case errors.Is(err, errAssignmentKeyNotFound):
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "key_not_found", "one or more keys do not exist")
	default:
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "assignment_update_failed", "failed to update key assignment")
	}
}
