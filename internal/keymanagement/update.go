package keymanagement

import (
	"context"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

func ApplyStructuredPatchToURI(protocol, currentURI, label string, patch *model.StructuredProfilePatch) (string, error) {
	if structuredPatchHasNoFields(patch) {
		return currentURI, nil
	}
	parsed, err := profiles.Parse(currentURI)
	if err != nil {
		return BuildURIFromStructuredCreate(protocol, label, patch)
	}

	applyStructuredEndpointPatch(parsed, patch)
	if err := applyStructuredProtocolPatch(parsed, patch); err != nil {
		return "", err
	}

	res, err := profiles.Serialize(parsed, profiles.CanonicalSerialization)
	if err != nil {
		return "", err
	}
	return res.URI.Reveal(), nil
}

func structuredPatchHasNoFields(patch *model.StructuredProfilePatch) bool {
	if patch == nil {
		return false
	}
	for _, present := range []bool{patch.Server != nil, patch.Port != nil, patch.DisplayName != nil, patch.Shadowsocks != nil, patch.Hysteria2 != nil, patch.TUIC != nil} {
		if present {
			return false
		}
	}
	return true
}

func applyStructuredEndpointPatch(parsed *profiles.Profile, patch *model.StructuredProfilePatch) {
	parsed.Server = setString(patch.Server, parsed.Server)
	if isSet(patch.Port) {
		parsed.Port = profiles.PortSpec{Expression: patch.Port.Value, Kind: profiles.PortSingle, Explicit: true}
	}
	parsed.DisplayName = setString(patch.DisplayName, parsed.DisplayName)
}

func applyStructuredProtocolPatch(parsed *profiles.Profile, patch *model.StructuredProfilePatch) error {
	switch data := parsed.Data.(type) {
	case profiles.ShadowsocksData:
		if err := applyShadowsocksPatch(parsed, &data, patch.Shadowsocks); err != nil {
			return err
		}
		parsed.Data = data
	case profiles.Hysteria2Data:
		if err := applyHysteria2Patch(&data, patch.Hysteria2); err != nil {
			return err
		}
		parsed.Data = data
	case profiles.TUICData:
		if err := applyTUICPatch(&data, patch.TUIC); err != nil {
			return err
		}
		parsed.Data = data
	}

	return nil
}

func applyShadowsocksPatch(parsed *profiles.Profile, data *profiles.ShadowsocksData, p *model.ShadowsocksStructuredPatch) error {
	if p == nil {
		return nil
	}
	data.Method = setString(p.Method, data.Method)
	if err := patchRequired(p.Password, &data.Password, "password", "shadowsocks"); err != nil {
		return err
	}
	applyShadowsocksPluginName(parsed, data, p.PluginName)
	if data.Plugin != nil {
		patchSensitive(p.PluginOptions, &data.Plugin.Options)
	}
	return nil
}

func applyShadowsocksPluginName(parsed *profiles.Profile, data *profiles.ShadowsocksData, field *model.TriStatePatch[string]) {
	if field == nil || !field.Set {
		return
	}
	if field.Operation == model.TriStateClear {
		data.Plugin = nil
		filteredQP := make([]profiles.QueryParameter, 0, len(parsed.QueryParameters))
		for _, qp := range parsed.QueryParameters {
			if strings.ToLower(qp.Key) != "plugin" {
				filteredQP = append(filteredQP, qp)
			}
		}
		parsed.QueryParameters = filteredQP
		return
	}
	if field.Value == "" {
		return
	}
	if data.Plugin == nil {
		data.Plugin = &profiles.ShadowsocksPlugin{Name: field.Value}
		return
	}
	data.Plugin.Name = field.Value
}

func applyHysteria2Patch(data *profiles.Hysteria2Data, p *model.Hysteria2StructuredPatch) error {
	if p == nil {
		return nil
	}
	if err := patchRequired(p.Authentication, &data.Authentication, "authentication", "hysteria2"); err != nil {
		return err
	}
	patchField(p.SNI, &data.SNI, "")
	patchBool(p.Insecure, &data.Insecure)
	patchField(p.CertificateSHA256, &data.CertificateSHA256, "")
	patchField(p.ObfuscationType, &data.ObfuscationType, "")
	patchSensitive(p.ObfuscationPassword, &data.ObfuscationPassword)
	return nil
}

func applyTUICPatch(data *profiles.TUICData, p *model.TUICStructuredPatch) error {
	if p == nil {
		return nil
	}
	if err := patchRequired(p.UUID, &data.UUID, "uuid", "tuic"); err != nil {
		return err
	}
	if err := patchRequired(p.Password, &data.Password, "password", "tuic"); err != nil {
		return err
	}
	patchField(p.SNI, &data.SNI, "")
	patchField(p.ALPN, &data.ALPN, []string{})
	patchBool(p.SkipCertVerify, &data.SkipCertificateVerification)
	patchField(p.CongestionController, &data.CongestionController, "")
	patchField(p.UDPRelayMode, &data.UDPRelayMode, "")
	patchBool(p.UDPOverStream, &data.UDPOverStream)
	patchBool(p.ZeroRTT, &data.ZeroRTT)
	patchField(p.Heartbeat, &data.Heartbeat, "")
	return nil
}

// isSet reports whether the field carries an explicit "set" operation.
func isSet[T any](field *model.TriStatePatch[T]) bool {
	return field != nil && field.Set && field.Operation == model.TriStateSet
}

// patchField writes the patched value into dst: "clear" stores cleared, any
// other present operation stores the value. Reports whether dst was touched.
func patchField[T any](field *model.TriStatePatch[T], dst *T, cleared T) bool {
	if field == nil || !field.Set {
		return false
	}
	if field.Operation == model.TriStateClear {
		*dst = cleared
	} else {
		*dst = field.Value
	}
	return true
}

// patchBool stores the patched value whenever the field is present, whatever
// the operation says.
func patchBool(field *model.TriStatePatch[bool], dst *bool) {
	if field != nil && field.Set {
		*dst = field.Value
	}
}

func patchSensitive(field *model.TriStatePatch[string], dst *profiles.SensitiveValue) {
	var value string
	if patchField(field, &value, "") {
		*dst = profiles.NewSensitiveValue(value)
	}
}

// patchRequired is patchSensitive for fields a real profile cannot live without.
func patchRequired(field *model.TriStatePatch[string], dst *profiles.SensitiveValue, what, proto string) error {
	if field == nil || !field.Set {
		return nil
	}
	if field.Operation == model.TriStateClear {
		return fmt.Errorf("%s cannot be cleared for real %s profile", what, proto)
	}
	*dst = profiles.NewSensitiveValue(field.Value)
	return nil
}

func (s *Service) UpdateLocal(ctx context.Context, params UpdateLocalParams) (*model.KeyProfileDetailResponse, error) {
	key, decryptedURI, fetchErr := s.repo.GetByID(ctx, params.ID)
	if fetchErr != nil {
		return nil, fetchErr
	}

	if key.ProfileRevision != params.ProfileRevision {
		return nil, ErrProfileRevisionConflict
	}
	if key.ExternalSourceID > 0 {
		return s.updateSourceOwnedMetadata(ctx, key, decryptedURI, params)
	}

	command, err := planLocalProfileUpdate(key, decryptedURI, params)
	if err != nil {
		return nil, err
	}
	updatedKey, updatedURI, err := s.repo.UpdateLocal(ctx, command)
	if err != nil {
		return nil, err
	}

	detail := BuildKeyProfileDetailResponse(*updatedKey, updatedURI, s.capabilityResolver)
	return &detail, nil
}

func (s *Service) updateSourceOwnedMetadata(ctx context.Context, key *model.VLESSKey, decryptedURI string, params UpdateLocalParams) (*model.KeyProfileDetailResponse, error) {
	command, err := planSourceMetadataUpdate(key, params)
	if err != nil {
		return nil, err
	}
	updatedKey, updatedURI, err := s.repo.UpdateSourceOwnedMetadata(ctx, command)
	if err != nil {
		return nil, err
	}
	if updatedURI == "" {
		updatedURI = decryptedURI
	}
	detail := BuildKeyProfileDetailResponse(*updatedKey, updatedURI, s.capabilityResolver)
	return &detail, nil
}
