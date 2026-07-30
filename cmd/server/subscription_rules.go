package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"subshare/internal/model"
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

var templateSlugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeTemplateFormat(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "base64", "plain", "xray-json", "mihomo", "sing-box":
		return value, true
	default:
		return "", false
	}
}

func normalizeResponseType(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "browser", "base64", "plain", "xray-json", "mihomo", "sing-box", "block", "not-found":
		return value, true
	default:
		return "", false
	}
}

func templateSlug(name string) string {
	slug := templateSlugSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "template"
	}
	return slug
}

func validateTemplateInput(input templateInput) (templateInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return input, fmt.Errorf("name must contain 1..80 characters")
	}
	format, ok := normalizeTemplateFormat(input.Format)
	if !ok {
		return input, fmt.Errorf("unsupported template format")
	}
	input.Format = format
	if len(input.Content) > 1<<20 {
		return input, fmt.Errorf("template content is too large")
	}
	if (format == "mihomo" || format == "sing-box") && strings.TrimSpace(input.Content) != "" &&
		!strings.Contains(input.Content, "{{subscription}}") {
		return input, fmt.Errorf("custom template must contain {{subscription}}")
	}
	return input, nil
}

func validateResponseRuleInput(input responseRuleInput) (responseRuleInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return input, fmt.Errorf("name must contain 1..80 characters")
	}
	if len([]rune(input.Description)) > 250 {
		return input, fmt.Errorf("description is too long")
	}
	input.Operator = strings.ToUpper(strings.TrimSpace(input.Operator))
	if input.Operator != "AND" && input.Operator != "OR" {
		return input, fmt.Errorf("operator must be AND or OR")
	}
	responseType, ok := normalizeResponseType(input.ResponseType)
	if !ok {
		return input, fmt.Errorf("unsupported response type")
	}
	input.ResponseType = responseType
	if input.Priority < 0 || input.Priority > 100000 {
		return input, fmt.Errorf("priority must be between 0 and 100000")
	}
	if len(input.Conditions) > 20 {
		return input, fmt.Errorf("too many conditions")
	}
	for index := range input.Conditions {
		condition := &input.Conditions[index]
		condition.HeaderName = strings.ToLower(strings.TrimSpace(condition.HeaderName))
		condition.Operator = strings.ToUpper(strings.TrimSpace(condition.Operator))
		condition.Value = strings.TrimSpace(condition.Value)
		if condition.HeaderName == "" || len(condition.HeaderName) > 80 ||
			strings.ContainsAny(condition.HeaderName, "\r\n:") {
			return input, fmt.Errorf("invalid condition header name")
		}
		switch condition.Operator {
		case "EQUALS", "NOT_EQUALS", "CONTAINS", "NOT_CONTAINS", "STARTS_WITH", "NOT_STARTS_WITH", "ENDS_WITH", "NOT_ENDS_WITH", "REGEX", "NOT_REGEX":
		default:
			return input, fmt.Errorf("unsupported condition operator")
		}
		if condition.Value == "" || len(condition.Value) > 255 {
			return input, fmt.Errorf("condition value must contain 1..255 characters")
		}
		if condition.Operator == "REGEX" || condition.Operator == "NOT_REGEX" {
			if _, err := regexp.Compile(condition.Value); err != nil {
				return input, fmt.Errorf("invalid condition regex")
			}
		}
	}
	if len(input.Headers) > 30 {
		return input, fmt.Errorf("too many response headers")
	}
	for index := range input.Headers {
		header := &input.Headers[index]
		header.Key = http.CanonicalHeaderKey(strings.TrimSpace(header.Key))
		header.Value = strings.TrimSpace(header.Value)
		if !safeCustomResponseHeader(header.Key, header.Value) {
			return input, fmt.Errorf("unsafe response header %q", header.Key)
		}
	}
	return input, nil
}

func safeCustomResponseHeader(key, value string) bool {
	if key == "" || len(key) > 80 || len(value) > 1024 || strings.ContainsAny(key+value, "\r\n") {
		return false
	}
	switch strings.ToLower(key) {
	case "set-cookie", "content-length", "transfer-encoding", "connection", "content-type",
		"content-disposition", "strict-transport-security", "access-control-allow-origin":
		return false
	default:
		return true
	}
}

