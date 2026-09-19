package keymanagement

import (
	"context"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

type Service struct {
	repo               Repository
	capabilityResolver CapabilityResolver
}

func NewService(repo Repository, resolver CapabilityResolver) *Service {
	return &Service{repo: repo, capabilityResolver: resolver}
}

func (s *Service) GetRawByID(ctx context.Context, id int64) (*model.VLESSKey, string, error) {
	return s.repo.GetByID(ctx, id)
}
