package profilestorage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func sampleKeyringJSON(t *testing.T) ([]byte, string, string) {
	t.Helper()
	data, err := GenerateKeyringJSON("key-1", "bik-1")
	if err != nil {
		t.Fatalf("GenerateKeyringJSON failed: %v", err)
	}
	return data, "key-1", "bik-1"
}

func TestSecretProfileURI_Redaction(t *testing.T) {
	raw := "vless://user@example.com:443?security=reality#label"
	sec := NewSecretProfileURI(raw)

	if sec.Reveal() != raw {
		t.Errorf("Reveal mismatch: got %q, want %q", sec.Reveal(), raw)
	}
	if fmt.Sprintf("%s", sec) != "[REDACTED URI]" {
		t.Errorf("%%s formatting exposed secret")
	}
	if fmt.Sprintf("%v", sec) != "[REDACTED URI]" {
		t.Errorf("%%v formatting exposed secret")
	}
	if fmt.Sprintf("%+v", sec) != "[REDACTED URI]" {
		t.Errorf("%%+v formatting exposed secret")
	}
	if fmt.Sprintf("%#v", sec) != "[REDACTED URI]" {
		t.Errorf("%%#v formatting exposed secret")
	}
	if sec.GoString() != "[REDACTED URI]" {
		t.Errorf("GoString exposed secret")
	}

	b, err := json.Marshal(sec)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}
	if string(b) != `"[REDACTED URI]"` {
		t.Errorf("JSON exposed secret: got %s", string(b))
	}
}

func TestKeyring_JSONValidation(t *testing.T) {
	data, activeID, bikID := sampleKeyringJSON(t)
	kr, err := LoadKeyringJSON(data)
	if err != nil {
		t.Fatalf("LoadKeyringJSON failed: %v", err)
	}
	if kr.ActiveKeyID != activeID {
		t.Errorf("ActiveKeyID mismatch: got %q, want %q", kr.ActiveKeyID, activeID)
	}
	if kr.ActiveBlindIndexKeyID != bikID {
		t.Errorf("ActiveBlindIndexKeyID mismatch: got %q, want %q", kr.ActiveBlindIndexKeyID, bikID)
	}

	// Reject duplicate JSON keys
	dupJSON := []byte(`{
		"active_key_id": "key-1",
		"active_key_id": "key-2",
		"keys": {"key-1": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		"active_blind_index_key_id": "bik-1",
		"blind_index_keys": {"bik-1": "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"}
	}`)
	if _, err := LoadKeyringJSON(dupJSON); err == nil {
		t.Errorf("expected error for duplicate JSON keys, got nil")
	}

	// Reject key length != 32 bytes
	shortKeyJSON := []byte(`{
		"active_key_id": "key-1",
		"keys": {"key-1": "c2hvcnRrZXk"},
		"active_blind_index_key_id": "bik-1",
		"blind_index_keys": {"bik-1": "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"}
	}`)
	if _, err := LoadKeyringJSON(shortKeyJSON); err == nil {
		t.Errorf("expected error for short key, got nil")
	}
}

func TestKeyring_DuplicateKeyMaterialRejection(t *testing.T) {
	// Encryption key and blind index key sharing same material
	sameKeyAndBIK := []byte(`{
		"active_key_id": "key-1",
		"keys": {"key-1": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		"active_blind_index_key_id": "bik-1",
		"blind_index_keys": {"bik-1": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}
	}`)
	if _, err := LoadKeyringJSON(sameKeyAndBIK); err == nil {
		t.Errorf("expected error when key and BIK share key material, got nil")
	}

	// Two blind index keys sharing same material
	sameBIKeys := []byte(`{
		"active_key_id": "key-1",
		"keys": {"key-1": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		"active_blind_index_key_id": "bik-1",
		"blind_index_keys": {
			"bik-1": "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
			"bik-2": "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
		}
	}`)
	if _, err := LoadKeyringJSON(sameBIKeys); err == nil {
		t.Errorf("expected error when duplicate BIK material exists, got nil")
	}
}

func TestCrypto_RoundTrip_AAD_Tampering(t *testing.T) {
	data, _, _ := sampleKeyringJSON(t)
	kr, err := LoadKeyringJSON(data)
	if err != nil {
		t.Fatalf("LoadKeyringJSON: %v", err)
	}

	rawURI := "vless://33333333-3333-4333-8333-333333333333@example.com:443?type=tcp"
	rowID := int64(100)

	activeID, activeKey, err := kr.GetActiveEncryptionKey()
	if err != nil {
		t.Fatalf("GetActiveEncryptionKey: %v", err)
	}

	env, err := Encrypt([]byte(rawURI), activeID, activeKey, rowID)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Successful decryption with matching rowID
	dec, err := Decrypt(env, kr, rowID)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if dec.Reveal() != rawURI {
		t.Fatalf("Decrypted URI mismatch: got %q, want %q", dec.Reveal(), rawURI)
	}

	// Cross-row copy attack: attempt decrypting envelope with a different rowID (must fail authentication)
	if _, err := Decrypt(env, kr, int64(101)); err == nil {
		t.Fatalf("expected authentication failure when decrypting with wrong rowID, got nil error")
	}

	// Ciphertext tampering (must fail)
	parts := splitExact(env, '$')
	tamperedCipher := parts[6][:len(parts[6])-4] + "AAAA"
	tamperedEnv := fmt.Sprintf("$%s$%s$%s$%s$%s$%s", parts[1], parts[2], parts[3], parts[4], parts[5], tamperedCipher)
	if _, err := Decrypt(tamperedEnv, kr, rowID); err == nil {
		t.Fatalf("expected authentication failure for tampered ciphertext, got nil error")
	}
}

func TestCrypto_BlindIndex(t *testing.T) {
	bik := []byte("01234567890123456789012345678901")
	uri1 := "vless://user@example.com:443"
	uri2 := "vless://user@example.com:443"
	uri3 := "vless://user2@example.com:443"

	idx1 := ComputeBlindIndex(bik, uri1)
	idx2 := ComputeBlindIndex(bik, uri2)
	idx3 := ComputeBlindIndex(bik, uri3)

	if idx1 != idx2 {
		t.Errorf("Blind index non-deterministic for identical inputs")
	}
	if idx1 == idx3 {
		t.Errorf("Blind index collision for distinct inputs")
	}

	clone1, err := GenerateCloneBlindIndex(bik, uri1)
	if err != nil {
		t.Fatalf("GenerateCloneBlindIndex: %v", err)
	}
	clone2, err := GenerateCloneBlindIndex(bik, uri1)
	if err != nil {
		t.Fatalf("GenerateCloneBlindIndex: %v", err)
	}
	if clone1 == idx1 || clone2 == idx1 || clone1 == clone2 {
		t.Fatal("clone blind indexes must be distinct from deterministic and prior clone indexes")
	}
}

func TestKeyringFile_Permissions(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "keyring.json")
	data, _, _ := sampleKeyringJSON(t)

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	kr, err := LoadKeyringFile(filePath)
	if err != nil {
		t.Fatalf("LoadKeyringFile failed: %v", err)
	}
	if kr == nil {
		t.Fatalf("nil keyring returned")
	}

	if runtime.GOOS != "windows" {
		_ = os.Chmod(filePath, 0666)
		if _, err := LoadKeyringFile(filePath); err == nil {
			t.Errorf("expected error for permissive keyring file mode 0666")
		}
	}
}
