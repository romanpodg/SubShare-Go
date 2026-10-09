package main

import (
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"net/http"
	"strconv"
)

func (a *App) apiV1ListTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := a.listSubscriptionTemplates()
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "templates_list_failed", "failed to load templates")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (a *App) apiV1PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	var input templateInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	input, err := validateTemplateInput(input)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "template_invalid", err.Error())
		return
	}
	body, contentType, err := renderTemplatePreview(input)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusUnprocessableEntity, "template_render_invalid", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"content": body, "content_type": contentType})
}

func (a *App) apiV1CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var input templateInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	input, err := validateTemplateInput(input)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "template_invalid", err.Error())
		return
	}
	id, slug, err := a.responsePolicyStore().createTemplate(input)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusConflict, "template_create_failed", "failed to create template")
		return
	}
	a.recordAuditEvent(r, "template.create", "template", strconv.FormatInt(id, 10), map[string]any{"name": input.Name, "format": input.Format})
	httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "slug": slug})
}

func (a *App) apiV1UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	var input templateInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	input, err := validateTemplateInput(input)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "template_invalid", err.Error())
		return
	}
	affected, err := a.responsePolicyStore().updateTemplate(id, input)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusConflict, "template_update_failed", "failed to update template")
		return
	}
	if affected == 0 {
		httpapi.WriteV1Error(w, r, http.StatusNotFound, "template_not_found", "template not found")
		return
	}
	a.recordAuditEvent(r, "template.update", "template", strconv.FormatInt(id, 10), map[string]any{"name": input.Name, "format": input.Format})
	httpapi.WriteMessage(w, "template updated")
}

func (a *App) apiV1DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	a.serveResponseDeletion(w, r, responseDeletionHTTPCommand{
		remove: a.responsePolicyStore().deleteTemplate, errors: templateDeletionErrors,
		action: "template.delete", target: "template", message: "template deleted",
	})
}
