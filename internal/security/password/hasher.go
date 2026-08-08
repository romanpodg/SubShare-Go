package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

type Algorithm string

const (
	AlgorithmUnknown  Algorithm = "unknown"
	AlgorithmBcrypt   Algorithm = "bcrypt"
	AlgorithmArgon2id Algorithm = "argon2id"

	argon2Version = argon2.Version
)

var ErrInvalidEncodedHash = errors.New("password hash has an unsupported or malformed format")

type Parameters struct {
	MemoryKiB    uint32
	Iterations   uint32
	Parallelism  uint8
	SaltLength   uint32
	OutputLength uint32
}

var DefaultParameters = Parameters{
	MemoryKiB:    32 * 1024,
	Iterations:   3,
	Parallelism:  2,
	SaltLength:   16,
	OutputLength: 32,
}

// unknownCredentialHash is a synthetic verification target used to keep an
// unknown administrator on the same password-verification path as a known
// administrator. Its fixed salt is safe because it never protects a real
// credential and the encoded value is never stored or logged.
var unknownCredentialHash = encodeArgon2id(
	"synthetic unknown administrator credential",
	[]byte("subshare-no-user"),
	DefaultParameters,
)

type Verification struct {
	Algorithm   Algorithm
	Valid       bool
	NeedsRehash bool
}

type Hasher struct {
	parameters Parameters
	random     io.Reader
}

func New(parameters Parameters, random io.Reader) (*Hasher, error) {
	if err := validateParameters(parameters); err != nil {
		return nil, err
	}
	if random == nil {
		random = rand.Reader
	}
	return &Hasher{parameters: parameters, random: random}, nil
}

func NewDefault() *Hasher {
	hasher, err := New(DefaultParameters, rand.Reader)
	if err != nil {
		panic("invalid built-in Argon2id parameters")
	}
	return hasher
}

func (h *Hasher) Parameters() Parameters {
	return h.parameters
}

func Identify(encoded string) Algorithm {
	switch {
	case strings.HasPrefix(encoded, "$argon2id$"):
		return AlgorithmArgon2id
	case strings.HasPrefix(encoded, "$2a$"), strings.HasPrefix(encoded, "$2b$"), strings.HasPrefix(encoded, "$2y$"):
		return AlgorithmBcrypt
	default:
		return AlgorithmUnknown
	}
}

func (h *Hasher) Hash(value string) (string, error) {
	if err := Validate(value); err != nil {
		return "", err
	}
	return h.hash(value)
}

// RehashVerified upgrades a password that has already been successfully
// verified against a stored hash. It deliberately does not apply the policy
// for new passwords, so legacy administrators are not locked out merely
// because their existing password is shorter than today's minimum.
func (h *Hasher) RehashVerified(value string) (string, error) {
	if !utf8SafeForVerification(value) {
		return "", errors.New("verified password exceeds the defensive hashing limit")
	}
	return h.hash(value)
}

func (h *Hasher) hash(value string) (string, error) {
	salt := make([]byte, h.parameters.SaltLength)
	if _, err := io.ReadFull(h.random, salt); err != nil {
		return "", fmt.Errorf("generate password salt")
	}
	return encodeArgon2id(value, salt, h.parameters), nil
}

func encodeArgon2id(value string, salt []byte, parameters Parameters) string {
	digest := argon2.IDKey(
		[]byte(value),
		salt,
		parameters.Iterations,
		parameters.MemoryKiB,
		parameters.Parallelism,
		parameters.OutputLength,
	)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2Version,
		parameters.MemoryKiB,
		parameters.Iterations,
		parameters.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	)
}

// VerifyUnknown performs comparable-work credential checking without a real
// stored password hash. Callers deliberately ignore its result.
func (h *Hasher) VerifyUnknown(value string) {
	_, _ = h.Verify(value, unknownCredentialHash)
}

func (h *Hasher) Verify(value, encoded string) (Verification, error) {
	algorithm := Identify(encoded)
	verification := Verification{Algorithm: algorithm}
	if !utf8SafeForVerification(value) {
		return verification, nil
	}

	switch algorithm {
	case AlgorithmBcrypt:
		if _, err := bcrypt.Cost([]byte(encoded)); err != nil {
			return verification, ErrInvalidEncodedHash
		}
		if err := bcrypt.CompareHashAndPassword([]byte(encoded), []byte(value)); err != nil {
			if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) || errors.Is(err, bcrypt.ErrPasswordTooLong) {
				return verification, nil
			}
			return verification, ErrInvalidEncodedHash
		}
		verification.Valid = true
		verification.NeedsRehash = true
		return verification, nil

	case AlgorithmArgon2id:
		parsed, err := parseArgon2id(encoded)
		if err != nil {
			return verification, ErrInvalidEncodedHash
		}
		digest := argon2.IDKey(
			[]byte(value), parsed.salt, parsed.parameters.Iterations,
			parsed.parameters.MemoryKiB, parsed.parameters.Parallelism,
			uint32(len(parsed.digest)),
		)
		verification.Valid = subtle.ConstantTimeCompare(digest, parsed.digest) == 1
		if verification.Valid {
			verification.NeedsRehash = !sameParameters(parsed.parameters, h.parameters)
		}
		return verification, nil

	default:
		return verification, ErrInvalidEncodedHash
	}
}

