package keymanagement

import (
	"context"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
)

type ProfileRepository = profilepersistence.ProfileRepository

type Service struct {
	repo               ProfileRepository
	capabilityResolver CapabilityResolver
}

func NewService(repo ProfileRepository, resolver CapabilityResolver) *Service {
	return &Service{
		repo:               repo,
		capabilityResolver: resolver,
	}
}

func (s *Service) GetRawByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	return s.repo.GetByID(ctx, id)
}
