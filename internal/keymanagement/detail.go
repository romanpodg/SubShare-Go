package keymanagement

import (
	"context"
	"strconv"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

type CapabilityResolver func(parsed *profiles.Profile) map[string]map[string]any

func SanitizeCheckError(raw string) model.SanitizedCheckError {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return model.SanitizedCheckError{Code: "none", Message: ""}
	}
	switch {
	case strings.Contains(trimmed, "connection refused") || strings.Contains(trimmed, "refused"):
		return model.SanitizedCheckError{Code: "connection_refused", Message: "Connection refused by target server"}
	case strings.Contains(trimmed, "timeout") || strings.Contains(trimmed, "timed out") || strings.Contains(trimmed, "deadline"):
		return model.SanitizedCheckError{Code: "timeout", Message: "Connection timed out"}
	case strings.Contains(trimmed, "tls") || strings.Contains(trimmed, "handshake") || strings.Contains(trimmed, "certificate"):
		return model.SanitizedCheckError{Code: "tls_handshake_failed", Message: "TLS handshake failed"}
	case strings.Contains(trimmed, "dns") || strings.Contains(trimmed, "lookup") || strings.Contains(trimmed, "host"):
		return model.SanitizedCheckError{Code: "dns_lookup_failed", Message: "DNS resolution failed"}
	default:
		return model.SanitizedCheckError{Code: "health_check_failed", Message: "Health check failed"}
	}
}

func BuildSafeStructuredProfile(parsed *profiles.Profile) *model.SafeStructuredProfile {
	if parsed == nil {
		return nil
	}
	safe := &model.SafeStructuredProfile{
		Server:      parsed.Server,
		Port:        parsed.Port.Expression,
		PortKind:    string(parsed.Port.Kind),
		DisplayName: parsed.DisplayName,
	}

	switch data := parsed.Data.(type) {
	case profiles.ShadowsocksData:
		safe.Shadowsocks = &model.SafeShadowsocksDetail{
			Method:               data.Method,
			PasswordPresent:      data.Password.IsSet(),
			PluginName:           "",
			PluginOptionsPresent: data.Plugin != nil && data.Plugin.Options.IsSet(),
			UserInfoStyle:        string(data.UserInfoStyle),
		}
		if data.Plugin != nil {
			safe.Shadowsocks.PluginName = data.Plugin.Name
		}
	case profiles.Hysteria2Data:
		safe.Hysteria2 = &model.SafeHysteria2Detail{
			AuthenticationPresent:      data.Authentication.IsSet(),
			SNI:                        data.SNI,
			Insecure:                   data.Insecure,
			CertificateSHA256:          data.CertificateSHA256,
			ObfuscationType:            data.ObfuscationType,
			ObfuscationPasswordPresent: data.ObfuscationPassword.IsSet(),
		}
	case profiles.TUICData:
		obsDTO := make([]model.TUICFieldObservationDTO, 0, len(data.FieldObservations))
		for _, obs := range data.FieldObservations {
			obsDTO = append(obsDTO, model.TUICFieldObservationDTO{
				Field:      obs.Field,
				FieldClass: string(obs.FieldClass),
				Provenance: string(obs.SourceProvenance),
			})
		}
		safe.TUIC = &model.SafeTUICDetail{
			Generation:                  data.Generation,
			UUIDPresent:                 data.UUID.IsSet(),
			PasswordPresent:             data.Password.IsSet(),
			TokenPresent:                data.Token.IsSet(),
			SNI:                         data.SNI,
			ALPN:                        append([]string(nil), data.ALPN...),
			SkipCertificateVerification: data.SkipCertificateVerification,
			DisableSNI:                  data.DisableSNI,
			CongestionController:        data.CongestionController,
			UDPRelayMode:                data.UDPRelayMode,
			UDPOverStream:               data.UDPOverStream,
			ZeroRTT:                     data.ZeroRTT,
			Heartbeat:                   data.Heartbeat,
			FieldObservations:           obsDTO,
		}
	}

	return safe
}

