package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type categoryStore struct {
	db *sql.DB
}

func newCategoryStore(db *sql.DB) *categoryStore {
	return &categoryStore{db: db}
}

func (s *categoryStore) ensure(ctx context.Context, category string, color string) (int64, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		return 0, nil
	}
	var nextSortOrder int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextSortOrder); err != nil {
		return 0, fmt.Errorf("failed to query sort order for category: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO key_categories(name, color, sort_order, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
	`, category, color, nextSortOrder); err != nil {
		return 0, fmt.Errorf("failed to upsert category: %w", err)
	}
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, category).Scan(&id); err != nil {
		return 0, fmt.Errorf("failed to resolve key category: %w", err)
	}
	return id, nil
}

func (s *categoryStore) upsertTx(ctx context.Context, tx *sql.Tx, category string, color string) (any, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		return nil, nil
	}
	var nextSortOrder int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_categories`).Scan(&nextSortOrder); err != nil {
		return nil, fmt.Errorf("failed to query sort order for category: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO key_categories(name, color, sort_order, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
	`, category, color, nextSortOrder); err != nil {
		return nil, fmt.Errorf("failed to upsert category: %w", err)
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM key_categories WHERE name = ?`, category).Scan(&id); err != nil {
		return nil, fmt.Errorf("failed to resolve key category: %w", err)
	}
	return id, nil
}

func (s *categoryStore) resolveTx(ctx context.Context, tx *sql.Tx, category string, requestedID *int64, color string) (any, error) {
	if requestedID != nil && *requestedID > 0 {
		return *requestedID, nil
	}
	if strings.TrimSpace(category) == "" {
		return nil, nil
	}
	return s.upsertTx(ctx, tx, category, color)
}
