package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
)

func (store responsePolicyStore) listRules() ([]responseRule, error) {
	rows, err := store.db.Query(`
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
			_ = rows.Close()
			store.disableInvalidRule(item.ID, "conditions_json", err)
			return nil, fmt.Errorf("response rule %d has invalid conditions JSON: %w", item.ID, err)
		}
		if err := json.Unmarshal([]byte(headersJSON), &item.Headers); err != nil {
			_ = rows.Close()
			store.disableInvalidRule(item.ID, "headers_json", err)
			return nil, fmt.Errorf("response rule %d has invalid headers JSON: %w", item.ID, err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (store responsePolicyStore) disableInvalidRule(id int64, field string, decodeErr error) {
	_, _ = store.db.Exec(`UPDATE response_rules SET enabled = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	metadata, _ := json.Marshal(map[string]string{
		"field": field,
		"error": decodeErr.Error(),
	})
	_, _ = store.db.Exec(`
		INSERT INTO audit_events(action, target_type, target_id, metadata_json)
		VALUES('response_rule.disabled_invalid', 'response_rule', ?, ?)
	`, strconv.FormatInt(id, 10), string(metadata))
}

func (store responsePolicyStore) validateRuleTemplate(input responseRuleInput) *responseTemplatePolicyError {
	if input.TemplateID == nil {
		return nil
	}
	var enabled int
	var format string
	if err := store.db.QueryRow(`SELECT enabled, format FROM subscription_templates WHERE id = ?`, *input.TemplateID).Scan(&enabled, &format); err != nil {
		return &responseTemplatePolicyError{"template_invalid", "selected template does not exist or is disabled"}
	}
	return validateResponseTemplateReference(input.ResponseType, format, enabled != 0)
}

func responseRuleWriteValues(input responseRuleInput) []any {
	conditions, _ := json.Marshal(input.Conditions)
	headers, _ := json.Marshal(input.Headers)
	return []any{input.Name, input.Description, boolToInt(input.Enabled), input.Priority, input.Operator, string(conditions), input.ResponseType, input.TemplateID, string(headers)}
}

func (store responsePolicyStore) createRule(input responseRuleInput) (int64, error) {
	result, err := store.db.Exec(`
			INSERT INTO response_rules(name, description, enabled, priority, operator, conditions_json, response_type, template_id, headers_json)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, responseRuleWriteValues(input)...)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return id, nil
}

func (store responsePolicyStore) updateRule(id int64, input responseRuleInput) (int64, error) {
	values := append(responseRuleWriteValues(input), id)
	result, err := store.db.Exec(`
		UPDATE response_rules
		SET name = ?, description = ?, enabled = ?, priority = ?, operator = ?,
		    conditions_json = ?, response_type = ?, template_id = ?, headers_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, values...)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (store responsePolicyStore) ruleIsSystem(id int64) (bool, error) {
	var system int
	err := store.db.QueryRow(`SELECT is_system FROM response_rules WHERE id = ?`, id).Scan(&system)
	return system != 0, err
}

func (store responsePolicyStore) deleteRule(id int64) responseDeletionFailure {
	system, err := store.ruleIsSystem(id)
	if failure := responseDeletionGuard(system, err); failure != responseDeleteOK {
		return failure
	}
	if _, err := store.db.Exec(`DELETE FROM response_rules WHERE id = ?`, id); err != nil {
		return responseDeleteWriteFailed
	}
	return responseDeleteOK
}
