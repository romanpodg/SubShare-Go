package storage

import (
	"context"
	"fmt"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
)

func (r *Repository) ReorderKeyCategories(ctx context.Context, names []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE key_categories SET sort_order = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare reorder statement: %w", err)
	}
	defer stmt.Close()

	for index, name := range names {
		if _, err := stmt.ExecContext(ctx, index+1, name); err != nil {
			return fmt.Errorf("failed to update category order for %s: %w", name, err)
		}
	}

	return tx.Commit()
}

func (r *Repository) ReorderKeys(ctx context.Context, ids []int64) error {
	existingIDs, err := r.loadKeyOrderIDs(ctx)
	if err != nil {
		return err
	}
	if err := validateKeyOrder(existingIDs, ids); err != nil {
		return err
	}
	return r.saveKeyOrder(ctx, ids)
}

func (r *Repository) loadKeyOrderIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM vless_keys ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("failed to load keys for reorder: %w", err)
	}
	defer rows.Close()

	existingIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to read key id: %w", err)
		}
		existingIDs = append(existingIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate key ids: %w", err)
	}
	return existingIDs, nil
}

func validateKeyOrder(existingIDs, ids []int64) error {
	if len(existingIDs) != len(ids) {
		return keymanagement.ErrInvalidKeyOrderCount
	}

	allowed := make(map[int64]struct{}, len(existingIDs))
	for _, id := range existingIDs {
		allowed[id] = struct{}{}
	}
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := allowed[id]; !ok {
			return keymanagement.ErrUnknownKeyInOrder
		}
		if _, ok := seen[id]; ok {
			return keymanagement.ErrDuplicateKeyInOrder
		}
		seen[id] = struct{}{}
	}
	return nil
}

func (r *Repository) saveKeyOrder(ctx context.Context, ids []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `UPDATE vless_keys SET sort_order = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare key reorder statement: %w", err)
	}
	defer stmt.Close()

	for index, id := range ids {
		if _, err := stmt.ExecContext(ctx, index+1, id); err != nil {
			return fmt.Errorf("failed to update key sort order: %w", err)
		}
	}

	return tx.Commit()
}