func buildSafeXrayJSONProfile(raw string) *model.SafeStructuredProfile {
	drafts, err := profileconfig.ParseXrayJSONDrafts(raw)
	if err != nil || len(drafts) == 0 {
		return nil
	}
	draft := drafts[0]
	return &model.SafeStructuredProfile{
		Server:      draft.Server,
		Port:        strconv.Itoa(draft.Port),
		PortKind:    string(profiles.PortSingle),
		DisplayName: draft.Remark,
		XrayJSON: &model.SafeXrayJSONDetail{
			HasRawJSON: true,
			Network:    draft.Network,
			Security:   draft.Security,
		},
	}
}

func BuildKeyProfileDetailResponse(key model.VLESSKey, decryptedURI string, resolver CapabilityResolver) model.KeyProfileDetailResponse {
	var categoryID *int64
	if key.CategoryID > 0 {
		categoryID = &key.CategoryID
	}
	var extSourceID *int64
	ownership := model.OwnershipLocal
	if key.ExternalSourceID > 0 {
		extSourceID = &key.ExternalSourceID
		ownership = model.OwnershipExternalSource
	}

	warnings := key.ProfileWarnings
	if warnings == nil {
		warnings = []string{}
	}

	var parsed *profiles.Profile
	if key.Kind == model.KeyKindReal && decryptedURI != "" {
		if p, err := profiles.Parse(decryptedURI); err == nil {
			parsed = p
		}
	}

	safeStructured := BuildSafeStructuredProfile(parsed)
	if safeStructured == nil && key.Kind == model.KeyKindReal && profileconfig.SupportedConfigScheme(decryptedURI) == "xray-json" {
		safeStructured = buildSafeXrayJSONProfile(decryptedURI)
	}
	unknownParams := make([]model.UnknownQueryParamDTO, 0)
	if parsed != nil {
		for _, qp := range parsed.UnknownQueryParameters {
			unknownParams = append(unknownParams, model.UnknownQueryParamDTO{
				Key:      qp.Key,
				HasValue: qp.HasValue,
			})
		}
	}

	var caps map[string]map[string]any
	if resolver != nil {
		caps = resolver(parsed)
	} else {
		caps = make(map[string]map[string]any)
	}
	sanitizedErr := SanitizeCheckError(key.CheckError)

	return model.KeyProfileDetailResponse{
		ID:                          key.ID,
		Label:                       key.Label,
		ClientDisplayName:           key.ClientDisplayName,
		ClientDisplayNameOverridden: key.ClientDisplayNameOverridden,
		CategoryID:                  categoryID,
		Category:                    key.Category,
		Kind:                        key.Kind,
		Status:                      key.Status,
		CheckStatus:                 key.CheckStatus,
		CheckError:                  sanitizedErr,
		LastLatencyMS:               key.LastLatencyMS,
		LastCheckedAt:               key.LastCheckedAtText,
		TemplateText:                key.TemplateText,
		Ownership:                   ownership,
		ExternalSourceID:            extSourceID,
		ExternalSourceName:          key.ExternalSourceName,
		Protocol:                    key.Protocol,
		ProfileSchemaVersion:        key.ProfileSchemaVersion,
		ProfileCompatibility:        key.ProfileCompatibility,
		ProfileWarnings:             warnings,
		ProfileRevision:             key.ProfileRevision,
		CreatedAt:                   key.CreatedAt,
		UpdatedAt:                   key.UpdatedAt,
		SafeStructured:              safeStructured,
		UnknownQueryParameters:      unknownParams,
		Capabilities:                caps,
	}
}

func (s *Service) GetDetail(ctx context.Context, id int64) (*model.KeyProfileDetailResponse, error) {
	key, decryptedURI, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	detail := BuildKeyProfileDetailResponse(*key, decryptedURI, s.capabilityResolver)
	return &detail, nil
}
