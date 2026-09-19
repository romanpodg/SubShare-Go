package keymanagement

import (
	"context"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (s *Service) CloneAsLocal(ctx context.Context, params CloneParams) (*model.KeyProfileDetailResponse, error) {
	newLabel := strings.TrimSpace(params.NewLabel)
	if len(newLabel) > 255 {
		return nil, ErrLabelTooLong
	}

	clonedKey, clonedURI, err := s.repo.CloneLocal(ctx, CloneProfileParams{
		ID:               params.ID,
		ExpectedRevision: params.ExpectedProfileRevision,
		NewLabel:         newLabel,
	})
	if err != nil {
		return nil, err
	}

	detail := BuildKeyProfileDetailResponse(*clonedKey, clonedURI, s.capabilityResolver)
	return &detail, nil
}
