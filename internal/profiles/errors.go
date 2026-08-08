package profiles

import "errors"

type ErrorCode string

const (
	ErrorUnsupportedScheme      ErrorCode = "unsupported_scheme"
	ErrorInvalidAuthority       ErrorCode = "invalid_authority"
	ErrorInvalidPort            ErrorCode = "invalid_port"
	ErrorInvalidEncoding        ErrorCode = "invalid_encoding"
	ErrorMissingCredential      ErrorCode = "missing_credential"
	ErrorUnsupportedGeneration  ErrorCode = "unsupported_protocol_generation"
	ErrorAmbiguousTUICDialect   ErrorCode = "ambiguous_tuic_dialect"
	ErrorCompatibilityOnlyInput ErrorCode = "compatibility_only_input"
	ErrorInvalidProfile         ErrorCode = "invalid_profile"
	ErrorMissingFingerprintKey  ErrorCode = "missing_fingerprint_key"
)

// Error is intentionally value-free: it names only a stable category,
// protocol, and field so credentials and complete URIs cannot leak.
type Error struct {
	Code     ErrorCode
	Protocol Protocol
	Field    string
}

func (profileError *Error) Error() string {
	if profileError == nil {
		return "protocol profile error"
	}
	message := string(profileError.Code)
	if profileError.Protocol != "" {
		message += ": protocol=" + string(profileError.Protocol)
	}
	if profileError.Field != "" {
		message += " field=" + profileError.Field
	}
	return message
}

func (profileError *Error) Is(target error) bool {
	var other *Error
	return errors.As(target, &other) && other.Code == profileError.Code
}

func newError(code ErrorCode, protocol Protocol, field string) error {
	return &Error{Code: code, Protocol: protocol, Field: field}
}

func ErrorCodeOf(err error) ErrorCode {
	var profileError *Error
	if errors.As(err, &profileError) {
		return profileError.Code
	}
	return ""
}
