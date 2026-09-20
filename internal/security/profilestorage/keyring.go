package profilestorage

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var validKeyIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

const MaxKeyringFileSize = 64 * 1024 // 64 KB

type Keyring struct {
	ActiveKeyID           string
	Keys                  map[string][]byte
	ActiveBlindIndexKeyID string
	BlindIndexKeys        map[string][]byte
	mu                    sync.RWMutex
}

func (k *Keyring) String() string {
	return "[REDACTED KEYRING]"
}
func (k *Keyring) GoString() string {
	return "[REDACTED KEYRING]"
}
func (k *Keyring) Format(f fmt.State, c rune) {
	_, _ = io.WriteString(f, "[REDACTED KEYRING]")
}

func (k *Keyring) GetEncryptionKey(keyID string) ([]byte, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	key, ok := k.Keys[keyID]
	if !ok {
		return nil, false
	}
	res := make([]byte, len(key))
	copy(res, key)
	return res, true
}

func (k *Keyring) GetActiveEncryptionKey() (string, []byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	key, ok := k.Keys[k.ActiveKeyID]
	if !ok {
		return "", nil, fmt.Errorf("%w: active key %q not found in keyring", ErrUnknownKeyID, k.ActiveKeyID)
	}
	res := make([]byte, len(key))
	copy(res, key)
	return k.ActiveKeyID, res, nil
}

func (k *Keyring) GetBlindIndexKey(bikID string) ([]byte, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	key, ok := k.BlindIndexKeys[bikID]
	if !ok {
		return nil, false
	}
	res := make([]byte, len(key))
	copy(res, key)
	return res, true
}

func (k *Keyring) GetActiveBlindIndexKey() (string, []byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	key, ok := k.BlindIndexKeys[k.ActiveBlindIndexKeyID]
	if !ok {
		return "", nil, fmt.Errorf("%w: active blind index key %q not found in keyring", ErrBlindIndexKeyMismatch, k.ActiveBlindIndexKeyID)
	}
	res := make([]byte, len(key))
	copy(res, key)
	return k.ActiveBlindIndexKeyID, res, nil
}

// LoadKeyringFile reads and parses a keyring file from disk after verifying file permissions.
func LoadKeyringFile(filePath string) (*Keyring, error) {
	cleanPath := filepath.Clean(filePath)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("%w: stat keyring file: %v", ErrMissingKeyring, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%w: keyring path is a directory", ErrMissingKeyring)
	}
	if info.Size() > MaxKeyringFileSize {
		return nil, fmt.Errorf("%w: keyring file exceeds maximum size of 64KB", ErrMissingKeyring)
	}

	if err := checkFilePermissions(cleanPath, info); err != nil {
		return nil, fmt.Errorf("%w: invalid keyring permissions: %v", ErrMissingKeyring, err)
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("%w: read keyring file: %v", ErrMissingKeyring, err)
	}

	return LoadKeyringJSON(data)
}

// LoadKeyringJSON parses strict JSON containing active_key_id, keys, active_blind_index_key_id, blind_index_keys.
// It strictly rejects duplicate keys in JSON objects.
func LoadKeyringJSON(data []byte) (*Keyring, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: keyring JSON is empty", ErrMissingKeyring)
	}

	rawMap, err := parseStrictJSONMap(data)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid keyring JSON: %v", ErrMissingKeyring, err)
	}

	activeKeyID, _ := rawMap["active_key_id"].(string)
	if activeKeyID == "" || !validKeyIDRegex.MatchString(activeKeyID) {
		return nil, fmt.Errorf("%w: invalid or missing active_key_id", ErrMissingKeyring)
	}

	rawKeys, ok := rawMap["keys"].(map[string]any)
	if !ok || len(rawKeys) == 0 {
		return nil, fmt.Errorf("%w: missing or empty keys map", ErrMissingKeyring)
	}

	seenMaterial := make(map[string]string)
	keys, err := decodeEncryptionKeys(rawKeys, seenMaterial)
	if err != nil {
		return nil, err
	}

	if _, exists := keys[activeKeyID]; !exists {
		return nil, fmt.Errorf("%w: active_key_id %q not found in keys map", ErrMissingKeyring, activeKeyID)
	}

	activeBIKID, _ := rawMap["active_blind_index_key_id"].(string)
	if activeBIKID == "" || !validKeyIDRegex.MatchString(activeBIKID) {
		return nil, fmt.Errorf("%w: invalid or missing active_blind_index_key_id", ErrMissingKeyring)
	}

	rawBIKeys, ok := rawMap["blind_index_keys"].(map[string]any)
	if !ok || len(rawBIKeys) == 0 {
		return nil, fmt.Errorf("%w: missing or empty blind_index_keys map", ErrMissingKeyring)
	}

	biKeys, err := decodeBlindIndexKeys(rawBIKeys, seenMaterial)
	if err != nil {
		return nil, err
	}

	if _, exists := biKeys[activeBIKID]; !exists {
		return nil, fmt.Errorf("%w: active_blind_index_key_id %q not found in blind_index_keys map", ErrMissingKeyring, activeBIKID)
	}

	return &Keyring{
		ActiveKeyID:           activeKeyID,
		Keys:                  keys,
		ActiveBlindIndexKeyID: activeBIKID,
		BlindIndexKeys:        biKeys,
	}, nil
}

