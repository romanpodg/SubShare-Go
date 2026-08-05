package profilepersistence

import "errors"

var (
	ErrProfileNotFound         = errors.New("profile not found")
	ErrSourceOwnedProfile      = errors.New("profile is source-owned and read-only")
	ErrProfileRevisionConflict = errors.New("profile revision conflict")
	ErrStorageIntegrity        = errors.New("profile storage integrity error")
	ErrEncryptionUnavailable   = errors.New("encryption key unavailable")
	ErrBlindIndexUnavailable   = errors.New("blind index key unavailable")
)
