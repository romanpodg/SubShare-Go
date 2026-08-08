package storage

import (
	"errors"
	"fmt"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

type credentialStore struct {
	keyring *profilestorage.Keyring
}

var errCredentialMissing = errors.New("missing encrypted credential")

func newCredentialStore(keyring *profilestorage.Keyring) *credentialStore {
	return &credentialStore{keyring: keyring}
}

func (s *credentialStore) encryptionAvailable() error {
	if s == nil || s.keyring == nil {
		return profilestorage.ErrMissingKeyring
	}
	_, _, err := s.keyring.GetActiveEncryptionKey()
	return err
}

func (s *credentialStore) blindIndex(raw string) (string, error) {
	if s == nil || s.keyring == nil {
		return "", profilestorage.ErrMissingKeyring
	}
	_, key, err := s.keyring.GetActiveBlindIndexKey()
	if err != nil {
		return "", err
	}
	return profilestorage.ComputeBlindIndex(key, raw), nil
}

func (s *credentialStore) encrypt(raw string, rowID int64) (string, error) {
	if s == nil || s.keyring == nil {
		return "", profilestorage.ErrMissingKeyring
	}
	keyID, key, err := s.keyring.GetActiveEncryptionKey()
	if err != nil {
		return "", err
	}
	return profilestorage.Encrypt([]byte(raw), keyID, key, rowID)
}

func (s *credentialStore) decrypt(envelope string, rowID int64) (string, error) {
	if envelope == "" {
		return "", fmt.Errorf("%w: %w", errCredentialMissing, profilestorage.ErrMigrationVerificationFailed)
	}
	if s == nil {
		return "", profilestorage.ErrMissingKeyring
	}
	secret, err := profilestorage.Decrypt(envelope, s.keyring, rowID)
	if err != nil {
		return "", err
	}
	return secret.Reveal(), nil
}

func credentialKeyUnavailable(err error) bool {
	return errors.Is(err, profilestorage.ErrMissingKeyring) || errors.Is(err, profilestorage.ErrUnknownKeyID)
}
