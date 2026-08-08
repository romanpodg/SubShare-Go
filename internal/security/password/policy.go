package password

import (
	"errors"
	"unicode/utf8"
)

const (
	MinCodePoints   = 15
	MaxCodePoints   = 256
	MaxEncodedBytes = 1024
)

var (
	ErrInvalidUTF8  = errors.New("password must be valid UTF-8")
	ErrTooShort     = errors.New("password must contain at least 15 Unicode characters")
	ErrTooLong      = errors.New("password must contain at most 256 Unicode characters")
	ErrTooManyBytes = errors.New("password encoded length must not exceed 1024 bytes")
)

// Validate enforces the password policy without trimming, normalizing,
// transforming, or truncating the supplied value.
func Validate(value string) error {
	if !utf8.ValidString(value) {
		return ErrInvalidUTF8
	}
	if len(value) > MaxEncodedBytes {
		return ErrTooManyBytes
	}
	codePoints := utf8.RuneCountInString(value)
	if codePoints < MinCodePoints {
		return ErrTooShort
	}
	if codePoints > MaxCodePoints {
		return ErrTooLong
	}
	return nil
}

func IsPolicyError(err error) bool {
	return errors.Is(err, ErrInvalidUTF8) ||
		errors.Is(err, ErrTooShort) ||
		errors.Is(err, ErrTooLong) ||
		errors.Is(err, ErrTooManyBytes)
}
