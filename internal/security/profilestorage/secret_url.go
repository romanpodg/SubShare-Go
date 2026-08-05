package profilestorage

import (
	"fmt"
	"io"
)

// SecretProfileURI holds sensitive profile URI material in unexported bytes
// and explicitly prevents accidental leakage via formatting, JSON, or logging.
type SecretProfileURI struct {
	data []byte
}

func NewSecretProfileURI(raw string) SecretProfileURI {
	return SecretProfileURI{data: []byte(raw)}
}

func NewSecretProfileURIFromBytes(raw []byte) SecretProfileURI {
	if raw == nil {
		return SecretProfileURI{}
	}
	b := make([]byte, len(raw))
	copy(b, raw)
	return SecretProfileURI{data: b}
}

func (s SecretProfileURI) Format(f fmt.State, c rune) {
	_, _ = io.WriteString(f, "[REDACTED URI]")
}

func (s SecretProfileURI) String() string {
	return "[REDACTED URI]"
}

func (s SecretProfileURI) GoString() string {
	return "[REDACTED URI]"
}

func (s SecretProfileURI) MarshalJSON() ([]byte, error) {
	return []byte(`"[REDACTED URI]"`), nil
}

func (s SecretProfileURI) MarshalText() ([]byte, error) {
	return []byte("[REDACTED URI]"), nil
}

// Reveal returns the raw plaintext profile URI string.
// This method is narrowly scoped and must only be used at immediate delivery/parser boundaries.
func (s SecretProfileURI) Reveal() string {
	return string(s.data)
}

func (s SecretProfileURI) IsZero() bool {
	return len(s.data) == 0
}
