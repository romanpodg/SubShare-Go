// Package configuration loads and validates the process environment before
// runtime resources such as the database, HTTP listener, or jobs are started.
package configuration

import (
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
)

const (
	DefaultDBPath                   = "data/app.db"
	DefaultAdminUser                = "admin"
	DefaultEnvironment              = "development"
	DefaultSubscriptionBodyEncoding = "base64"
	DefaultSQLiteJournalMode        = "WAL"
	DefaultBackupInterval           = time.Hour
	DefaultListenAddress            = ":8080"
)

// Config contains validated runtime configuration. Secret values are retained
// only for the startup code that needs them and are never included in errors.
type Config struct {
	DBPath                    string
	AdminUser                 string
	AdminPassword             string
	DeviceLimitMessage        string
	BaseURL                   string
	Environment               string
	HappCryptoAPIURL          string
	SubscriptionBodyEncoding  string
	CORSOrigins               []string
	TrustedProxyNetworks      []netip.Prefix
	ListenAddress             string
	SQLiteJournalMode         string
	BackupPath                string
	BackupInterval            time.Duration
	ProfileFingerprintKey     []byte
	ProfileFingerprintOldKeys [][]byte
	ProfileKeyring            *profilestorage.Keyring
}

// Format prevents configuration secrets from being emitted by fmt/log calls.
func (config Config) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "configuration{environment="+config.Environment+", secrets=[redacted]}")
}

func (config Config) String() string {
	return "configuration{environment=" + config.Environment + ", secrets=[redacted]}"
}
func (config Config) GoString() string { return config.String() }

// Load parses os.Environ()-style entries. It is separated from os.Getenv so
// callers can validate before runtime initialization and tests are hermetic.
func Load(environ []string) (Config, error) {
	values := make(map[string]string, len(environ))
	for _, entry := range environ {
		name, value, found := strings.Cut(entry, "=")
		if found {
			values[name] = value
		}
	}
	return LoadFromMap(values)
}

// LoadFromMap is useful for tests and embedded deployments that supply their
// own environment source.
func LoadFromMap(values map[string]string) (Config, error) {
	config := Config{
		DBPath:                   DefaultDBPath,
		AdminUser:                DefaultAdminUser,
		Environment:              DefaultEnvironment,
		SubscriptionBodyEncoding: DefaultSubscriptionBodyEncoding,
		SQLiteJournalMode:        DefaultSQLiteJournalMode,
		BackupInterval:           DefaultBackupInterval,
		ListenAddress:            DefaultListenAddress,
	}

	if value := trimmed(values, "DB_PATH"); value != "" {
		if strings.ContainsRune(value, 0) || filepath.Clean(value) == "." {
			return Config{}, fmt.Errorf("DB_PATH must be a non-empty database file path")
		}
		config.DBPath = value
	}
	if value := trimmed(values, "ADMIN_USER"); value != "" {
		config.AdminUser = value
	}
	// Passwords are exact-value secrets: never trim or otherwise transform them.
	// Bootstrap decides whether this optional value is required after inspecting
	// the administrator table.
	config.AdminPassword = values["ADMIN_PASSWORD"]
	config.DeviceLimitMessage = trimmed(values, "DEVICE_LIMIT_MESSAGE")

	var err error
	if config.Environment, err = parseEnum("APP_ENV", trimmed(values, "APP_ENV"), DefaultEnvironment, "development", "production"); err != nil {
		return Config{}, err
	}
	if config.SubscriptionBodyEncoding, err = parseEnum("SUBSCRIPTION_BODY_ENCODING", trimmed(values, "SUBSCRIPTION_BODY_ENCODING"), DefaultSubscriptionBodyEncoding, "base64", "plain"); err != nil {
		return Config{}, err
	}
	if config.SQLiteJournalMode, err = parseEnum("DB_JOURNAL_MODE", trimmed(values, "DB_JOURNAL_MODE"), DefaultSQLiteJournalMode, "wal", "delete", "truncate", "persist", "memory", "off"); err != nil {
		return Config{}, err
	}
	config.SQLiteJournalMode = strings.ToUpper(config.SQLiteJournalMode)

	if value := trimmed(values, "BASE_URL"); value != "" {
		if config.BaseURL, err = parseHTTPURL("BASE_URL", value, true); err != nil {
			return Config{}, err
		}
	} else if config.Environment == "production" {
		return Config{}, fmt.Errorf("BASE_URL is required when APP_ENV=production")
	}
	if value := trimmed(values, "HAPP_CRYPTO_API_URL"); value != "" {
		if config.HappCryptoAPIURL, err = parseHTTPURL("HAPP_CRYPTO_API_URL", value, false); err != nil {
			return Config{}, err
		}
	}
	if config.CORSOrigins, err = parseCORSOrigins(trimmed(values, "CORS_ORIGINS")); err != nil {
		return Config{}, err
	}
	if config.TrustedProxyNetworks, err = parseTrustedProxyNetworks(trimmed(values, "TRUSTED_PROXIES")); err != nil {
		return Config{}, err
	}
	if config.ListenAddress, err = parseListenAddress(trimmed(values, "PORT")); err != nil {
		return Config{}, err
	}
	if config.BackupPath = trimmed(values, "BACKUP_PATH"); strings.ContainsRune(config.BackupPath, 0) {
		return Config{}, fmt.Errorf("BACKUP_PATH must be a valid file path")
	}
	if config.BackupInterval, err = parsePositiveDuration("BACKUP_INTERVAL", trimmed(values, "BACKUP_INTERVAL"), DefaultBackupInterval); err != nil {
		return Config{}, err
	}
	if err = loadProfileSecrets(values, &config); err != nil {
		return Config{}, err
	}

	return config, nil
}

