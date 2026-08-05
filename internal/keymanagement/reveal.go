package keymanagement

import (
	"context"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func (s *Service) Reveal(ctx context.Context, params RevealParams) (*RevealResult, error) {
	if params.Target != "raw" && params.Target != "structured-secrets" {
		return nil, ErrInvalidTarget
	}

	key, decryptedURI, err := s.repo.GetByID(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	if key.ProfileRevision != params.ProfileRevision {
		return nil, ErrProfileRevisionConflict
	}

	result := &RevealResult{
		KeyID:                    params.ID,
		ConfirmedProfileRevision: key.ProfileRevision,
		Target:                   params.Target,
	}

	if params.Target == "raw" {
		result.RawURI = decryptedURI
		return result, nil
	}

	// structured-secrets
	secrets := model.StructuredSecretsMap{}
	if p, parseErr := profiles.Parse(decryptedURI); parseErr == nil && p != nil {
		switch data := p.Data.(type) {
		case profiles.ShadowsocksData:
			secrets.Password = data.Password.Reveal()
			if data.Plugin != nil {
				secrets.PluginOptions = data.Plugin.Options.Reveal()
			}
		case profiles.Hysteria2Data:
			secrets.Authentication = data.Authentication.Reveal()
			secrets.ObfuscationPassword = data.ObfuscationPassword.Reveal()
		case profiles.TUICData:
			secrets.UUID = data.UUID.Reveal()
			secrets.Password = data.Password.Reveal()
			secrets.Token = data.Token.Reveal()
		}
	}

	result.Secrets = secrets
	return result, nil
}
