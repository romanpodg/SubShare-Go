package main

import (
	"net/http"
	"strconv"

	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) apiV1PatchUserSubscription(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	var input model.PatchSubscriptionRequest
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}

	updated, err := a.patchUserSubscription(r.Context(), id, input)
	if err != nil {
		writeSubscriptionPatchError(w, r, err)
		return
	}
	a.recordAuditEvent(r, "user.subscription.update", "user", strconv.FormatInt(id, 10), map[string]any{"status": updated.status})
	httpapi.WriteMessage(w, "subscription updated")
}
