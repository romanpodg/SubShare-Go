package profilestorage

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

const (
	EnvelopeMarker     = "subshare-profile"
	EnvelopeVersion    = "v1"
	EnvelopeAlgorithm  = "xchacha20poly1305"
	MaxPlaintextBytes  = 64 * 1024
	NonceSize          = 24
	AEADOverhead       = 16
	MaxCiphertextBytes = MaxPlaintextBytes + AEADOverhead
)

// BuildAAD constructs deterministic length-prefixed associated data bound to the immutable rowID.
func BuildAAD(version, algo, keyID string, rowID int64) []byte {
	app := "subshare"
	domain := "profilestorage"
	entity := "vless_keys"
	field := "encrypted_url"

	buf := make([]byte, 0, 256)

	appendString := func(s string) {
		l := uint16(len(s))
		buf = binary.BigEndian.AppendUint16(buf, l)
		buf = append(buf, s...)
	}

	appendString(app)
	appendString(domain)
	appendString(entity)
	appendString(field)
	appendString(version)
	appendString(algo)
	appendString(keyID)

	buf = binary.BigEndian.AppendUint64(buf, uint64(rowID))
	return buf
}

// Encrypt encrypts raw URI bytes using XChaCha20-Poly1305 and returns a formatted envelope string.
func Encrypt(plaintext []byte, keyID string, key []byte, rowID int64) (string, error) {
	if len(plaintext) > MaxPlaintextBytes {
		return "", fmt.Errorf("%w: plaintext exceeds maximum size of %d bytes", ErrEncryptionFailed, MaxPlaintextBytes)
	}
	if len(key) != 32 {
		return "", fmt.Errorf("%w: key must be 32 bytes", ErrEncryptionFailed)
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("%w: failed to generate nonce: %v", ErrEncryptionFailed, err)
	}

	aad := BuildAAD(EnvelopeVersion, EnvelopeAlgorithm, keyID, rowID)
	ciphertext := aead.Seal(nil, nonce, plaintext, aad)

	nonceB64 := base64.RawURLEncoding.EncodeToString(nonce)
	cipherB64 := base64.RawURLEncoding.EncodeToString(ciphertext)

	envelope := fmt.Sprintf("$%s$%s$%s$%s$%s$%s",
		EnvelopeMarker, EnvelopeVersion, EnvelopeAlgorithm, keyID, nonceB64, cipherB64,
	)
	return envelope, nil
}

type ParsedEnvelope struct {
	Marker     string
	Version    string
	Algorithm  string
	KeyID      string
	Nonce      []byte
	Ciphertext []byte
}

func ParseEnvelope(envelope string) (*ParsedEnvelope, error) {
	if len(envelope) > 120000 {
		return nil, fmt.Errorf("%w: envelope exceeds size limit", ErrMalformedEnvelope)
	}
	parts := splitExact(envelope, '$')
	if len(parts) != 7 || parts[0] != "" {
		return nil, fmt.Errorf("%w: invalid envelope structure", ErrMalformedEnvelope)
	}

	marker, version, algo, keyID, nonceB64, cipherB64 := parts[1], parts[2], parts[3], parts[4], parts[5], parts[6]

	if marker != EnvelopeMarker {
		return nil, fmt.Errorf("%w: unrecognized marker %q", ErrMalformedEnvelope, marker)
	}
	if version != EnvelopeVersion {
		return nil, fmt.Errorf("%w: unsupported version %q", ErrUnsupportedVersion, version)
	}
	if algo != EnvelopeAlgorithm {
		return nil, fmt.Errorf("%w: unsupported algorithm %q", ErrMalformedEnvelope, algo)
	}
	if !validKeyIDRegex.MatchString(keyID) {
		return nil, fmt.Errorf("%w: invalid key ID %q", ErrMalformedEnvelope, keyID)
	}

	nonce, err := base64.RawURLEncoding.DecodeString(nonceB64)
	if err != nil || len(nonce) != NonceSize {
		return nil, fmt.Errorf("%w: invalid nonce encoding or length", ErrMalformedEnvelope)
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(cipherB64)
	if err != nil || len(ciphertext) < AEADOverhead || len(ciphertext) > MaxCiphertextBytes {
		return nil, fmt.Errorf("%w: invalid ciphertext encoding or length", ErrMalformedEnvelope)
	}

	return &ParsedEnvelope{
		Marker:     marker,
		Version:    version,
		Algorithm:  algo,
		KeyID:      keyID,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}, nil
}

// Decrypt parses the envelope and authenticates/decrypts the payload bound to rowID.
func Decrypt(envelope string, keyring *Keyring, rowID int64) (SecretProfileURI, error) {
	if keyring == nil {
		return SecretProfileURI{}, ErrMissingKeyring
	}

	parsed, err := ParseEnvelope(envelope)
	if err != nil {
		return SecretProfileURI{}, err
	}

	key, ok := keyring.GetEncryptionKey(parsed.KeyID)
	if !ok {
		return SecretProfileURI{}, fmt.Errorf("%w: key ID %q not in keyring", ErrUnknownKeyID, parsed.KeyID)
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return SecretProfileURI{}, fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}

	aad := BuildAAD(parsed.Version, parsed.Algorithm, parsed.KeyID, rowID)
	plaintext, err := aead.Open(nil, parsed.Nonce, parsed.Ciphertext, aad)
	if err != nil {
		return SecretProfileURI{}, fmt.Errorf("%w: %v", ErrAuthenticationFailed, err)
	}

	return NewSecretProfileURIFromBytes(plaintext), nil
}

// InspectEnvelope extracts metadata without attempting decryption or requiring key material.
func InspectEnvelope(envelope string) (keyID, version, algo string, err error) {
	parsed, err := ParseEnvelope(envelope)
	if err != nil {
		return "", "", "", err
	}
	return parsed.KeyID, parsed.Version, parsed.Algorithm, nil
}

// NeedsReEncryption returns true if the envelope's key ID differs from the active key ID.
func NeedsReEncryption(envelope string, activeKeyID string) bool {
	parsed, err := ParseEnvelope(envelope)
	if err != nil {
		return false
	}
	return parsed.KeyID != activeKeyID
}

// ReEncrypt decrypts an existing envelope and re-encrypts it under the active key in the keyring with a fresh nonce.
func ReEncrypt(envelope string, keyring *Keyring, rowID int64) (string, error) {
	secret, err := Decrypt(envelope, keyring, rowID)
	if err != nil {
		return "", err
	}

	activeID, activeKey, err := keyring.GetActiveEncryptionKey()
	if err != nil {
		return "", err
	}

	return Encrypt(secret.data, activeID, activeKey, rowID)
}

// ComputeBlindIndex calculates HMAC-SHA256(bik, "subshare|vless_keys|url_blind_index|" + rawURL).
func ComputeBlindIndex(bik []byte, rawURL string) string {
	h := hmac.New(sha256.New, bik)
	_, _ = h.Write([]byte("subshare|vless_keys|url_blind_index|"))
	_, _ = h.Write([]byte(rawURL))
	return hex.EncodeToString(h.Sum(nil))
}

func splitExact(s string, sep byte) []string {
	var n int
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			n++
		}
	}
	res := make([]string, 0, n+1)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			res = append(res, s[start:i])
			start = i + 1
		}
	}
	res = append(res, s[start:])
	return res
}