// decodeEncryptionKeys validates the keys map. seenMaterial is shared with the
// blind index decoder so identical material across the two maps is rejected.
func decodeEncryptionKeys(rawKeys map[string]any, seenMaterial map[string]string) (map[string][]byte, error) {
	keys := make(map[string][]byte, len(rawKeys))
	for id, val := range rawKeys {
		if !validKeyIDRegex.MatchString(id) {
			return nil, fmt.Errorf("%w: invalid key ID syntax %q", ErrMissingKeyring, id)
		}
		strVal, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("%w: key %q value must be string", ErrMissingKeyring, id)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(strVal)
		if err != nil {
			return nil, fmt.Errorf("%w: key %q must be unpadded Base64URL encoded", ErrMissingKeyring, id)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("%w: key %q must decode to exactly 32 bytes (got %d)", ErrMissingKeyring, id, len(decoded))
		}
		mHex := fmt.Sprintf("%x", decoded)
		if existingID, exists := seenMaterial[mHex]; exists {
			return nil, fmt.Errorf("%w: duplicate key material between %q and %q", ErrMissingKeyring, existingID, id)
		}
		seenMaterial[mHex] = id
		keys[id] = decoded
	}
	return keys, nil
}

func decodeBlindIndexKeys(rawBIKeys map[string]any, seenMaterial map[string]string) (map[string][]byte, error) {
	biKeys := make(map[string][]byte, len(rawBIKeys))
	for id, val := range rawBIKeys {
		if !validKeyIDRegex.MatchString(id) {
			return nil, fmt.Errorf("%w: invalid blind index key ID syntax %q", ErrMissingKeyring, id)
		}
		strVal, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("%w: blind index key %q value must be string", ErrMissingKeyring, id)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(strVal)
		if err != nil {
			return nil, fmt.Errorf("%w: blind index key %q must be unpadded Base64URL encoded", ErrMissingKeyring, id)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("%w: blind index key %q must decode to exactly 32 bytes", ErrMissingKeyring, id)
		}
		mHex := fmt.Sprintf("%x", decoded)
		if existingID, exists := seenMaterial[mHex]; exists {
			return nil, fmt.Errorf("%w: duplicate key material between %q and blind index key %q", ErrMissingKeyring, existingID, id)
		}
		seenMaterial[mHex] = id
		biKeys[id] = decoded
	}
	return biKeys, nil
}

func parseStrictJSONMap(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("expected JSON object")
	}

	res, err := parseJSONObject(dec)
	if err != nil {
		return nil, err
	}

	if dec.More() {
		return nil, fmt.Errorf("unexpected trailing JSON data")
	}
	return res, nil
}

func parseJSONObject(dec *json.Decoder) (map[string]any, error) {
	m := make(map[string]any)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string object key")
		}
		if _, duplicate := m[key]; duplicate {
			return nil, fmt.Errorf("duplicate object key %q", key)
		}

		valTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		val, err := parseJSONValue(dec, valTok)
		if err != nil {
			return nil, err
		}
		m[key] = val
	}
	_, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return m, nil
}

func parseJSONValue(dec *json.Decoder, tok json.Token) (any, error) {
	switch v := tok.(type) {
	case json.Delim:
		if v == '{' {
			return parseJSONObject(dec)
		} else if v == '[' {
			var list []any
			for dec.More() {
				elemTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				elem, err := parseJSONValue(dec, elemTok)
				if err != nil {
					return nil, err
				}
				list = append(list, elem)
			}
			_, err := dec.Token()
			if err != nil {
				return nil, err
			}
			return list, nil
		}
		return nil, fmt.Errorf("unexpected delimiter %v", v)
	default:
		return v, nil
	}
}

func GenerateKeyringJSON(keyID, bikID string) ([]byte, error) {
	if !validKeyIDRegex.MatchString(keyID) || !validKeyIDRegex.MatchString(bikID) {
		return nil, fmt.Errorf("invalid key ID syntax")
	}

	encKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, encKey); err != nil {
		return nil, err
	}
	bikKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, bikKey); err != nil {
		return nil, err
	}

	m := map[string]any{
		"active_key_id": keyID,
		"keys": map[string]string{
			keyID: base64.RawURLEncoding.EncodeToString(encKey),
		},
		"active_blind_index_key_id": bikID,
		"blind_index_keys": map[string]string{
			bikID: base64.RawURLEncoding.EncodeToString(bikKey),
		},
	}
	return json.MarshalIndent(m, "", "  ")
}
