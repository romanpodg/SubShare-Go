package keymanagement

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/keypersistence"
	"github.com/romanpodg/SubShare-Go/internal/model"
)

type BulkUpdateKeysParams struct {
	IDs      []int64
	Status   string
	Category string
}

type HealthCheckTarget struct {
	ID  int64
	URL string
}

type HealthCheckResult struct {
	ID            int64
	Status        string
	Error         string
	LastCheckedAt *time.Time
	Latency       int64
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
		categoryID, err = s.keyRepo.EnsureKeyCategory(ctx, category)
		if err != nil {
			return 0, fmt.Errorf("%w: %w", ErrBulkCategoryPersistence, mapKeyPersistenceError(err))
		}
	}
	err = s.keyRepo.BulkUpdateKeys(ctx, keypersistence.BulkUpdateKeysParams{
		IDs:           ids,
		Status:        status,
		ApplyCategory: applyCategory,
		Category:      category,
		CategoryID:    categoryID,
	})
	if err != nil {
		return 0, mapKeyPersistenceError(err)
	}
	return len(ids), nil
}

func (s *Service) BulkDeleteKeys(ctx context.Context, ids []int64) (int, error) {
	normalized, err := normalizeBulkKeyIDs(ids)
	if err != nil {
		return 0, err
	}
	if err := s.keyRepo.BulkDeleteKeys(ctx, normalized); err != nil {
		return 0, mapKeyPersistenceError(err)
	}
	return len(normalized), nil
}

func (s *Service) GetHealthCheckTarget(ctx context.Context, id int64) (HealthCheckTarget, error) {
	target, err := s.keyRepo.GetHealthCheckTarget(ctx, id)
	if err != nil {
		return HealthCheckTarget{}, mapKeyPersistenceError(err)
	}
	if target.URL == "" {
		return HealthCheckTarget{}, ErrInformationalHealthCheck
	}
	return HealthCheckTarget{ID: target.ID, URL: target.URL}, nil
}

func (s *Service) ListHealthCheckTargets(ctx context.Context) ([]HealthCheckTarget, error) {
	targets, err := s.keyRepo.ListHealthCheckTargets(ctx)
	if err != nil {
		return nil, mapKeyPersistenceError(err)
	}
	result := make([]HealthCheckTarget, 0, len(targets))
	for _, target := range targets {
		result = append(result, HealthCheckTarget{ID: target.ID, URL: target.URL})
	}
	return result, nil
}

func (s *Service) SaveHealthCheckResult(ctx context.Context, id int64, status string, detail string, latency int64) error {
	return mapKeyPersistenceError(s.keyRepo.SaveHealthCheckResult(ctx, keypersistence.SaveHealthCheckResultParams{
		ID: id, Status: status, Error: detail, Latency: latency,
	}))
}

func (s *Service) GetHealthCheckResult(ctx context.Context, id int64) (HealthCheckResult, error) {
	result, err := s.keyRepo.GetHealthCheckResult(ctx, id)
	if err != nil {
		return HealthCheckResult{}, mapKeyPersistenceError(err)
	}
	return healthCheckResult(result), nil
}

func (s *Service) ListHealthCheckResults(ctx context.Context) ([]HealthCheckResult, error) {
	results, err := s.keyRepo.ListHealthCheckResults(ctx)
	if err != nil {
		return nil, mapKeyPersistenceError(err)
	}
	out := make([]HealthCheckResult, 0, len(results))
	for _, result := range results {
		out = append(out, healthCheckResult(result))
	}
	return out, nil
}

func healthCheckResult(result keypersistence.HealthCheckResult) HealthCheckResult {
	return HealthCheckResult{
		ID: result.ID, Status: result.Status, Error: result.Error,
		LastCheckedAt: result.LastCheckedAt, Latency: result.Latency,
	}
}

func MissingKeyID(err error) (int64, bool) {
	var target KeyNotFoundError
	if !errors.As(err, &target) {
		return 0, false
	}
	return target.ID, true
}