func ruleConditionMatches(condition responseRuleCondition, header http.Header) bool {
	actual := strings.Join(header.Values(condition.HeaderName), ",")
	expected := condition.Value
	if !condition.CaseSensitive {
		actual = strings.ToLower(actual)
		expected = strings.ToLower(expected)
	}
	switch condition.Operator {
	case "EQUALS":
		return actual == expected
	case "NOT_EQUALS":
		return actual != expected
	case "CONTAINS":
		return strings.Contains(actual, expected)
	case "NOT_CONTAINS":
		return !strings.Contains(actual, expected)
	case "STARTS_WITH":
		return strings.HasPrefix(actual, expected)
	case "NOT_STARTS_WITH":
		return !strings.HasPrefix(actual, expected)
	case "ENDS_WITH":
		return strings.HasSuffix(actual, expected)
	case "NOT_ENDS_WITH":
		return !strings.HasSuffix(actual, expected)
	case "REGEX", "NOT_REGEX":
		re, err := regexp.Compile(expected)
		if err != nil {
			return false
		}
		matched := re.MatchString(actual)
		if condition.Operator == "NOT_REGEX" {
			return !matched
		}
		return matched
	default:
		return false
	}
}

func responseRuleMatches(rule responseRule, header http.Header) bool {
	if len(rule.Conditions) == 0 {
		return true
	}
	if rule.Operator == "OR" {
		for _, condition := range rule.Conditions {
			if ruleConditionMatches(condition, header) {
				return true
			}
		}
		return false
	}
	for _, condition := range rule.Conditions {
		if !ruleConditionMatches(condition, header) {
			return false
		}
	}
	return true
}

