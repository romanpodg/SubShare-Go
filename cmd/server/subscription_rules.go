package main

import (
	"encoding/base64"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type subscriptionTemplate struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Format    string    `json:"format"`
	Content   string    `json:"content"`
	Enabled   bool      `json:"enabled"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type responseRuleCondition struct {
	HeaderName    string `json:"headerName"`
	Operator      string `json:"operator"`
	Value         string `json:"value"`
	CaseSensitive bool   `json:"caseSensitive"`
}

type responseHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type responseRule struct {
	ID           int64                   `json:"id"`
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	Enabled      bool                    `json:"enabled"`
	Priority     int                     `json:"priority"`
	Operator     string                  `json:"operator"`
	Conditions   []responseRuleCondition `json:"conditions"`
	ResponseType string                  `json:"response_type"`
	TemplateID   *int64                  `json:"template_id"`
	Headers      []responseHeader        `json:"headers"`
	IsSystem     bool                    `json:"is_system"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
}

type templateInput struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	Content string `json:"content"`
	Enabled bool   `json:"enabled"`
}

type responseRuleInput struct {
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	Enabled      bool                    `json:"enabled"`
	Priority     int                     `json:"priority"`
	Operator     string                  `json:"operator"`
	Conditions   []responseRuleCondition `json:"conditions"`
	ResponseType string                  `json:"response_type"`
	TemplateID   *int64                  `json:"template_id"`
	Headers      []responseHeader        `json:"headers"`
}

func (a *App) matchSubscriptionResponseRule(r *http.Request) (*responseRule, error) {
	rules, err := a.listResponseRules()
	if err != nil {
		return nil, err
	}
	return firstMatchingResponseRule(rules, r.Header), nil
}

func (a *App) apiV1ListTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := a.listSubscriptionTemplates()
	if err != nil {
		httpapi.WriteV1Error(w, r, http.StatusInternalServerError, "templates_list_failed", "failed to load templates")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"data": items})
}

func renderTemplatePreview(input templateInput) (string, string, error) {
	const sampleVLESS = "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&sni=edge.example.com&type=ws&path=%2Fws#Example"
	body := sampleVLESS
	contentType := "text/plain; charset=utf-8"
	var err error
	switch input.Format {
	case "xray-json":
		body = `[{"outbounds":[{"protocol":"vless","tag":"Example"}]}]`
		contentType = "application/json; charset=utf-8"
	case "mihomo":
		body, err = renderMihomoSubscription(sampleVLESS)
		contentType = "application/yaml; charset=utf-8"
	case "sing-box":
		body, err = renderSingBoxSubscription(sampleVLESS)
		contentType = "application/json; charset=utf-8"
	}
	if err != nil {
		return "", "", err
	}
	body = applyTemplateContent(input.Content, body, "SubShare Preview")
	if input.Format == "base64" {
		body = base64.StdEncoding.EncodeToString([]byte(body))
	}
	if err := delivery.ValidateStructuredBody(input.Format, body); err != nil {
		return "", "", fmt.Errorf("rendered preview is not valid %s", input.Format)
	}
	return body, contentType, nil
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

func applyTemplateContent(content, subscriptionBody, title string) string {
	if strings.TrimSpace(content) == "" {
		return subscriptionBody
	}
	result := strings.ReplaceAll(content, "{{subscription}}", subscriptionBody)
	result = strings.ReplaceAll(result, "{{title}}", title)
	return result
}

func renderMihomoSubscription(raw string) (string, error) {
	generated, err := delivery.RenderMihomo(delivery.SyntheticEntries(raw))
	if err != nil {
		return "", err
	}
	return generated.Body, nil
}

func renderSingBoxSubscription(raw string) (string, error) {
	generated, err := delivery.RenderSingBox(delivery.SyntheticEntries(raw))
	if err != nil {
		return "", err
	}
	return generated.Body, nil
}

func applyRuleHeaders(w http.ResponseWriter, headers []responseHeader) {
	for _, header := range headers {
		if safeCustomResponseHeader(header.Key, header.Value) {
			w.Header().Set(header.Key, header.Value)
		}
	}
}

func (a *App) incrementSubscriptionMetric(responseType, resultStatus string) {
	responseType = strings.TrimSpace(responseType)
	resultStatus = strings.TrimSpace(resultStatus)
	if responseType == "" {
		responseType = "unknown"
	}
	if resultStatus == "" {
		resultStatus = "unknown"
	}
	_, _ = a.db.Exec(`
		INSERT INTO subscription_metrics(day, response_type, result_status, requests)
		VALUES(date('now'), ?, ?, 1)
		ON CONFLICT(day, response_type, result_status)
		DO UPDATE SET requests = requests + 1
	`, responseType, resultStatus)
}