// loadProfileSecrets fills the fingerprint keys and encryption keyring.
func loadProfileSecrets(values map[string]string, config *Config) error {
	var err error
	if config.ProfileFingerprintKey, err = parseFingerprintKey("PROFILE_FINGERPRINT_KEY", trimmed(values, "PROFILE_FINGERPRINT_KEY")); err != nil {
		return err
	}
	previousFingerprintKeys := values["PROFILE_FINGERPRINT_PREVIOUS_KEYS"]
	if previousFingerprintKeys != "" && strings.TrimSpace(previousFingerprintKeys) == "" {
		return fmt.Errorf("PROFILE_FINGERPRINT_PREVIOUS_KEYS must not contain whitespace-only entries")
	}
	if config.ProfileFingerprintOldKeys, err = parseFingerprintOldKeys(strings.TrimSpace(previousFingerprintKeys), config.ProfileFingerprintKey); err != nil {
		return err
	}
	config.ProfileKeyring, err = loadProfileKeyring(values)
	return err
}

func loadProfileKeyring(values map[string]string) (*profilestorage.Keyring, error) {
	keyringFile := trimmed(values, "PROFILE_ENCRYPTION_KEYRING_FILE")
	keyringJSON := values["PROFILE_ENCRYPTION_KEYRING_JSON"]

	if keyringFile != "" && keyringJSON != "" {
		return nil, fmt.Errorf("PROFILE_ENCRYPTION_KEYRING_FILE and PROFILE_ENCRYPTION_KEYRING_JSON are mutually exclusive")
	}
	if keyringFile != "" {
		kr, err := profilestorage.LoadKeyringFile(keyringFile)
		if err != nil {
			return nil, fmt.Errorf("load keyring file: %w", err)
		}
		return kr, nil
	}
	if keyringJSON != "" {
		kr, err := profilestorage.LoadKeyringJSON([]byte(keyringJSON))
		if err != nil {
			return nil, fmt.Errorf("load keyring JSON: %w", err)
		}
		return kr, nil
	}
	return nil, nil
}

func parseFingerprintKey(name, raw string) ([]byte, error) {
	if raw == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) < 32 {
		return nil, fmt.Errorf("%s must be hexadecimal encoding of at least 32 random bytes", name)
	}
	return decoded, nil
}

func parseFingerprintOldKeys(raw string, current []byte) ([][]byte, error) {
	if raw == "" {
		return nil, nil
	}
	seen := map[string]struct{}{hex.EncodeToString(current): {}}
	result := make([][]byte, 0)
	for _, item := range strings.Split(raw, ",") {
		key, err := parseFingerprintKey("PROFILE_FINGERPRINT_PREVIOUS_KEYS", strings.TrimSpace(item))
		if err != nil {
			return nil, err
		}
		encoded := hex.EncodeToString(key)
		if _, exists := seen[encoded]; exists {
			return nil, fmt.Errorf("PROFILE_FINGERPRINT_PREVIOUS_KEYS must contain distinct keys")
		}
		seen[encoded] = struct{}{}
		result = append(result, key)
	}
	return result, nil
}

// ParseBoolean accepts only true or false, ignoring case and surrounding
// whitespace. Configuration currently has no boolean variables, but this
// shared parser prevents permissive behavior when one is introduced.
func ParseBoolean(name, raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", name)
	}
}

// RedactURL removes paths, query strings, credentials, and fragments before a
// URL is included in diagnostic output.
func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "[REDACTED]"
	}
	return parsed.Scheme + "://" + parsed.Host + "/…"
}

func trimmed(values map[string]string, name string) string {
	return strings.TrimSpace(values[name])
}

func parseEnum(name, raw, defaultValue string, allowed ...string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return defaultValue, nil
	}
	for _, candidate := range allowed {
		if value == candidate {
			return value, nil
		}
	}
	return "", fmt.Errorf("%s must be one of: %s", name, strings.Join(allowed, ", "))
}

func parseHTTPURL(name, raw string, requireOriginOnly bool) (string, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("%s must be an absolute http(s) URL", name)
	}
	if requireOriginOnly && (parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "") {
		return "", fmt.Errorf("%s must contain only scheme and host", name)
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func parseCORSOrigins(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	origins := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		origin, err := parseHTTPURL("CORS_ORIGINS", item, true)
		if err != nil {
			return nil, err
		}
		origins = append(origins, origin)
	}
	return origins, nil
}

func parseTrustedProxyNetworks(raw string) ([]netip.Prefix, error) {
	if raw == "" {
		return nil, nil
	}
	networks := make([]netip.Prefix, 0)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if address, err := netip.ParseAddr(item); err == nil {
			bits := 128
			if address.Is4() {
				bits = 32
			}
			networks = append(networks, netip.PrefixFrom(address, bits))
			continue
		}
		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES contains an invalid IP address or CIDR")
		}
		networks = append(networks, prefix.Masked())
	}
	return networks, nil
}

func parseListenAddress(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return DefaultListenAddress, nil
	}
	if !strings.Contains(value, ":") {
		return listenAddressForPort(value)
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", fmt.Errorf("PORT must be a port number or host:port with a port from 1 to 65535")
	}
	validated, err := listenAddressForPort(port)
	if err != nil {
		return "", err
	}
	_, port, _ = net.SplitHostPort(validated)
	return net.JoinHostPort(host, port), nil
}

func listenAddressForPort(raw string) (string, error) {
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("PORT must be a port number from 1 to 65535")
	}
	return ":" + strconv.Itoa(port), nil
}

func parsePositiveDuration(name, raw string, defaultValue time.Duration) (time.Duration, error) {
	if raw == "" {
		return defaultValue, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", name)
	}
	return value, nil
}
