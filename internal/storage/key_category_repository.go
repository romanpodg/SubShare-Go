package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (r *Repository) EnsureKeyCategory(ctx context.Context, name string) (int64, error) {
	return r.categories.ensure(ctx, name, "#d8b33d")
}

var categoryHexColorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func normalizeCategoryColor(raw string) string {
	val := strings.TrimSpace(raw)
	if val == "" {
		return "#D8B33D"
	}
	if categoryHexColorRegex.MatchString(val) {
		return strings.ToUpper(val)
	}
	return "#D8B33D"
}

func normalizeCategoryName(raw string) string {
	val := strings.TrimSpace(raw)
	if len(val) > 24 {
		val = val[:24]
	}
	return val
}

func (r *Repository) ListKeyCategories(ctx context.Context) ([]model.KeyCategory, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, color FROM key_categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("failed to load key categories: %w", err)
	}
	defer rows.Close()

	countByName := make(map[string]int)
	colorByName := make(map[string]string)
	categories := make([]model.KeyCategory, 0, 16)
	for rows.Next() {
		var id int64
		var name sql.NullString
		var color sql.NullString
		if err := rows.Scan(&id, &name, &color); err != nil {
			return nil, fmt.Errorf("failed to scan key category: %w", err)
		}
		normalized := normalizeCategoryName(name.String)
		if normalized == "" {
			continue
		}
		if _, exists := countByName[normalized]; exists {
			continue
		}
		countByName[normalized] = 0
		colorByName[normalized] = normalizeCategoryColor(color.String)
		categories = append(categories, model.KeyCategory{ID: id, Name: normalized, Color: colorByName[normalized], KeysCount: 0})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read key categories: %w", err)
	}

	countRows, err := r.db.QueryContext(ctx, `
		SELECT kc.name, COUNT(*)
		FROM vless_keys k
		JOIN key_categories kc ON kc.id = k.category_id
		GROUP BY k.category_id, kc.name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to load key category counts: %w", err)
	}
	defer countRows.Close()

	for countRows.Next() {
		var category sql.NullString
		var count int64
		if err := countRows.Scan(&category, &count); err != nil {
			return nil, fmt.Errorf("failed to scan category count: %w", err)
		}
		normalized := normalizeCategoryName(category.String)
		if normalized == "" {
			continue
		}
		if _, exists := countByName[normalized]; !exists {
			colorByName[normalized] = "#D8B33D"
			var id int64
			_ = r.db.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, normalized).Scan(&id)
			categories = append(categories, model.KeyCategory{ID: id, Name: normalized, Color: colorByName[normalized], KeysCount: 0})
		}
		countByName[normalized] += int(count)
	}
	if err := countRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read category counts: %w", err)
	}

	for index := range categories {
		categories[index].KeysCount = countByName[categories[index].Name]
		categories[index].Color = normalizeCategoryColor(colorByName[categories[index].Name])
	}
	return categories, nil
}

func (r *Repository) GetCategoryColor(ctx context.Context, name string) (string, error) {
	var currentColor sql.NullString
	_ = r.db.QueryRowContext(ctx, `SELECT color FROM key_categories WHERE name = ?`, normalizeCategoryName(name)).Scan(&currentColor)
	return currentColor.String, nil
}

func (r *Repository) CreateKeyCategory(ctx context.Context, params keymanagement.CreateCategoryParams) (model.KeyCategory, error) {
	name := normalizeCategoryName(params.Name)
	color := params.Color

	var nextSortOrder int64
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextSortOrder); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to prepare category order: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO key_categories(name, color, sort_order, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET color = excluded.color, updated_at = CURRENT_TIMESTAMP
	`, name, color, nextSortOrder); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to create key category: %w", err)
	}

	var catID int64
	_ = r.db.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, name).Scan(&catID)

	return model.KeyCategory{
		ID:    catID,
		Name:  name,
		Color: color,
	}, nil
}

func (r *Repository) UpdateKeyCategory(ctx context.Context, params keymanagement.UpdateCategoryParams) (model.KeyCategory, error) {
	oldName := normalizeCategoryName(params.OldName)
	newName := normalizeCategoryName(params.NewName)
	color := params.Color

	var keyCount int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM vless_keys
		 WHERE category_id = (SELECT id FROM key_categories WHERE name = ?)
		    OR (category_id IS NULL AND category = ?)
	`, oldName, oldName).Scan(&keyCount); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to count category keys: %w", err)
	}

	var categoryCount int64
	var existingSortOrder int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM key_categories WHERE name = ?`, oldName).Scan(&categoryCount); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to count category: %w", err)
	}
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(sort_order, 0) FROM key_categories WHERE name = ?`, oldName).Scan(&existingSortOrder)

	if keyCount == 0 && categoryCount == 0 {
		return model.KeyCategory{}, keymanagement.ErrCategoryNotFound
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO key_categories(name, color, sort_order, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET color = excluded.color, sort_order = COALESCE(NULLIF(key_categories.sort_order, 0), excluded.sort_order), updated_at = CURRENT_TIMESTAMP
	`, newName, color, existingSortOrder); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to upsert key category: %w", err)
	}

	if oldName != newName {
		var newCategoryID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, newName).Scan(&newCategoryID); err != nil {
			return model.KeyCategory{}, fmt.Errorf("failed to resolve key category ID: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE vless_keys
			   SET category_id = ?, category = ?
			 WHERE category_id = (SELECT id FROM key_categories WHERE name = ?)
			    OR (category_id IS NULL AND category = ?)
		`, newCategoryID, newName, oldName, oldName); err != nil {
			return model.KeyCategory{}, fmt.Errorf("failed to update key category references: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE external_subscription_sources
			   SET key_category_id = ?, key_category = ?
			 WHERE key_category_id = (SELECT id FROM key_categories WHERE name = ?)
			    OR (key_category_id IS NULL AND key_category = ?)
		`, newCategoryID, newName, oldName, oldName); err != nil {
			return model.KeyCategory{}, fmt.Errorf("failed to update source category references: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM key_categories WHERE name = ?`, oldName); err != nil {
			return model.KeyCategory{}, fmt.Errorf("failed to delete old category: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE key_categories SET color = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`, color, newName); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to update category color: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.KeyCategory{}, fmt.Errorf("failed to commit category update: %w", err)
	}

	var catID int64
	_ = r.db.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, newName).Scan(&catID)

	return model.KeyCategory{
		ID:    catID,
		Name:  newName,
		Color: color,
	}, nil
}

func (r *Repository) DeleteKeyCategory(ctx context.Context, params keymanagement.DeleteCategoryParams) error {
	name := normalizeCategoryName(params.Name)
	mode := strings.TrimSpace(params.Mode)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	var categoryID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, name).Scan(&categoryID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to resolve key category: %w", err)
	}

	if mode == "delete_with_keys" {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM vless_keys
			 WHERE category_id = ? OR (category_id IS NULL AND category = ?)
		`, categoryID, name); err != nil {
			return fmt.Errorf("failed to delete keys in category: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE vless_keys SET category_id = NULL, category = ''
			 WHERE category_id = ? OR (category_id IS NULL AND category = ?)
		`, categoryID, name); err != nil {
			return fmt.Errorf("failed to clear keys category: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM key_categories WHERE name = ?`, name); err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	return tx.Commit()
}
