package keymanagement

import (
	"context"
	"regexp"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

var hexColorRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func NormalizeKeyCategoryColor(raw string) string {
	val := strings.TrimSpace(raw)
	if val == "" {
		return "#d8b33d"
	}
	if hexColorRegex.MatchString(val) {
		return strings.ToUpper(val)
	}
	return "#d8b33d"
}

func (s *Service) ListCategories(ctx context.Context) ([]model.KeyCategory, error) {
	categories, err := s.repo.ListKeyCategories(ctx)
	if err != nil {
		return nil, err
	}
	if categories == nil {
		categories = []model.KeyCategory{}
	}
	return categories, nil
}

func (s *Service) EnsureCategory(ctx context.Context, rawName string) (int64, error) {
	name := NormalizeKeyCategory(rawName)
	if name == "" {
		return 0, nil
	}
	id, err := s.repo.EnsureKeyCategory(ctx, name)
	return id, err
}

func (s *Service) CreateCategory(ctx context.Context, params CreateCategoryParams) (model.KeyCategory, error) {
	rawName := strings.TrimSpace(params.Name)
	if rawName == "" {
		return model.KeyCategory{}, ErrCategoryNameRequired
	}
	color := NormalizeKeyCategoryColor(params.Color)
	name := NormalizeKeyCategory(rawName)

	category, err := s.repo.CreateKeyCategory(ctx, CreateCategoryParams{
		Name:  name,
		Color: color,
	})
	return category, err
}

func (s *Service) UpdateCategory(ctx context.Context, params UpdateCategoryParams) (model.KeyCategory, error) {
	oldRaw := strings.TrimSpace(params.OldName)
	newRaw := strings.TrimSpace(params.NewName)
	color := NormalizeKeyCategoryColor(params.Color)
	if oldRaw == "" || newRaw == "" {
		return model.KeyCategory{}, ErrCategoryOldAndNewRequired
	}

	oldName := NormalizeKeyCategory(oldRaw)
	newName := NormalizeKeyCategory(newRaw)
	if oldName == "" || newName == "" {
		return model.KeyCategory{}, ErrCategoryNameEmpty
	}

	category, err := s.repo.UpdateKeyCategory(ctx, UpdateCategoryParams{
		OldName: oldName,
		NewName: newName,
		Color:   color,
	})
	return category, err
}

func (s *Service) RenameCategory(ctx context.Context, params RenameCategoryParams) (model.KeyCategory, error) {
	oldRaw := strings.TrimSpace(params.OldName)
	if oldRaw == "" {
		return model.KeyCategory{}, ErrCategoryOldAndNewRequired
	}
	oldName := NormalizeKeyCategory(oldRaw)
	currentColor, err := s.repo.GetCategoryColor(ctx, oldName)
	if err != nil {
		return model.KeyCategory{}, err
	}

	return s.UpdateCategory(ctx, UpdateCategoryParams{
		OldName: params.OldName,
		NewName: params.NewName,
		Color:   currentColor,
	})
}

func (s *Service) DeleteCategory(ctx context.Context, params DeleteCategoryParams) error {
	name := NormalizeKeyCategory(params.Name)
	mode := strings.TrimSpace(params.Mode)
	if mode != "delete_with_keys" && mode != "keep_keys" {
		return ErrInvalidDeleteMode
	}

	return s.repo.DeleteKeyCategory(ctx, DeleteCategoryParams{
		Name: name,
		Mode: mode,
	})
}

func (s *Service) ReorderCategories(ctx context.Context, params ReorderCategoriesParams) error {
	if len(params.Names) == 0 {
		return ErrCategoryNamesRequired
	}

	normalized := make([]string, 0, len(params.Names))
	seen := make(map[string]struct{}, len(params.Names))
	for _, name := range params.Names {
		value := NormalizeKeyCategory(name)
		if value == "" {
			return ErrCategoryNameEmpty
		}
		if _, exists := seen[value]; exists {
			return ErrDuplicateCategoryNames
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	return s.repo.ReorderKeyCategories(ctx, normalized)
}

func (s *Service) ReorderKeys(ctx context.Context, ids []int64) error {
	return s.repo.ReorderKeys(ctx, ids)
}
