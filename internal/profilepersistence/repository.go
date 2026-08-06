package profilepersistence

import (
	"context"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type ProfileRepository interface {
	GetByID(ctx context.Context, id int64) (*model.VLESSKey, string, error)
	CreateLocal(ctx context.Context, params CreateProfileParams) (*model.VLESSKey, string, error)
	UpdateLocal(ctx context.Context, params UpdateProfileParams) (*model.VLESSKey, string, error)
	CloneLocal(ctx context.Context, params CloneProfileParams) (*model.VLESSKey, string, error)
	ListLegacy(ctx context.Context) ([]model.VLESSKey, error)
	CreateLegacy(ctx context.Context, params CreateLegacyKeyParams) (int64, error)
	UpdateLegacy(ctx context.Context, params UpdateLegacyKeyParams) error
	DeleteLegacy(ctx context.Context, id int64) error
}
