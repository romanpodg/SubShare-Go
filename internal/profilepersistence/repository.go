package profilepersistence

import (
	"context"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type ProfileRepository interface {
	GetByID(ctx context.Context, id int64) (*model.VLESSKey, string, error)
	CreateLocal(ctx context.Context, params CreateProfileParams) (*model.VLESSKey, string, error)
	UpdateLocal(ctx context.Context, params UpdateProfileParams) (*model.VLESSKey, string, error)
	UpdateSourceOwnedMetadata(ctx context.Context, params UpdateSourceOwnedMetadataParams) (*model.VLESSKey, string, error)
	CloneLocal(ctx context.Context, params CloneProfileParams) (*model.VLESSKey, string, error)
}
