package configuration

import (
	"strings"
	"testing"
	"time"
)

func testEnvironment(values map[string]string) map[string]string {
	result := map[string]string{"ADMIN_PASSWORD": "test-password"}
	for key, value := range values {
		result[key] = value
	}
	return result
}

func TestLoadDefaultsAndNormalizedEnums(t *testing.T) {
	config, err := LoadFromMap(testEnvironment(map[string]string{
		"SUBSCRIPTION_BODY_ENCODING": " Plain ",
		"APP_ENV":                    " PRODUCTION ",
		"BASE_URL":                   "https://vpn.example.com/",
		"DB_JOURNAL_MODE":            " delete ",
		"PORT":                       "8443",
		"BACKUP_INTERVAL":            "30m",
		"TRUSTED_PROXIES":            "127.0.0.1, 10.0.0.0/8",
		"CORS_ORIGINS":               " https://admin.example.com, http://localhost:3000 ",
	}))
	if err != nil {
		t.Fatalf("LoadFromMap: %v", err)
	}
	if config.SubscriptionBodyEncoding != "plain" || config.Environment != "production" || config.SQLiteJournalMode != "DELETE" {
		t.Fatalf("unexpected normalized enums: %#v", config)
	}
	if config.BaseURL != "https://vpn.example.com" || config.ListenAddress != ":8443" || config.BackupInterval != 30*time.Minute {
		t.Fatalf("unexpected normalized configuration: %#v", config)
	}
	if len(config.TrustedProxyNetworks) != 2 || len(config.CORSOrigins) != 2 {
		t.Fatalf("list configuration was not parsed: %#v", config)
	}

	defaults, err := LoadFromMap(testEnvironment(nil))
	if err != nil {
		t.Fatalf("Load defaults: %v", err)
	}
	if defaults.DBPath != DefaultDBPath || defaults.SubscriptionBodyEncoding != "base64" || defaults.ListenAddress != ":8080" || defaults.BackupInterval != time.Hour {
		t.Fatalf("unexpected defaults: %#v", defaults)
	}
}

func TestLoadRejectsUnsupportedEnumsAndInvalidValues(t *testing.T) {
	tests := []struct {
		name, key, value, want string
	}{
		{"encoding", "SUBSCRIPTION_BODY_ENCODING", "base-64", "SUBSCRIPTION_BODY_ENCODING"},
		{"environment", "APP_ENV", "staging", "APP_ENV"},
		{"journal mode", "DB_JOURNAL_MODE", "wal2", "DB_JOURNAL_MODE"},
		{"malformed port", "PORT", "eight", "PORT"},
		{"port range", "PORT", "70000", "1 to 65535"},
		{"malformed duration", "BACKUP_INTERVAL", "tomorrow", "BACKUP_INTERVAL"},
		{"negative duration", "BACKUP_INTERVAL", "-1h", "BACKUP_INTERVAL"},
		{"invalid proxy", "TRUSTED_PROXIES", "proxy.local", "TRUSTED_PROXIES"},
		{"invalid cors URL", "CORS_ORIGINS", "https://admin.example.com/path", "CORS_ORIGINS"},
		{"invalid base URL", "BASE_URL", "ftp://example.com", "BASE_URL"},
		{"invalid happ URL", "HAPP_CRYPTO_API_URL", "not-a-url", "HAPP_CRYPTO_API_URL"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadFromMap(testEnvironment(map[string]string{test.key: test.value}))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLoadRequiresProductionBaseURLAndNeverLeaksSecrets(t *testing.T) {
	secret := "not-for-logs"
	_, err := LoadFromMap(testEnvironment(map[string]string{
		"ADMIN_PASSWORD": secret,
		"APP_ENV":        "production",
	}))
	if err == nil || !strings.Contains(err.Error(), "BASE_URL") || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe production error: %v", err)
	}

	_, err = LoadFromMap(map[string]string{"ADMIN_PASSWORD": ""})
	if err == nil || strings.Contains(err.Error(), "ADMIN_PASSWORD=") {
		t.Fatalf("unsafe required-secret error: %v", err)
	}
}

func TestParseBooleanRejectsMalformedValues(t *testing.T) {
	for _, raw := range []string{"true", " FALSE ", "TrUe"} {
		if _, err := ParseBoolean("EXAMPLE_BOOLEAN", raw); err != nil {
			t.Fatalf("ParseBoolean(%q): %v", raw, err)
		}
	}
	if _, err := ParseBoolean("EXAMPLE_BOOLEAN", "yes"); err == nil || !strings.Contains(err.Error(), "EXAMPLE_BOOLEAN") {
		t.Fatalf("malformed boolean error = %v", err)
	}
}

func TestRedactURLRemovesCredentialsAndSensitiveParts(t *testing.T) {
	redacted := RedactURL("https://user:password@example.com/path?token=secret#fragment")
	if redacted != "https://example.com/…" || strings.Contains(redacted, "password") || strings.Contains(redacted, "secret") {
		t.Fatalf("unsafe redaction: %q", redacted)
	}
}