func (h *Hasher) NeedsRehash(encoded string) (bool, error) {
	switch Identify(encoded) {
	case AlgorithmBcrypt:
		if _, err := bcrypt.Cost([]byte(encoded)); err != nil {
			return false, ErrInvalidEncodedHash
		}
		return true, nil
	case AlgorithmArgon2id:
		parsed, err := parseArgon2id(encoded)
		if err != nil {
			return false, ErrInvalidEncodedHash
		}
		return !sameParameters(parsed.parameters, h.parameters), nil
	default:
		return false, ErrInvalidEncodedHash
	}
}

type parsedArgon2id struct {
	parameters Parameters
	salt       []byte
	digest     []byte
}

func parseArgon2id(encoded string) (parsedArgon2id, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return parsedArgon2id{}, ErrInvalidEncodedHash
	}
	parameterParts := strings.Split(parts[3], ",")
	if len(parameterParts) != 3 {
		return parsedArgon2id{}, ErrInvalidEncodedHash
	}
	memory, err := parseUintParameter(parameterParts[0], "m=", 32)
	if err != nil {
		return parsedArgon2id{}, ErrInvalidEncodedHash
	}
	iterations, err := parseUintParameter(parameterParts[1], "t=", 32)
	if err != nil {
		return parsedArgon2id{}, ErrInvalidEncodedHash
	}
	parallelism, err := parseUintParameter(parameterParts[2], "p=", 8)
	if err != nil {
		return parsedArgon2id{}, ErrInvalidEncodedHash
	}
	if len(parts[4]) > 128 || len(parts[5]) > 128 {
		return parsedArgon2id{}, ErrInvalidEncodedHash
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return parsedArgon2id{}, ErrInvalidEncodedHash
	}
	digest, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return parsedArgon2id{}, ErrInvalidEncodedHash
	}
	parameters := Parameters{
		MemoryKiB:    uint32(memory),
		Iterations:   uint32(iterations),
		Parallelism:  uint8(parallelism),
		SaltLength:   uint32(len(salt)),
		OutputLength: uint32(len(digest)),
	}
	if err := validateVerificationParameters(parameters); err != nil {
		return parsedArgon2id{}, ErrInvalidEncodedHash
	}
	return parsedArgon2id{parameters: parameters, salt: salt, digest: digest}, nil
}

func parseUintParameter(raw, prefix string, bits int) (uint64, error) {
	value, found := strings.CutPrefix(raw, prefix)
	if !found || value == "" {
		return 0, ErrInvalidEncodedHash
	}
	return strconv.ParseUint(value, 10, bits)
}

func validateParameters(parameters Parameters) error {
	if parameters.MemoryKiB < 8*1024 || parameters.MemoryKiB > 128*1024 ||
		parameters.Iterations < 1 || parameters.Iterations > 10 ||
		parameters.Parallelism < 1 || parameters.Parallelism > 16 ||
		parameters.SaltLength < 16 || parameters.SaltLength > 64 ||
		parameters.OutputLength < 16 || parameters.OutputLength > 64 {
		return fmt.Errorf("invalid Argon2id parameters")
	}
	return nil
}

func validateVerificationParameters(parameters Parameters) error {
	if parameters.MemoryKiB < 8*1024 || parameters.MemoryKiB > 128*1024 ||
		parameters.Iterations < 1 || parameters.Iterations > 10 ||
		parameters.Parallelism < 1 || parameters.Parallelism > 16 ||
		parameters.SaltLength < 8 || parameters.SaltLength > 64 ||
		parameters.OutputLength < 16 || parameters.OutputLength > 64 {
		return ErrInvalidEncodedHash
	}
	return nil
}

func sameParameters(left, right Parameters) bool {
	return left == right
}

func utf8SafeForVerification(value string) bool {
	return len(value) <= MaxEncodedBytes && strings.ToValidUTF8(value, "") == value
}
