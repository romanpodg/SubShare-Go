package main

import (
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"net/http"
	"strconv"
)

func (a *App) apiV1ListResponseRules(w http.ResponseWriter, r *http.Request) {
	items, err := a.listResponseRules()
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "rules_list_failed", "failed to load response rules")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (a *App) saveResponseRule(w http.ResponseWriter, r *http.Request, id *int64) {
	var input responseRuleInput
	if err := httpapi.ReadJSON(r, &input); err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	input, err := validateResponseRuleInput(input)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, "rule_invalid", err.Error())
		return
	}
	if failure := a.responsePolicyStore().validateRuleTemplate(input); failure != nil {
		httpapi.WriteV1Error(w, r, http.StatusBadRequest, failure.Code, failure.Message)
		return
	}
	if id == nil {
		createdID, err := a.responsePolicyStore().createRule(input)
		if err != nil {
			httpapi.WriteV1Error(w, r, http.StatusConflict, "rule_create_failed", "failed to create response rule")
			return
		}
		a.recordAuditEvent(r, "response_rule.create", "response_rule", strconv.FormatInt(createdID, 10), map[string]any{"name": input.Name})
		httpapi.WriteJSON(w, http.StatusCreated, map[string]any{"id": createdID})
		return
	}
	affected, err := a.responsePolicyStore().updateRule(*id, input)
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusConflict, "rule_update_failed", "failed to update response rule")
		return
	}
	if affected == 0 {
		httpapi.WriteV1Error(w, r, http.StatusNotFound, "rule_not_found", "response rule not found")
		return
	}
	a.recordAuditEvent(r, "response_rule.update", "response_rule", strconv.FormatInt(*id, 10), map[string]any{"name": input.Name})
	httpapi.WriteMessage(w, "response rule updated")
}

func (a *App) apiV1CreateResponseRule(w http.ResponseWriter, r *http.Request) {
	a.saveResponseRule(w, r, nil)
}

func (a *App) apiV1UpdateResponseRule(w http.ResponseWriter, r *http.Request) {
	id, ok := httpapi.PathID(w, r, "id")
	if !ok {
		return
	}
	a.saveResponseRule(w, r, &id)
}

func (a *App) apiV1DeleteResponseRule(w http.ResponseWriter, r *http.Request) {
	a.serveResponseDeletion(w, r, responseDeletionHTTPCommand{
		remove: a.responsePolicyStore().deleteRule, errors: ruleDeletionErrors,
		action: "response_rule.delete", target: "response_rule", message: "response rule deleted",
	})
}
