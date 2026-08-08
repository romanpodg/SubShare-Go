package profilestorage

import "errors"

var (
	ErrMissingKeyring              = errors.New("profile_storage_integrity_error: missing keyring")
	ErrUnknownKeyID                = errors.New("profile_storage_integrity_error: unknown key id")
	ErrMalformedEnvelope           = errors.New("profile_storage_integrity_error: malformed envelope")
	ErrUnsupportedVersion          = errors.New("profile_storage_integrity_error: unsupported envelope version")
	ErrAuthenticationFailed        = errors.New("profile_storage_integrity_error: authentication failed")
	ErrEncryptionFailed            = errors.New("profile_storage_integrity_error: encryption failed")
	ErrMigrationIncomplete         = errors.New("profile_storage_integrity_error: migration incomplete")
	ErrMigrationVerificationFailed = errors.New("profile_storage_integrity_error: migration verification failed")
	ErrKeyReferenced               = errors.New("profile_storage_integrity_error: key still referenced by rows")
	ErrPlaintextDisallowed         = errors.New("profile_storage_integrity_error: plaintext profile URI disallowed")
	ErrMissingBlindIndexMetadata   = errors.New("profile_storage_integrity_error: missing blind index metadata")
	ErrBlindIndexKeyMismatch       = errors.New("profile_storage_integrity_error: blind index key mismatch")
)
