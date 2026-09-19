package keymanagement

import (
	"context"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

// Repository is the single persistence seam for keys, profiles, categories and
// health checks. Production adapter: storage.Repository. Test adapter: the
// in-memory fake in keymanagement_test.go.
type Repository interface {
	// Profile (modern, revisioned) operations.
	GetByID(ctx context.Context, id int64) (*model.VLESSKey, string, error)
	CreateLocal(ctx context.Context, params CreateProfileParams) (*model.VLESSKey, string, error)
	UpdateLocal(ctx context.Context, params UpdateProfileParams) (*model.VLESSKey, string, error)
	UpdateSourceOwnedMetadata(ctx context.Context, params UpdateSourceOwnedMetadataParams) (*model.VLESSKey, string, error)
	CloneLocal(ctx context.Context, params CloneProfileParams) (*model.VLESSKey, string, error)

	// Legacy key CRUD.
	GetLegacyByID(ctx context.Context, id int64) (*model.VLESSKey, string, error)
	ListLegacy(ctx context.Context) ([]model.VLESSKey, error)
	CreateLegacy(ctx context.Context, params CreateLegacyKeyParams) (int64, error)
	UpdateLegacy(ctx context.Context, params UpdateLegacyKeyParams) error
	DeleteLegacy(ctx context.Context, id int64) error

	// Categories and ordering.
	EnsureKeyCategory(ctx context.Context, name string) (int64, error)
	ListKeyCategories(ctx context.Context) ([]model.KeyCategory, error)
	CreateKeyCategory(ctx context.Context, params CreateCategoryParams) (model.KeyCategory, error)
	UpdateKeyCategory(ctx context.Context, params UpdateCategoryParams) (model.KeyCategory, error)
	DeleteKeyCategory(ctx context.Context, params DeleteCategoryParams) error
	ReorderKeyCategories(ctx context.Context, names []string) error
	ReorderKeys(ctx context.Context, ids []int64) error
	GetCategoryColor(ctx context.Context, name string) (string, error)

	// Bulk operations.
	BulkUpdateKeys(ctx context.Context, params BulkKeyUpdate) error
	BulkDeleteKeys(ctx context.Context, ids []int64) error

	// Health checks.
	GetHealthCheckTarget(ctx context.Context, id int64) (HealthCheckTarget, error)
	ListHealthCheckTargets(ctx context.Context) ([]HealthCheckTarget, error)
	SaveHealthCheckResult(ctx context.Context, params SaveHealthCheckResultParams) error
	GetHealthCheckResult(ctx context.Context, id int64) (HealthCheckResult, error)
	ListHealthCheckResults(ctx context.Context) ([]HealthCheckResult, error)
}

type CreateProfileParams struct {
	Label             string
	ClientDisplayName string
	Status            string
	Kind              string
	Category          string
	CategoryID        *int64
	TemplateText      string
	Protocol          string
	BuiltURI          string
}

type UpdateProfileParams struct {
	ID                int64
	ExpectedRevision  int64
	Label             string
	ClientDisplayName *string
	Status            string
	Kind              string
	Category          string
	CategoryID        *int64
	TemplateText      string
	Protocol          string
	NewURI            string
}

type UpdateSourceOwnedMetadataParams struct {
	ID                int64
	ExpectedRevision  int64
	Status            string
	ClientDisplayName *string
}

type CloneProfileParams struct {
	ID               int64
	ExpectedRevision int64
	NewLabel         string
}

type CreateLegacyKeyParams struct {
	Label        string
	Status       string
	Kind         string
	Category     string
	TemplateText string
	KeyURL       string
}

type UpdateLegacyKeyParams struct {
	ID           int64
	Label        string
	Status       string
	Kind         string
	Category     string
	TemplateText string
	BuiltURL     string
}

// BulkKeyUpdate is the resolved form of BulkUpdateKeysParams handed to the
// repository after category normalization.
type BulkKeyUpdate struct {
	IDs           []int64
	Status        string
	ApplyCategory bool
	Category      string
	CategoryID    int64
}

type HealthCheckTarget struct {
	ID         int64
	URL        string
	Status     string
	Kind       string
	Unreadable bool
}

type SaveHealthCheckResultParams struct {
	ID      int64
	Status  string
	Error   string
	Latency int64
}

type HealthCheckResult struct {
	ID            int64
	Status        string
	Error         string
	LastCheckedAt *time.Time
	Latency       int64
}
