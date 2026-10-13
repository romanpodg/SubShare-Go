package keymanagement

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func generateToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Service) CreateLocal(ctx context.Context, params CreateLocalParams) (*model.KeyProfileDetailResponse, error) {
	normalized, err := normalizeLocalCreation(params)
	if err != nil {
		return nil, err
	}
	label, clientDisplayName, status, kind, templateText := normalized.Label, *normalized.ClientDisplayName, normalized.Status, normalized.Kind, normalized.TemplateText

	creationMode, err := resolveCreationMode(params)
	if err != nil {
		return nil, err
	}

	builtURI := ""
	if kind == model.KeyKindReal {
		builtURI, err = buildRealProfileURI(creationMode, label, params)
	} else {
		builtURI, templateText, err = buildInformationalProfile(creationMode, label, templateText, params)
	}
	if err != nil {
		return nil, err
	}

	storedProtocol := "legacy"
	if protocol, _, validationErr := validateStoredConfiguration(builtURI); validationErr == nil {
		storedProtocol = protocol
	}

	createdKey, decryptedURI, err := s.repo.CreateLocal(ctx, CreateProfileParams{
		Label:             label,
		ClientDisplayName: clientDisplayName,
		Status:            status,
		Kind:              kind,
		Category:          params.Category,
		CategoryID:        params.CategoryID,
		TemplateText:      templateText,
		Protocol:          storedProtocol,
		BuiltURI:          builtURI,
	})
	if err != nil {
		return nil, err
	}

	detail := BuildKeyProfileDetailResponse(*createdKey, decryptedURI, s.capabilityResolver)
	return &detail, nil
}

// resolveCreationMode normalises the requested creation mode ("raw" or
// "structured"), defaulting from the payload shape, and rejects mixed payloads.
func resolveCreationMode(params CreateLocalParams) (string, error) {
	creationMode := strings.ToLower(strings.TrimSpace(params.CreationMode))
	if creationMode == "" {
		if params.RawURI != "" {
			creationMode = "raw"
		} else {
			creationMode = "structured"
		}
	}
	if creationMode != "raw" && creationMode != "structured" {
		return "", ErrInvalidCreationMode
	}
	if creationMode == "raw" && params.Structured != nil {
		return "", ErrMutuallyExclusiveMode
	}
	if creationMode == "structured" && strings.TrimSpace(params.RawURI) != "" {
		return "", ErrMutuallyExclusiveMode
	}
	return creationMode, nil
}

// buildRealProfileURI produces the stored configuration for a real profile
// from either the raw URI or the structured payload.
func buildRealProfileURI(creationMode, label string, params CreateLocalParams) (string, error) {
	if creationMode == "raw" {
		builtURI := strings.TrimSpace(params.RawURI)
		if builtURI == "" {
			return "", ErrRawURIRequired
		}
		if _, _, parseErr := validateStoredConfiguration(builtURI); parseErr != nil {
			return "", invalidProfileURIError(parseErr)
		}
		return builtURI, nil
	}
	proto := strings.ToLower(strings.TrimSpace(params.Protocol))
	if proto == "tuic_v4" {
		return "", ErrTUICv4StructuredForbidden
	}
	if params.Structured == nil {
		return "", ErrStructuredPayloadRequired
	}
	return BuildURIFromStructuredCreate(proto, label, params.Structured)
}

// buildInformationalProfile returns the placeholder URI and effective template
// text for an informational profile.
func buildInformationalProfile(creationMode, label, templateText string, params CreateLocalParams) (string, string, error) {
	if creationMode == "structured" && params.Structured != nil {
		return "", templateText, ErrInformationalStructuredForbidden
	}
	if templateText == "" {
		templateText = label
	}
	token, err := generateToken(12)
	if err != nil {
		return "", templateText, fmt.Errorf("failed to generate key token: %w", err)
	}
	return "info://" + token, templateText, nil
}

func creationClientName(value *string, label string) string {
	if value == nil {
		return ""
	}
	name := strings.TrimSpace(*value)
	if name == "" {
		return label
	}
	return name
}

func validCreationMetadata(statusOK, kindOK bool, label string) bool {
	if !statusOK {
		return false
	}
	if !kindOK {
		return false
	}
	return label != ""
}

func normalizeLocalCreation(params CreateLocalParams) (CreateLocalParams, error) {
	label := strings.TrimSpace(params.Label)
	clientDisplayName := creationClientName(params.ClientDisplayName, label)
	status, statusOK := model.NormalizeKeyStatus(params.Status)
	kind, kindOK := model.NormalizeKeyKind(params.Kind)
	templateText := strings.TrimSpace(params.TemplateText)

	if !validCreationMetadata(statusOK, kindOK, label) {
		return CreateLocalParams{}, ErrInvalidInput
	}
	if len(label) > 255 {
		return CreateLocalParams{}, ErrLabelTooLong
	}
	if len(clientDisplayName) > 255 {
		return CreateLocalParams{}, ErrLabelTooLong
	}

	params.Label, params.ClientDisplayName, params.Status, params.Kind, params.TemplateText = label, &clientDisplayName, status, kind, templateText
	return params, nil
}
