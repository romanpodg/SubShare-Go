package keymanagement

import (
	"context"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/keypersistence"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/vless"
)

func NormalizeKeyCategory(raw string) string {
	value := strings.TrimSpace(raw)
	if len(value) > 24 {
		value = value[:24]
	}
	return value
}

func (s *Service) ListLegacy(ctx context.Context) ([]model.VLESSKey, error) {
	keys, err := s.keyRepo.ListLegacy(ctx)
	return keys, mapKeyPersistenceError(err)
}

func (s *Service) CreateLegacy(ctx context.Context, params CreateLegacyParams) (int64, string, error) {
	label := strings.TrimSpace(params.Label)
	keyURL := strings.TrimSpace(params.URL)
	category := NormalizeKeyCategory(params.Category)
	templateText := strings.TrimSpace(params.TemplateText)

	kind, kindOK := model.NormalizeKeyKind(params.Kind)
	if !kindOK {
		return 0, "", ErrInvalidKeyKind
	}
	status, statusOK := model.NormalizeKeyStatus(params.Status)
	if !statusOK {
		return 0, "", ErrInvalidKeyStatus
	}
	if label == "" {
		return 0, "", ErrLabelRequired
	}
	if kind == model.KeyKindReal {
		if keyURL == "" {
			return 0, "", ErrURLRequired
		}
		if err := profileconfig.ValidateRealConfigURL(keyURL); err != nil {
			return 0, "", fmt.Errorf("%w: %s", ErrInvalidProfileURI, err.Error())
		}
	} else {
		if templateText == "" {
			templateText = label
		}
		token, err := generateToken(12)
		if err != nil {
			return 0, "", fmt.Errorf("failed to generate key: %w", err)
		}
		keyURL = "info://" + token
	}
	if len(label) > 255 {
		return 0, "", ErrLabelTooLong
	}
	if len(keyURL) > 65535 {
		return 0, "", ErrConfigurationTooLong
	}
	if len(templateText) > 8192 {
		return 0, "", ErrTemplateTextTooLong
	}

	keyID, err := s.keyRepo.CreateLegacy(ctx, keypersistence.CreateLegacyKeyParams{
		Label:        label,
		Status:       status,
		Kind:         kind,
		Category:     category,
		TemplateText: templateText,
		KeyURL:       keyURL,
	})
	if err != nil {
		return 0, "", mapKeyPersistenceError(err)
	}
	return keyID, label, nil
}

func (s *Service) UpdateLegacy(ctx context.Context, id int64, params UpdateLegacyParams) (string, error) {
	label := strings.TrimSpace(params.Label)
	category := NormalizeKeyCategory(params.Category)
	templateText := strings.TrimSpace(params.TemplateText)

	kind, kindOK := model.NormalizeKeyKind(params.Kind)
	if !kindOK {
		return "", ErrInvalidKeyKind
	}
	status, statusOK := model.NormalizeKeyStatus(params.Status)
	if !statusOK {
		return "", ErrInvalidKeyStatus
	}
	if label == "" {
		return "", ErrLabelRequired
	}
	if len(label) > 255 {
		return "", ErrLabelTooLong
	}

	_, existingURL, err := s.keyRepo.GetLegacyByID(ctx, id)
	if err != nil {
		return "", mapKeyPersistenceError(err)
	}

	builtURL := ""
	if kind == model.KeyKindReal {
		if rawURL := strings.TrimSpace(params.RawURL); rawURL != "" {
			if err := profileconfig.ValidateRealConfigURL(rawURL); err != nil {
				return "", fmt.Errorf("%w: %s", ErrInvalidProfileURI, err.Error())
			}
			builtURL = rawURL
		} else {
			var err error
			builtURL, err = vless.BuildVLESSURL(params.UUID, params.Host, params.Port, params.Query, params.Fragment)
			if err != nil {
				return "", fmt.Errorf("%w: %s", ErrInvalidProfileURI, err.Error())
			}
		}
	} else {
		if templateText == "" {
			templateText = label
		}
		if existingURL != "" {
			builtURL = strings.TrimSpace(existingURL)
		}
		if builtURL == "" {
			token, err := generateToken(12)
			if err != nil {
				return "", fmt.Errorf("failed to generate key: %w", err)
			}
			builtURL = "info://" + token
		}
	}

	if len(builtURL) > 65535 {
		return "", ErrConfigurationTooLong
	}
	if len(templateText) > 8192 {
		return "", ErrTemplateTextTooLong
	}

	if err := s.keyRepo.UpdateLegacy(ctx, keypersistence.UpdateLegacyKeyParams{
		ID:           id,
		Label:        label,
		Status:       status,
		Kind:         kind,
		Category:     category,
		TemplateText: templateText,
		BuiltURL:     builtURL,
	}); err != nil {
		return "", mapKeyPersistenceError(err)
	}

	return label, nil
}

func (s *Service) DeleteLegacy(ctx context.Context, id int64) error {
	return mapKeyPersistenceError(s.keyRepo.DeleteLegacy(ctx, id))
}
