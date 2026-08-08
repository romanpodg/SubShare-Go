package keymanagement

import (
	"errors"
	"fmt"

	"github.com/romanpodg/SubShare-Go/internal/keypersistence"
	"github.com/romanpodg/SubShare-Go/internal/profilepersistence"
)

type KeyNotFoundError struct {
	ID int64
}

func (e KeyNotFoundError) Error() string {
	return fmt.Sprintf("key not found: %d", e.ID)
}

func (e KeyNotFoundError) Unwrap() error {
	return ErrKeyNotFound
}

var (
	ErrKeyNotFound                      = profilepersistence.ErrProfileNotFound
	ErrProfileRevisionConflict          = profilepersistence.ErrProfileRevisionConflict
	ErrSourceOwnedReadOnly              = profilepersistence.ErrSourceOwnedProfile
	ErrStorageIntegrity                 = profilepersistence.ErrStorageIntegrity
	ErrEncryptionUnavailable            = profilepersistence.ErrEncryptionUnavailable
	ErrBlindIndexUnavailable            = errors.New("blind index key unavailable")
	ErrProfileCreateConflict            = profilepersistence.ErrProfileCreateConflict
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
	ErrCategoryNameRequired             = errors.New("category name is required")
	ErrCategoryNameEmpty                = errors.New("category name cannot be empty")
	ErrCategoryOldAndNewRequired        = errors.New("both old_name and new_name are required")
	ErrInvalidDeleteMode                = errors.New("invalid delete mode")
	ErrCategoryNamesRequired            = errors.New("category names are required")
	ErrDuplicateCategoryNames           = errors.New("duplicate category names")
	ErrCategoryNotFound                 = errors.New("key category not found")
	ErrCategoryPersistence              = errors.New("failed to save key category")
	ErrKeyOrderPersistence              = errors.New("failed to prepare key order")
	ErrKeyCreateConflict                = errors.New("failed to create key")
	ErrCredentialEncryption             = errors.New("failed to encrypt key")
	ErrBulkKeyIDsRequired               = errors.New("ids list is empty")
	ErrInvalidBulkKeyID                 = errors.New("ids list contains invalid key id")
	ErrDuplicateBulkKeyIDs              = errors.New("ids list contains duplicates")
	ErrBulkCategoryPersistence          = errors.New("failed to persist bulk key category")
	ErrInformationalHealthCheck         = errors.New("informational keys do not require checks")
	ErrInvalidKeyOrderCount             = keypersistence.ErrInvalidKeyOrderCount
	ErrUnknownKeyInOrder                = keypersistence.ErrUnknownKeyInOrder
	ErrDuplicateKeyInOrder              = keypersistence.ErrDuplicateKeyInOrder
)

var ErrCredentialMissing = fmt.Errorf("%w: missing encrypted credential", ErrStorageIntegrity)
