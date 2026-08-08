package keymanagement

import (
	"context"

	"github.com/romanpodg/SubShare-Go/internal/keypersistence"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
)

type ProfileRepository = profilepersistence.ProfileRepository
type KeyRepository = keypersistence.KeyRepository

type Service struct {
	profileRepo        ProfileRepository
	keyRepo            KeyRepository
	capabilityResolver CapabilityResolver
}

func NewService(profileRepo ProfileRepository, keyRepo KeyRepository, resolver CapabilityResolver) *Service {
	return &Service{
		profileRepo:        profileRepo,
		keyRepo:            keyRepo,
		capabilityResolver: resolver,
	}
}

func (s *Service) GetRawByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	return s.profileRepo.GetByID(ctx, id)
}
