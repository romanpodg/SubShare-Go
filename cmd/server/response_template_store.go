package main

import "strconv"

func (store responsePolicyStore) listTemplates() ([]subscriptionTemplate, error) {
	rows, err := store.db.Query(`
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

func (store responsePolicyStore) loadTemplate(id *int64) (*subscriptionTemplate, error) {
	if id == nil {
		return nil, nil
	}
	var item subscriptionTemplate
	var enabled, system int
	err := store.db.QueryRow(`
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

func (store responsePolicyStore) createTemplate(input templateInput) (int64, string, error) {
	slug := templateSlug(input.Name)
	for suffix := 2; ; suffix++ {
		var exists int
		_ = store.db.QueryRow(`SELECT COUNT(*) FROM subscription_templates WHERE slug = ?`, slug).Scan(&exists)
		if exists == 0 {
			break
		}
		slug = templateSlug(input.Name) + "-" + strconv.Itoa(suffix)
	}
	result, err := store.db.Exec(`
		INSERT INTO subscription_templates(slug, name, format, content, enabled)
		VALUES(?, ?, ?, ?, ?)
	`, slug, input.Name, input.Format, input.Content, boolToInt(input.Enabled))
	if err != nil {
		return 0, "", err
	}
	id, _ := result.LastInsertId()
	return id, slug, nil
}

func (store responsePolicyStore) updateTemplate(id int64, input templateInput) (int64, error) {
	result, err := store.db.Exec(`
		UPDATE subscription_templates
		SET name = ?, format = ?, content = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, input.Name, input.Format, input.Content, boolToInt(input.Enabled), id)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (store responsePolicyStore) templateIsSystem(id int64) (bool, error) {
	var system int
	err := store.db.QueryRow(`SELECT is_system FROM subscription_templates WHERE id = ?`, id).Scan(&system)
	return system != 0, err
}

func (store responsePolicyStore) deleteTemplate(id int64) responseDeletionFailure {
	system, err := store.templateIsSystem(id)
	if failure := responseDeletionGuard(system, err); failure != responseDeleteOK {
		return failure
	}
	if _, err := store.db.Exec(`DELETE FROM subscription_templates WHERE id = ?`, id); err != nil {
		return responseDeleteWriteFailed
	}
	return responseDeleteOK
}
