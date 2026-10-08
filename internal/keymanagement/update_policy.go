package keymanagement

import (
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

// planLocalProfileUpdate prepares the existing persistence command without I/O.
// Metadata errors precede mode/configuration errors, as in the HTTP contract.
func planLocalProfileUpdate(key *model.VLESSKey, currentURI string, params UpdateLocalParams) (UpdateProfileParams, error) {
	command, err := normalizeLocalUpdateMetadata(params)
	if err != nil {
		return UpdateProfileParams{}, err
	}
	params.PatchMode, err = resolvePatchMode(params)
	if err != nil {
		return UpdateProfileParams{}, err
	}
	command.NewURI = currentURI
	if command.Kind == model.KeyKindReal {
		command.NewURI, err = resolveUpdatedURI(key, currentURI, command, params)
		if err != nil {
			return UpdateProfileParams{}, err
		}
	}
	command.Protocol = "legacy"
	if storedProtocol, _, validationErr := validateStoredConfiguration(command.NewURI); validationErr == nil {
		command.Protocol = storedProtocol
	}
	return command, nil
}

func normalizeLocalUpdateMetadata(params UpdateLocalParams) (UpdateProfileParams, error) {
	label := strings.TrimSpace(params.Label)
	name, err := normalizeUpdateDisplayName(params.ClientDisplayName, label)
	if err != nil {
		return UpdateProfileParams{}, err
	}
	status, statusOK := model.NormalizeKeyStatus(params.Status)
	kind, kindOK := model.NormalizeKeyKind(params.Kind)
	if !statusOK {
		return UpdateProfileParams{}, ErrInvalidInput
	}
	if !kindOK {
		return UpdateProfileParams{}, ErrInvalidInput
	}
	if label == "" {
		return UpdateProfileParams{}, ErrInvalidInput
	}
	if len(label) > 255 {
		return UpdateProfileParams{}, ErrLabelTooLong
	}
	return UpdateProfileParams{
		ID: params.ID, ExpectedRevision: params.ProfileRevision,
		Label: label, ClientDisplayName: name, Status: status, Kind: kind,
		Category: params.Category, CategoryID: params.CategoryID,
		TemplateText: strings.TrimSpace(params.TemplateText),
	}, nil
}

// Empty local names reset to the label; source names reset to the empty
// override. The caller supplies that fallback before the shared length check.
func normalizeUpdateDisplayName(value *string, fallback string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	name := strings.TrimSpace(*value)
	if name == "" {
		name = fallback
	}
	if len(name) > 255 {
		return nil, ErrLabelTooLong
	}
	return &name, nil
}

// resolvePatchMode retains the distinction between absent raw input and raw
// input containing only whitespace when selecting the implicit mode.
func resolvePatchMode(params UpdateLocalParams) (string, error) {
	mode := defaultUpdatePatchMode(params)
	switch mode {
	case "raw":
		if params.StructuredPatch != nil {
			return "", ErrMutuallyExclusiveMode
		}
	case "structured":
		if strings.TrimSpace(params.RawURI) != "" {
			return "", ErrMutuallyExclusiveMode
		}
	default:
		return "", ErrInvalidPatchMode
	}
	return mode, nil
}

func defaultUpdatePatchMode(params UpdateLocalParams) string {
	mode := strings.ToLower(strings.TrimSpace(params.PatchMode))
	if mode != "" {
		return mode
	}
	if params.RawURI != "" {
		return "raw"
	}
	return "structured"
}

func resolveUpdatedURI(key *model.VLESSKey, currentURI string, command UpdateProfileParams, params UpdateLocalParams) (string, error) {
	if params.PatchMode == "raw" {
		newURI := strings.TrimSpace(params.RawURI)
		if newURI == "" {
			return "", ErrRawURIRequired
		}
		if _, _, parseErr := validateStoredConfiguration(newURI); parseErr != nil {
			return "", invalidProfileURIError(parseErr)
		}
		return newURI, nil
	}
	if params.StructuredPatch == nil {
		return "", ErrStructuredPatchRequired
	}
	return ApplyStructuredPatchToURI(key.Protocol, currentURI, command.Label, params.StructuredPatch)
}

// Source-owned requests must be an exact metadata-only projection. Rejection
// precedes display-name validation; the adapter cannot change source columns.
func planSourceMetadataUpdate(key *model.VLESSKey, params UpdateLocalParams) (UpdateSourceOwnedMetadataParams, error) {
	status, statusOK := model.NormalizeKeyStatus(params.Status)
	if !statusOK {
		return UpdateSourceOwnedMetadataParams{}, ErrSourceOwnedReadOnly
	}
	if !sourceUpdateIdentityMatches(key, params) {
		return UpdateSourceOwnedMetadataParams{}, ErrSourceOwnedReadOnly
	}
	if !sourceUpdateCategoryMatches(key, params) {
		return UpdateSourceOwnedMetadataParams{}, ErrSourceOwnedReadOnly
	}
	if !sourceUpdatePayloadIsMetadataOnly(params) {
		return UpdateSourceOwnedMetadataParams{}, ErrSourceOwnedReadOnly
	}
	name, err := normalizeUpdateDisplayName(params.ClientDisplayName, "")
	if err != nil {
		return UpdateSourceOwnedMetadataParams{}, err
	}
	return UpdateSourceOwnedMetadataParams{
		ID: params.ID, ExpectedRevision: params.ProfileRevision,
		Status: status, ClientDisplayName: name,
	}, nil
}

func sourceUpdateIdentityMatches(key *model.VLESSKey, params UpdateLocalParams) bool {
	if strings.TrimSpace(params.Label) != strings.TrimSpace(key.Label) {
		return false
	}
	kind, kindOK := model.NormalizeKeyKind(params.Kind)
	if !kindOK {
		return false
	}
	if kind != key.Kind {
		return false
	}
	return strings.TrimSpace(params.TemplateText) == strings.TrimSpace(key.TemplateText)
}

func sourceUpdateCategoryMatches(key *model.VLESSKey, params UpdateLocalParams) bool {
	if strings.TrimSpace(params.Category) != strings.TrimSpace(key.Category) {
		return false
	}
	if params.CategoryID == nil {
		return true
	}
	return key.CategoryID > 0 && *params.CategoryID == key.CategoryID
}

func sourceUpdatePayloadIsMetadataOnly(params UpdateLocalParams) bool {
	if strings.TrimSpace(params.RawURI) != "" {
		return false
	}
	if params.StructuredPatch != nil {
		return false
	}
	mode := strings.ToLower(strings.TrimSpace(params.PatchMode))
	return mode == "" || mode == "structured"
}
