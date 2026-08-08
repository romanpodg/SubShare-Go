package keypersistence

import (
	"context"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type KeyRepository interface {
	GetLegacyByID(ctx context.Context, id int64) (*model.VLESSKey, string, error)
	ListLegacy(ctx context.Context) ([]model.VLESSKey, error)
	CreateLegacy(ctx context.Context, params CreateLegacyKeyParams) (int64, error)
	UpdateLegacy(ctx context.Context, params UpdateLegacyKeyParams) error
	DeleteLegacy(ctx context.Context, id int64) error
	EnsureKeyCategory(ctx context.Context, name string) (int64, error)
	ListKeyCategories(ctx context.Context) ([]model.KeyCategory, error)
	CreateKeyCategory(ctx context.Context, params CreateCategoryParams) (model.KeyCategory, error)
	UpdateKeyCategory(ctx context.Context, params UpdateCategoryParams) (model.KeyCategory, error)
	DeleteKeyCategory(ctx context.Context, params DeleteCategoryParams) error
	ReorderKeyCategories(ctx context.Context, names []string) error
	ReorderKeys(ctx context.Context, ids []int64) error
	GetCategoryColor(ctx context.Context, name string) (string, error)
	BulkUpdateKeys(ctx context.Context, params BulkUpdateKeysParams) error
	BulkDeleteKeys(ctx context.Context, ids []int64) error
	GetHealthCheckTarget(ctx context.Context, id int64) (HealthCheckTarget, error)
	ListHealthCheckTargets(ctx context.Context) ([]HealthCheckTarget, error)
	SaveHealthCheckResult(ctx context.Context, params SaveHealthCheckResultParams) error
	GetHealthCheckResult(ctx context.Context, id int64) (HealthCheckResult, error)
	ListHealthCheckResults(ctx context.Context) ([]HealthCheckResult, error)
}
