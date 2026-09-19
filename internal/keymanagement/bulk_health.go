package keymanagement

import (
	"context"
	"errors"
	"fmt"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type BulkUpdateKeysParams struct {
	IDs      []int64
	Status   string
	Category string
}

func normalizeBulkKeyIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, ErrBulkKeyIDsRequired
	}
	normalized := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrInvalidBulkKeyID
		}
		if _, exists := seen[id]; exists {
			return nil, ErrDuplicateBulkKeyIDs
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func (s *Service) BulkUpdateKeys(ctx context.Context, params BulkUpdateKeysParams) (int, error) {
	ids, err := normalizeBulkKeyIDs(params.IDs)
	if err != nil {
		return 0, err
	}
	status, ok := model.NormalizeKeyStatus(params.Status)
	if !ok {
		return 0, ErrInvalidKeyStatus
	}
	category := NormalizeKeyCategory(params.Category)
	applyCategory := category != ""
	var categoryID int64
	if applyCategory {
		categoryID, err = s.repo.EnsureKeyCategory(ctx, category)
		if err != nil {
			return 0, fmt.Errorf("%w: %w", ErrBulkCategoryPersistence, err)
		}
	}
	err = s.repo.BulkUpdateKeys(ctx, BulkKeyUpdate{
		IDs:           ids,
		Status:        status,
		ApplyCategory: applyCategory,
		Category:      category,
		CategoryID:    categoryID,
	})
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (s *Service) BulkDeleteKeys(ctx context.Context, ids []int64) (int, error) {
	normalized, err := normalizeBulkKeyIDs(ids)
	if err != nil {
		return 0, err
	}
	if err := s.repo.BulkDeleteKeys(ctx, normalized); err != nil {
		return 0, err
	}
	return len(normalized), nil
}

func (s *Service) GetHealthCheckTarget(ctx context.Context, id int64) (HealthCheckTarget, error) {
	target, err := s.repo.GetHealthCheckTarget(ctx, id)
	if err != nil {
		return HealthCheckTarget{}, err
	}
	if target.URL == "" {
		return HealthCheckTarget{}, ErrInformationalHealthCheck
	}
	return target, nil
}

func (s *Service) ListHealthCheckTargets(ctx context.Context) ([]HealthCheckTarget, error) {
	targets, err := s.repo.ListHealthCheckTargets(ctx)
	if err != nil {
		return nil, err
	}
	return targets, nil
}

func (s *Service) SaveHealthCheckResult(ctx context.Context, id int64, status string, detail string, latency int64) error {
	return s.repo.SaveHealthCheckResult(ctx, SaveHealthCheckResultParams{
		ID: id, Status: status, Error: detail, Latency: latency,
	})
}

func (s *Service) GetHealthCheckResult(ctx context.Context, id int64) (HealthCheckResult, error) {
	result, err := s.repo.GetHealthCheckResult(ctx, id)
	return result, err
}

func (s *Service) ListHealthCheckResults(ctx context.Context) ([]HealthCheckResult, error) {
	results, err := s.repo.ListHealthCheckResults(ctx)
	if err != nil {
		return nil, err
	}
	return results, nil
}

func MissingKeyID(err error) (int64, bool) {
	var target KeyNotFoundError
	if !errors.As(err, &target) {
		return 0, false
	}
	return target.ID, true
}