func (a *App) listSubscriptionTemplates() ([]subscriptionTemplate, error) {
	rows, err := a.db.Query(`
		SELECT id, slug, name, format, content, enabled, is_system, created_at, updated_at
		FROM subscription_templates ORDER BY is_system DESC, name, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []subscriptionTemplate{}
	for rows.Next() {
		var item subscriptionTemplate
		var enabled, system int
		if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.Format, &item.Content, &enabled, &system, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.IsSystem = system != 0
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *App) listResponseRules() ([]responseRule, error) {
	rows, err := a.db.Query(`
		SELECT id, name, description, enabled, priority, operator, conditions_json,
		       response_type, template_id, headers_json, is_system, created_at, updated_at
		FROM response_rules ORDER BY priority, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []responseRule{}
	for rows.Next() {
		var item responseRule
		var enabled, system int
		var templateID sql.NullInt64
		var conditionsJSON, headersJSON string
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Description, &enabled, &item.Priority, &item.Operator,
			&conditionsJSON, &item.ResponseType, &templateID, &headersJSON, &system,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Enabled = enabled != 0
		item.IsSystem = system != 0
		if templateID.Valid {
			value := templateID.Int64
			item.TemplateID = &value
		}
		if err := json.Unmarshal([]byte(conditionsJSON), &item.Conditions); err != nil {
			a.disableInvalidResponseRule(item.ID, "conditions_json", err)
			return nil, fmt.Errorf("response rule %d has invalid conditions JSON: %w", item.ID, err)
		}
		if err := json.Unmarshal([]byte(headersJSON), &item.Headers); err != nil {
			a.disableInvalidResponseRule(item.ID, "headers_json", err)
			return nil, fmt.Errorf("response rule %d has invalid headers JSON: %w", item.ID, err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (a *App) disableInvalidResponseRule(id int64, field string, decodeErr error) {
	_, _ = a.db.Exec(`UPDATE response_rules SET enabled = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	metadata, _ := json.Marshal(map[string]string{
		"field": field,
		"error": decodeErr.Error(),
	})
	_, _ = a.db.Exec(`
		INSERT INTO audit_events(action, target_type, target_id, metadata_json)
		VALUES('response_rule.disabled_invalid', 'response_rule', ?, ?)
	`, strconv.FormatInt(id, 10), string(metadata))
}

func (a *App) matchSubscriptionResponseRule(r *http.Request) (*responseRule, error) {
	rules, err := a.listResponseRules()
	if err != nil {
		return nil, err
	}
	for index := range rules {
		if rules[index].Enabled && responseRuleMatches(rules[index], r.Header) {
			return &rules[index], nil
		}
	}
	return nil, nil
}

func (a *App) apiV1ListTemplates(w http.ResponseWriter, r *http.Request) {
	items, err := a.listSubscriptionTemplates()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "templates_list_failed", "failed to load templates")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func renderTemplatePreview(input templateInput) (string, string, error) {
	const sampleVLESS = "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&sni=edge.example.com&type=ws&path=%2Fws#Example"
	body := sampleVLESS
	contentType := "text/plain; charset=utf-8"
	var err error
	switch input.Format {
	case "xray-json":
		body = `{"outbounds":[{"protocol":"vless","tag":"Example"}]}`
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
	if (input.Format == "xray-json" || input.Format == "sing-box") && !json.Valid([]byte(body)) {
		return "", "", fmt.Errorf("rendered preview is not valid JSON")
	}
	return body, contentType, nil
}

func (a *App) apiV1PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	var input templateInput
	if err := readJSON(r, &input); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	input, err := validateTemplateInput(input)
	if err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "template_invalid", err.Error())
		return
	}
	body, contentType, err := renderTemplatePreview(input)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "template_render_invalid", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": body, "content_type": contentType})
}

func (a *App) apiV1CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var input templateInput
	if err := readJSON(r, &input); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	input, err := validateTemplateInput(input)
	if err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "template_invalid", err.Error())
		return
	}
	slug := templateSlug(input.Name)
	for suffix := 2; ; suffix++ {
		var exists int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM subscription_templates WHERE slug = ?`, slug).Scan(&exists)
		if exists == 0 {
			break
		}
		slug = templateSlug(input.Name) + "-" + strconv.Itoa(suffix)
	}
	result, err := a.db.Exec(`
		INSERT INTO subscription_templates(slug, name, format, content, enabled)
		VALUES(?, ?, ?, ?, ?)
	`, slug, input.Name, input.Format, input.Content, boolToInt(input.Enabled))
	if err != nil {
		writeV1Error(w, r, http.StatusConflict, "template_create_failed", "failed to create template")
		return
	}
	id, _ := result.LastInsertId()
	a.recordAuditEvent(r, "template.create", "template", strconv.FormatInt(id, 10), map[string]any{"name": input.Name, "format": input.Format})
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "slug": slug})
}

func (a *App) apiV1UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var input templateInput
	if err := readJSON(r, &input); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	input, err := validateTemplateInput(input)
	if err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "template_invalid", err.Error())
		return
	}
	result, err := a.db.Exec(`
		UPDATE subscription_templates
		SET name = ?, format = ?, content = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, input.Name, input.Format, input.Content, boolToInt(input.Enabled), id)
	if err != nil {
		writeV1Error(w, r, http.StatusConflict, "template_update_failed", "failed to update template")
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeV1Error(w, r, http.StatusNotFound, "template_not_found", "template not found")
		return
	}
	a.recordAuditEvent(r, "template.update", "template", strconv.FormatInt(id, 10), map[string]any{"name": input.Name, "format": input.Format})
	writeMessage(w, "template updated")
}

func (a *App) apiV1DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var system int
	err := a.db.QueryRow(`SELECT is_system FROM subscription_templates WHERE id = ?`, id).Scan(&system)
	if errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "template_not_found", "template not found")
		return
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "template_delete_failed", "failed to delete template")
		return
	}
	if system != 0 {
		writeV1Error(w, r, http.StatusConflict, "system_template", "system templates cannot be deleted")
		return
	}
	if _, err := a.db.Exec(`DELETE FROM subscription_templates WHERE id = ?`, id); err != nil {
		writeV1Error(w, r, http.StatusConflict, "template_in_use", "template is used by a response rule")
		return
	}
	a.recordAuditEvent(r, "template.delete", "template", strconv.FormatInt(id, 10), nil)
	writeMessage(w, "template deleted")
}

func (a *App) apiV1ListResponseRules(w http.ResponseWriter, r *http.Request) {
	items, err := a.listResponseRules()
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "rules_list_failed", "failed to load response rules")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items})
}

func (a *App) saveResponseRule(w http.ResponseWriter, r *http.Request, id *int64) {
	var input responseRuleInput
	if err := readJSON(r, &input); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_body", "invalid request body")
		return
	}
	input, err := validateResponseRuleInput(input)
	if err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "rule_invalid", err.Error())
		return
	}
	if input.TemplateID != nil {
		var enabled int
		var templateFormat string
		if err := a.db.QueryRow(`SELECT enabled, format FROM subscription_templates WHERE id = ?`, *input.TemplateID).Scan(&enabled, &templateFormat); err != nil || enabled == 0 {
			writeV1Error(w, r, http.StatusBadRequest, "template_invalid", "selected template does not exist or is disabled")
			return
		}
		if templateFormat != input.ResponseType {
			writeV1Error(w, r, http.StatusBadRequest, "template_format_mismatch", "selected template format must match the rule response type")
			return
		}
	} else if input.ResponseType == "browser" || input.ResponseType == "block" || input.ResponseType == "not-found" {
		input.TemplateID = nil
	}
	conditionsJSON, _ := json.Marshal(input.Conditions)
	headersJSON, _ := json.Marshal(input.Headers)
	if id == nil {
		result, err := a.db.Exec(`
			INSERT INTO response_rules(name, description, enabled, priority, operator, conditions_json, response_type, template_id, headers_json)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, input.Name, input.Description, boolToInt(input.Enabled), input.Priority, input.Operator, string(conditionsJSON), input.ResponseType, input.TemplateID, string(headersJSON))
		if err != nil {
			writeV1Error(w, r, http.StatusConflict, "rule_create_failed", "failed to create response rule")
			return
		}
		createdID, _ := result.LastInsertId()
		a.recordAuditEvent(r, "response_rule.create", "response_rule", strconv.FormatInt(createdID, 10), map[string]any{"name": input.Name})
		writeJSON(w, http.StatusCreated, map[string]any{"id": createdID})
		return
	}
	result, err := a.db.Exec(`
		UPDATE response_rules
		SET name = ?, description = ?, enabled = ?, priority = ?, operator = ?,
		    conditions_json = ?, response_type = ?, template_id = ?, headers_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, input.Name, input.Description, boolToInt(input.Enabled), input.Priority, input.Operator, string(conditionsJSON), input.ResponseType, input.TemplateID, string(headersJSON), *id)
	if err != nil {
		writeV1Error(w, r, http.StatusConflict, "rule_update_failed", "failed to update response rule")
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeV1Error(w, r, http.StatusNotFound, "rule_not_found", "response rule not found")
		return
	}
	a.recordAuditEvent(r, "response_rule.update", "response_rule", strconv.FormatInt(*id, 10), map[string]any{"name": input.Name})
	writeMessage(w, "response rule updated")
}

func (a *App) apiV1CreateResponseRule(w http.ResponseWriter, r *http.Request) {
	a.saveResponseRule(w, r, nil)
}

func (a *App) apiV1UpdateResponseRule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	a.saveResponseRule(w, r, &id)
}

func (a *App) apiV1DeleteResponseRule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var system int
	err := a.db.QueryRow(`SELECT is_system FROM response_rules WHERE id = ?`, id).Scan(&system)
	if errors.Is(err, sql.ErrNoRows) {
		writeV1Error(w, r, http.StatusNotFound, "rule_not_found", "response rule not found")
		return
	}
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "rule_delete_failed", "failed to delete response rule")
		return
	}
	if system != 0 {
		writeV1Error(w, r, http.StatusConflict, "system_rule", "system response rules cannot be deleted")
		return
	}
	if _, err := a.db.Exec(`DELETE FROM response_rules WHERE id = ?`, id); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "rule_delete_failed", "failed to delete response rule")
		return
	}
	a.recordAuditEvent(r, "response_rule.delete", "response_rule", strconv.FormatInt(id, 10), nil)
	writeMessage(w, "response rule deleted")
}

func (a *App) loadTemplate(id *int64) (*subscriptionTemplate, error) {
	if id == nil {
		return nil, nil
	}
	var item subscriptionTemplate
	var enabled, system int
	err := a.db.QueryRow(`
		SELECT id, slug, name, format, content, enabled, is_system, created_at, updated_at
		FROM subscription_templates WHERE id = ?
	`, *id).Scan(&item.ID, &item.Slug, &item.Name, &item.Format, &item.Content, &enabled, &system, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	item.Enabled = enabled != 0
	item.IsSystem = system != 0
	return &item, nil
}

func applyTemplateContent(content, subscriptionBody, title string) string {
	if strings.TrimSpace(content) == "" {
		return subscriptionBody
	}
	result := strings.ReplaceAll(content, "{{subscription}}", subscriptionBody)
	result = strings.ReplaceAll(result, "{{title}}", title)
	return result
}

func parseVLESSForClient(raw string, index int) (map[string]any, url.Values, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(parsed.Scheme, "vless") || parsed.User == nil || parsed.Hostname() == "" {
		return nil, nil, false
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		port = 443
	}
	name := strings.TrimSpace(parsed.Fragment)
	if name == "" {
		name = fmt.Sprintf("%s-%d", parsed.Hostname(), index+1)
	}
	return map[string]any{
		"name":   name,
		"server": parsed.Hostname(),
		"port":   port,
		"uuid":   parsed.User.Username(),
	}, parsed.Query(), true
}

func splitSubscriptionEntries(raw string) []string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	entries := make([]string, 0, len(lines))
	var jsonBuffer strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if jsonBuffer.Len() > 0 || strings.HasPrefix(trimmed, "{") {
			if jsonBuffer.Len() > 0 {
				jsonBuffer.WriteByte('\n')
			}
			jsonBuffer.WriteString(trimmed)
			if json.Valid([]byte(jsonBuffer.String())) {
				entries = append(entries, jsonBuffer.String())
				jsonBuffer.Reset()
			}
			continue
		}
		entries = append(entries, trimmed)
	}
	if jsonBuffer.Len() > 0 {
		entries = append(entries, jsonBuffer.String())
	}
	return entries
}

func subscriptionDrafts(raw string) ([]linkConfigurationDraft, error) {
	entries := splitSubscriptionEntries(raw)
	drafts := make([]linkConfigurationDraft, 0, len(entries))
	for _, entry := range entries {
		var parsed []linkConfigurationDraft
		var err error
		if supportedConfigScheme(entry) == model.SubscriptionFormatXrayJSON {
			parsed, err = parseXrayJSONDrafts(entry)
		} else {
			var draft linkConfigurationDraft
			draft, err = parseLinkConfiguration(entry)
			if err == nil {
				parsed = []linkConfigurationDraft{draft}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("parse subscription entry: %w", err)
		}
		drafts = append(drafts, parsed...)
	}
	if len(drafts) == 0 {
		return nil, fmt.Errorf("subscription contains no supported configurations")
	}
	return drafts, nil
}

func draftDisplayName(draft linkConfigurationDraft, index int) string {
	if name := strings.TrimSpace(draft.Remark); name != "" {
		return name
	}
	if name := strings.TrimSpace(draft.ServerDescription); name != "" {
		return name
	}
	return fmt.Sprintf("%s-%d", draft.Server, index+1)
}

func applyMihomoTransport(proxy map[string]any, draft linkConfigurationDraft) {
	network := strings.TrimSpace(draft.Network)
	if network == "" {
		network = "tcp"
	}
	proxy["network"] = network
	switch network {
	case "ws":
		options := map[string]any{}
		if draft.Path != "" {
			options["path"] = draft.Path
		}
		if draft.Host != "" {
			options["headers"] = map[string]string{"Host": draft.Host}
		}
		if len(options) > 0 {
			proxy["ws-opts"] = options
		}
	case "grpc":
		if draft.GRPCServiceName != "" {
			proxy["grpc-opts"] = map[string]string{"grpc-service-name": draft.GRPCServiceName}
		}
	}
}

func renderMihomoSubscription(raw string) (string, error) {
	drafts, err := subscriptionDrafts(raw)
	if err != nil {
		return "", err
	}
	proxies := make([]map[string]any, 0, len(drafts))
	for index, draft := range drafts {
		proxy := map[string]any{
			"name": draftDisplayName(draft, index), "type": draft.Protocol,
			"server": draft.Server, "port": draft.Port,
		}
		switch draft.Protocol {
		case "vless":
			proxy["uuid"] = draft.Identifier
			if draft.Flow != "" {
				proxy["flow"] = draft.Flow
			}
		case "vmess":
			proxy["uuid"] = draft.Identifier
			alterID, _ := strconv.Atoi(firstNonEmpty(draft.VMessAlterID, "0"))
			proxy["alterId"] = alterID
			proxy["cipher"] = firstNonEmpty(draft.VMessSecurity, "auto")
		case "trojan":
			proxy["password"] = draft.Identifier
		}
		proxy["udp"] = true
		security := strings.ToLower(strings.TrimSpace(draft.Security))
		if security == "tls" || security == "reality" {
			proxy["tls"] = true
			if draft.SNI != "" {
				proxy["servername"] = draft.SNI
			}
			if draft.Fingerprint != "" {
				proxy["client-fingerprint"] = draft.Fingerprint
			}
			proxy["skip-cert-verify"] = draft.AllowInsecure
		}
		if security == "reality" {
			reality := map[string]any{}
			if draft.PublicKey != "" {
				reality["public-key"] = draft.PublicKey
			}
			if draft.ShortID != "" {
				reality["short-id"] = draft.ShortID
			}
			if len(reality) > 0 {
				proxy["reality-opts"] = reality
			}
		}
		applyMihomoTransport(proxy, draft)
		proxies = append(proxies, proxy)
	}
	payload, err := json.MarshalIndent(map[string]any{"proxies": proxies}, "", "  ")
	return string(payload), err
}

func renderSingBoxSubscription(raw string) (string, error) {
	drafts, err := subscriptionDrafts(raw)
	if err != nil {
		return "", err
	}
	outbounds := make([]map[string]any, 0, len(drafts))
	for index, draft := range drafts {
		outbound := map[string]any{
			"type": draft.Protocol, "tag": draftDisplayName(draft, index),
			"server": draft.Server, "server_port": draft.Port,
		}
		switch draft.Protocol {
		case "vless":
			outbound["uuid"] = draft.Identifier
			if draft.Flow != "" {
				outbound["flow"] = draft.Flow
			}
		case "vmess":
			outbound["uuid"] = draft.Identifier
			outbound["security"] = firstNonEmpty(draft.VMessSecurity, "auto")
			if alterID, parseErr := strconv.Atoi(draft.VMessAlterID); parseErr == nil && alterID > 0 {
				outbound["alter_id"] = alterID
			}
		case "trojan":
			outbound["password"] = draft.Identifier
		}
		security := strings.ToLower(strings.TrimSpace(draft.Security))
		if security == "tls" || security == "reality" {
			tlsOptions := map[string]any{"enabled": true, "insecure": draft.AllowInsecure}
			if draft.SNI != "" {
				tlsOptions["server_name"] = draft.SNI
			}
			if draft.Fingerprint != "" {
				tlsOptions["utls"] = map[string]any{"enabled": true, "fingerprint": draft.Fingerprint}
			}
			if security == "reality" {
				reality := map[string]any{"enabled": true}
				if draft.PublicKey != "" {
					reality["public_key"] = draft.PublicKey
				}
				if draft.ShortID != "" {
					reality["short_id"] = draft.ShortID
				}
				tlsOptions["reality"] = reality
			}
			outbound["tls"] = tlsOptions
		}
		switch strings.TrimSpace(draft.Network) {
		case "ws":
			transport := map[string]any{"type": "ws"}
			if draft.Path != "" {
				transport["path"] = draft.Path
			}
			if draft.Host != "" {
				transport["headers"] = map[string]string{"Host": draft.Host}
			}
			outbound["transport"] = transport
		case "grpc":
			transport := map[string]any{"type": "grpc"}
			if draft.GRPCServiceName != "" {
				transport["service_name"] = draft.GRPCServiceName
			}
			outbound["transport"] = transport
		}
		outbounds = append(outbounds, outbound)
	}
	payload, err := json.MarshalIndent(map[string]any{"outbounds": outbounds}, "", "  ")
	return string(payload), err
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
