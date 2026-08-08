package keymanagement

import (
	"context"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func ApplyStructuredPatchToURI(protocol, currentURI, label string, patch *model.StructuredProfilePatch) (string, error) {
	parsed, err := profiles.Parse(currentURI)
	if err != nil {
		return BuildURIFromStructuredCreate(protocol, label, patch)
	}

	if patch.Server != nil && patch.Server.Set && patch.Server.Operation == model.TriStateSet {
		parsed.Server = patch.Server.Value
	}
	if patch.Port != nil && patch.Port.Set && patch.Port.Operation == model.TriStateSet {
		parsed.Port = profiles.PortSpec{Expression: patch.Port.Value, Kind: profiles.PortSingle, Explicit: true}
	}
	if patch.DisplayName != nil && patch.DisplayName.Set && patch.DisplayName.Operation == model.TriStateSet {
		parsed.DisplayName = patch.DisplayName.Value
	}

	switch data := parsed.Data.(type) {
	case profiles.ShadowsocksData:
		if patch.Shadowsocks != nil {
			if patch.Shadowsocks.Method != nil && patch.Shadowsocks.Method.Set && patch.Shadowsocks.Method.Operation == model.TriStateSet {
				data.Method = patch.Shadowsocks.Method.Value
			}
			if patch.Shadowsocks.Password != nil && patch.Shadowsocks.Password.Set {
				if patch.Shadowsocks.Password.Operation == model.TriStateClear {
					return "", fmt.Errorf("password cannot be cleared for real shadowsocks profile")
				}
				data.Password = profiles.NewSensitiveValue(patch.Shadowsocks.Password.Value)
			}
			if patch.Shadowsocks.PluginName != nil && patch.Shadowsocks.PluginName.Set {
				if patch.Shadowsocks.PluginName.Operation == model.TriStateClear {
					data.Plugin = nil
					filteredQP := make([]profiles.QueryParameter, 0, len(parsed.QueryParameters))
					for _, qp := range parsed.QueryParameters {
						if strings.ToLower(qp.Key) != "plugin" {
							filteredQP = append(filteredQP, qp)
						}
					}
					parsed.QueryParameters = filteredQP
				} else if patch.Shadowsocks.PluginName.Value != "" {
					if data.Plugin == nil {
						data.Plugin = &profiles.ShadowsocksPlugin{Name: patch.Shadowsocks.PluginName.Value}
					} else {
						data.Plugin.Name = patch.Shadowsocks.PluginName.Value
					}
				}
			}
			if patch.Shadowsocks.PluginOptions != nil && patch.Shadowsocks.PluginOptions.Set {
				if patch.Shadowsocks.PluginOptions.Operation == model.TriStateClear {
					if data.Plugin != nil {
						data.Plugin.Options = profiles.NewSensitiveValue("")
					}
				} else if data.Plugin != nil {
					data.Plugin.Options = profiles.NewSensitiveValue(patch.Shadowsocks.PluginOptions.Value)
				}
			}
		}
		parsed.Data = data
	case profiles.Hysteria2Data:
		if patch.Hysteria2 != nil {
			if patch.Hysteria2.Authentication != nil && patch.Hysteria2.Authentication.Set {
				if patch.Hysteria2.Authentication.Operation == model.TriStateClear {
					return "", fmt.Errorf("authentication cannot be cleared for real hysteria2 profile")
				}
				data.Authentication = profiles.NewSensitiveValue(patch.Hysteria2.Authentication.Value)
			}
			if patch.Hysteria2.SNI != nil && patch.Hysteria2.SNI.Set {
				if patch.Hysteria2.SNI.Operation == model.TriStateClear {
					data.SNI = ""
				} else {
					data.SNI = patch.Hysteria2.SNI.Value
				}
			}
			if patch.Hysteria2.Insecure != nil && patch.Hysteria2.Insecure.Set {
				data.Insecure = patch.Hysteria2.Insecure.Value
			}
			if patch.Hysteria2.CertificateSHA256 != nil && patch.Hysteria2.CertificateSHA256.Set {
				if patch.Hysteria2.CertificateSHA256.Operation == model.TriStateClear {
					data.CertificateSHA256 = ""
				} else {
					data.CertificateSHA256 = patch.Hysteria2.CertificateSHA256.Value
				}
			}
			if patch.Hysteria2.ObfuscationType != nil && patch.Hysteria2.ObfuscationType.Set {
				if patch.Hysteria2.ObfuscationType.Operation == model.TriStateClear {
					data.ObfuscationType = ""
				} else {
					data.ObfuscationType = patch.Hysteria2.ObfuscationType.Value
				}
			}
			if patch.Hysteria2.ObfuscationPassword != nil && patch.Hysteria2.ObfuscationPassword.Set {
				if patch.Hysteria2.ObfuscationPassword.Operation == model.TriStateClear {
					data.ObfuscationPassword = profiles.NewSensitiveValue("")
				} else {
					data.ObfuscationPassword = profiles.NewSensitiveValue(patch.Hysteria2.ObfuscationPassword.Value)
				}
			}
		}
		parsed.Data = data
	case profiles.TUICData:
		if patch.TUIC != nil {
			if patch.TUIC.UUID != nil && patch.TUIC.UUID.Set {
				if patch.TUIC.UUID.Operation == model.TriStateClear {
					return "", fmt.Errorf("uuid cannot be cleared for real tuic profile")
				}
				data.UUID = profiles.NewSensitiveValue(patch.TUIC.UUID.Value)
			}
			if patch.TUIC.Password != nil && patch.TUIC.Password.Set {
				if patch.TUIC.Password.Operation == model.TriStateClear {
					return "", fmt.Errorf("password cannot be cleared for real tuic profile")
				}
				data.Password = profiles.NewSensitiveValue(patch.TUIC.Password.Value)
			}
			if patch.TUIC.SNI != nil && patch.TUIC.SNI.Set {
				if patch.TUIC.SNI.Operation == model.TriStateClear {
					data.SNI = ""
				} else {
					data.SNI = patch.TUIC.SNI.Value
				}
			}
			if patch.TUIC.ALPN != nil && patch.TUIC.ALPN.Set {
				if patch.TUIC.ALPN.Operation == model.TriStateClear {
					data.ALPN = []string{}
				} else {
					data.ALPN = patch.TUIC.ALPN.Value
				}
			}
			if patch.TUIC.SkipCertVerify != nil && patch.TUIC.SkipCertVerify.Set {
				data.SkipCertificateVerification = patch.TUIC.SkipCertVerify.Value
			}
			if patch.TUIC.CongestionController != nil && patch.TUIC.CongestionController.Set {
				if patch.TUIC.CongestionController.Operation == model.TriStateClear {
					data.CongestionController = ""
				} else {
					data.CongestionController = patch.TUIC.CongestionController.Value
				}
			}
			if patch.TUIC.UDPRelayMode != nil && patch.TUIC.UDPRelayMode.Set {
				if patch.TUIC.UDPRelayMode.Operation == model.TriStateClear {
					data.UDPRelayMode = ""
				} else {
					data.UDPRelayMode = patch.TUIC.UDPRelayMode.Value
				}
			}
			if patch.TUIC.UDPOverStream != nil && patch.TUIC.UDPOverStream.Set {
				data.UDPOverStream = patch.TUIC.UDPOverStream.Value
			}
			if patch.TUIC.ZeroRTT != nil && patch.TUIC.ZeroRTT.Set {
				data.ZeroRTT = patch.TUIC.ZeroRTT.Value
			}
			if patch.TUIC.Heartbeat != nil && patch.TUIC.Heartbeat.Set {
				if patch.TUIC.Heartbeat.Operation == model.TriStateClear {
					data.Heartbeat = ""
				} else {
					data.Heartbeat = patch.TUIC.Heartbeat.Value
				}
			}
		}
		parsed.Data = data
	}

	res, err := profiles.Serialize(parsed, profiles.CanonicalSerialization)
	if err != nil {
		return "", err
	}
	return res.URI.Reveal(), nil
}

func (s *Service) UpdateLocal(ctx context.Context, params UpdateLocalParams) (*model.KeyProfileDetailResponse, error) {
	key, decryptedURI, fetchErr := s.profileRepo.GetByID(ctx, params.ID)
	if fetchErr != nil {
		return nil, fetchErr
	}

	if key.ExternalSourceID > 0 {
		return nil, ErrSourceOwnedReadOnly
	}

	if key.ProfileRevision != params.ProfileRevision {
		return nil, ErrProfileRevisionConflict
	}

	label := strings.TrimSpace(params.Label)
	status, statusOK := model.NormalizeKeyStatus(params.Status)
	kind, kindOK := model.NormalizeKeyKind(params.Kind)
	templateText := strings.TrimSpace(params.TemplateText)

	if !statusOK || !kindOK || label == "" {
		return nil, ErrInvalidInput
	}
	if len(label) > 255 {
		return nil, ErrLabelTooLong
	}

	patchMode := strings.ToLower(strings.TrimSpace(params.PatchMode))
	if patchMode == "" {
		if params.RawURI != "" {
			patchMode = "raw"
		} else {
			patchMode = "structured"
		}
	}
	if patchMode != "raw" && patchMode != "structured" {
		return nil, ErrInvalidPatchMode
	}
	if patchMode == "raw" && params.StructuredPatch != nil {
		return nil, ErrMutuallyExclusiveMode
	}
	if patchMode == "structured" && strings.TrimSpace(params.RawURI) != "" {
		return nil, ErrMutuallyExclusiveMode
	}

	newURI := decryptedURI
	if kind == model.KeyKindReal {
		if patchMode == "raw" {
			newURI = strings.TrimSpace(params.RawURI)
			if newURI == "" {
				return nil, ErrRawURIRequired
			}
			if _, parseErr := profiles.Parse(newURI); parseErr != nil {
				return nil, invalidProfileURIError(parseErr)
			}
		} else {
			if params.StructuredPatch == nil {
				return nil, ErrStructuredPatchRequired
			}
			var err error
			newURI, err = ApplyStructuredPatchToURI(key.Protocol, decryptedURI, label, params.StructuredPatch)
			if err != nil {
				return nil, err
			}
		}
	}

	storedProtocol := "legacy"
	if parsed, err := profiles.Parse(newURI); err == nil && parsed != nil {
		storedProtocol = string(parsed.Protocol)
	}

	updatedKey, updatedURI, err := s.profileRepo.UpdateLocal(ctx, profilepersistence.UpdateProfileParams{
		ID:               params.ID,
		ExpectedRevision: params.ProfileRevision,
		Label:            label,
		Status:           status,
		Kind:             kind,
		Category:         params.Category,
		CategoryID:       params.CategoryID,
		TemplateText:     templateText,
		Protocol:         storedProtocol,
		NewURI:           newURI,
	})
	if err != nil {
		return nil, err
	}

	detail := BuildKeyProfileDetailResponse(*updatedKey, updatedURI, s.capabilityResolver)
	return &detail, nil
}
