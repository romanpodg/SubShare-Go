package keypersistence

import (
	"errors"
	"fmt"
)

var (
	ErrKeyNotFound           = errors.New("key not found")
	ErrCategoryNotFound      = errors.New("key category not found")
	ErrStorageIntegrity      = errors.New("key storage integrity error")
	ErrEncryptionUnavailable = errors.New("encryption key unavailable")
	ErrBlindIndexUnavailable = errors.New("blind index key unavailable")
	ErrCategoryPersistence   = errors.New("failed to save key category")
	ErrKeyOrderPersistence   = errors.New("failed to prepare key order")
	ErrKeyCreateConflict     = errors.New("failed to create key")
	ErrCredentialEncryption  = errors.New("failed to encrypt key")
	ErrInvalidKeyOrderCount  = errors.New("ids list must include all keys")
	ErrUnknownKeyInOrder     = errors.New("ids list contains unknown key")
	ErrDuplicateKeyInOrder   = errors.New("ids list contains duplicates")
)

var ErrCredentialMissing = fmt.Errorf("%w: missing encrypted credential", ErrStorageIntegrity)

type KeyNotFoundError struct {
	ID int64
}

func (e KeyNotFoundError) Error() string {
	return fmt.Sprintf("key not found: %d", e.ID)
}

func (e KeyNotFoundError) Unwrap() error {
	return ErrKeyNotFound
}
