package keymanagement

import (
	"errors"

	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
)

var (
	ErrKeyNotFound                      = profilepersistence.ErrProfileNotFound
	ErrProfileRevisionConflict          = profilepersistence.ErrProfileRevisionConflict
	ErrSourceOwnedReadOnly              = profilepersistence.ErrSourceOwnedProfile
	ErrStorageIntegrity                 = profilepersistence.ErrStorageIntegrity
	ErrEncryptionUnavailable            = profilepersistence.ErrEncryptionUnavailable
	ErrInvalidInput                     = errors.New("label, valid status and kind are required")
	ErrLabelTooLong                     = errors.New("label is too long")
	ErrRawURIRequired                   = errors.New("raw_uri is required for real key in raw mode")
	ErrStructuredPayloadRequired        = errors.New("structured payload is required")
	ErrStructuredPatchRequired          = errors.New("structured_patch is required")
	ErrMutuallyExclusiveMode            = errors.New("cannot specify mutually exclusive raw and structured modes")
	ErrInvalidCreationMode              = errors.New("creation_mode must be raw or structured")
	ErrInvalidPatchMode                 = errors.New("patch_mode must be raw or structured")
	ErrTUICv4StructuredForbidden        = errors.New("TUIC v4 is compatibility-only; structured creation is unsupported")
	ErrInformationalStructuredForbidden = errors.New("informational keys do not support structured profile creation")
	ErrInvalidTarget                    = errors.New("target must be raw or structured-secrets")
	ErrInvalidProfileURI                = errors.New("invalid profile uri")
	ErrInvalidKeyKind                   = errors.New("invalid key kind")
	ErrInvalidKeyStatus                 = errors.New("invalid key status")
	ErrLabelRequired                    = errors.New("label is required")
	ErrURLRequired                      = errors.New("url is required for real keys")
	ErrConfigurationTooLong             = errors.New("configuration is too long (max 65535 characters)")
	ErrTemplateTextTooLong              = errors.New("template_text is too long (max 8192 characters)")
)
