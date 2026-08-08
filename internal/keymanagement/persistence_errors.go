package keymanagement

import (
	"errors"
	"fmt"

	"github.com/romanpodg/SubShare-Go/internal/keypersistence"
)

func mapKeyPersistenceError(err error) error {
	if err == nil {
		return nil
	}
	var missing keypersistence.KeyNotFoundError
	if errors.As(err, &missing) {
		return KeyNotFoundError{ID: missing.ID}
	}
	var target error
	switch {
	case errors.Is(err, keypersistence.ErrKeyNotFound):
		target = ErrKeyNotFound
	case errors.Is(err, keypersistence.ErrCategoryNotFound):
		target = ErrCategoryNotFound
	case errors.Is(err, keypersistence.ErrCredentialMissing):
		target = ErrCredentialMissing
	case errors.Is(err, keypersistence.ErrStorageIntegrity):
		target = ErrStorageIntegrity
	case errors.Is(err, keypersistence.ErrEncryptionUnavailable):
		target = ErrEncryptionUnavailable
	case errors.Is(err, keypersistence.ErrBlindIndexUnavailable):
		target = ErrBlindIndexUnavailable
	case errors.Is(err, keypersistence.ErrCategoryPersistence):
		target = ErrCategoryPersistence
	case errors.Is(err, keypersistence.ErrKeyOrderPersistence):
		target = ErrKeyOrderPersistence
	case errors.Is(err, keypersistence.ErrKeyCreateConflict):
		target = ErrKeyCreateConflict
	case errors.Is(err, keypersistence.ErrCredentialEncryption):
		target = ErrCredentialEncryption
	default:
		return err
	}
	return fmt.Errorf("%w: %v", target, err)
}
