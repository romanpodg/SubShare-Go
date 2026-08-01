package password

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const syntheticPassword = "synthetic passphrase 2026"

func TestArgon2idHashVerifyAndRehashDecision(t *testing.T) {
	hasher := NewDefault()
	encoded, err := hasher.Hash(syntheticPassword)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if Identify(encoded) != AlgorithmArgon2id || !strings.HasPrefix(encoded, "$argon2id$v=19$m=32768,t=3,p=2$") {
		t.Fatalf("unexpected hash format")
	}
	secondEncoded, err := hasher.Hash(syntheticPassword)
	if err != nil {
		t.Fatalf("second Hash: %v", err)
	}
	if encoded == secondEncoded {
		t.Fatalf("independent hashes unexpectedly reused a salt")
	}
	verified, err := hasher.Verify(syntheticPassword, encoded)
	if err != nil || !verified.Valid || verified.NeedsRehash {
		t.Fatalf("Verify = %#v, %v", verified, err)
	}
	needsRehash, err := hasher.NeedsRehash(encoded)
	if err != nil || needsRehash {
		t.Fatalf("NeedsRehash(current) = %v, %v", needsRehash, err)
	}
	wrong, err := hasher.Verify("wrong synthetic password", encoded)
	if err != nil || wrong.Valid {
		t.Fatalf("wrong password = %#v, %v", wrong, err)
	}

	olderParameters := DefaultParameters
	olderParameters.Iterations = 2
	older, err := New(olderParameters, nil)
	if err != nil {
		t.Fatalf("New older hasher: %v", err)
	}
	olderHash, err := older.Hash(syntheticPassword)
	if err != nil {
		t.Fatalf("older Hash: %v", err)
	}
	verified, err = hasher.Verify(syntheticPassword, olderHash)
	if err != nil || !verified.Valid || !verified.NeedsRehash {
		t.Fatalf("parameter upgrade decision = %#v, %v", verified, err)
	}
	needsRehash, err = hasher.NeedsRehash(olderHash)
	if err != nil || !needsRehash {
		t.Fatalf("NeedsRehash(older) = %v, %v", needsRehash, err)
	}
}

func TestMalformedEncodedHashIsRejectedWithoutEcho(t *testing.T) {
	malformed := "$argon2id$v=19$m=999999999,t=3,p=2$c2FsdA$aGFzaA"
	_, err := NewDefault().Verify(syntheticPassword, malformed)
	if !errors.Is(err, ErrInvalidEncodedHash) || strings.Contains(err.Error(), malformed) || strings.Contains(err.Error(), syntheticPassword) {
		t.Fatalf("unsafe malformed hash error: %v", err)
	}
}

func TestBcryptVerificationRequestsMigration(t *testing.T) {
	encoded, err := bcrypt.GenerateFromPassword([]byte("legacy-short"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	verified, err := NewDefault().Verify("legacy-short", string(encoded))
	if err != nil || !verified.Valid || !verified.NeedsRehash || verified.Algorithm != AlgorithmBcrypt {
		t.Fatalf("bcrypt Verify = %#v, %v", verified, err)
	}
	replacement, err := NewDefault().RehashVerified("legacy-short")
	if err != nil {
		t.Fatalf("RehashVerified: %v", err)
	}
	verified, err = NewDefault().Verify("legacy-short", replacement)
	if err != nil || !verified.Valid || verified.NeedsRehash || verified.Algorithm != AlgorithmArgon2id {
		t.Fatalf("migrated Verify = %#v, %v", verified, err)
	}
}

func TestPasswordPolicyCodePointsBytesAndExactValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  error
	}{
		{"14 code points", strings.Repeat("я", 14), ErrTooShort},
		{"15 code points", strings.Repeat("я", 15), nil},
		{"256 code points", strings.Repeat("😀", 256), nil},
		{"above code point limit", strings.Repeat("a", 257), ErrTooLong},
		{"Unicode and spaces", " пароль с пробелами ", nil},
		{"no silent trimming", "             a ", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Validate(test.value)
			if !errors.Is(err, test.want) {
				t.Fatalf("Validate(%d code points, %d bytes) = %v, want %v", utf8.RuneCountInString(test.value), len(test.value), err, test.want)
			}
		})
	}

	maxMultibyte := strings.Repeat("😀", 256)
	if len(maxMultibyte) != MaxEncodedBytes || Validate(maxMultibyte) != nil {
		t.Fatalf("256 supplementary code points should fit the encoded-byte limit")
	}
	overBytes := maxMultibyte + "😀"
	if err := Validate(overBytes); !errors.Is(err, ErrTooManyBytes) {
		t.Fatalf("multibyte limit error = %v", err)
	}
}

func TestHashPreservesSpaces(t *testing.T) {
	hasher := NewDefault()
	value := "  exact passphrase  "
	encoded, err := hasher.Hash(value)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	verified, err := hasher.Verify(value, encoded)
	if err != nil || !verified.Valid {
		t.Fatalf("exact value did not verify: %#v %v", verified, err)
	}
	trimmed, err := hasher.Verify(strings.TrimSpace(value), encoded)
	if err != nil || trimmed.Valid {
		t.Fatalf("trimmed value unexpectedly verified: %#v %v", trimmed, err)
	}
}

func BenchmarkArgon2idHash(b *testing.B) {
	hasher := NewDefault()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := hasher.Hash(syntheticPassword); err != nil {
			b.Fatal(err)
		}
	}
}
