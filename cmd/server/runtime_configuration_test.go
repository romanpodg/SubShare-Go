package main

import (
	"strings"
	"testing"

	"github.com/romanpodg/SubShare-Go/internal/platform/configuration"
)

func TestBootstrapRuntimeDoesNotStartAfterConfigurationFailure(t *testing.T) {
	started := false
	err := bootstrapRuntime(
		func() (configuration.Config, error) {
			return configuration.LoadFromMap(map[string]string{
				"ADMIN_PASSWORD":             "test-password",
				"PROFILE_FINGERPRINT_KEY":    strings.Repeat("ab", 32),
				"SUBSCRIPTION_BODY_ENCODING": "plaintext",
			})
		},
		func(configuration.Config) error {
			started = true
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "SUBSCRIPTION_BODY_ENCODING") {
		t.Fatalf("unexpected configuration error: %v", err)
	}
	if started {
		t.Fatal("runtime startup proceeded after configuration validation failed")
	}
}

func TestBootstrapRuntimeRequiresFingerprintKeyBeforeDatabaseStartup(t *testing.T) {
	for name, key := range map[string]string{"missing": "", "whitespace": "   "} {
		t.Run(name, func(t *testing.T) {
			started := false
			err := bootstrapRuntime(
				func() (configuration.Config, error) {
					values := map[string]string{"ADMIN_PASSWORD": "test-password"}
					if name != "missing" {
						values["PROFILE_FINGERPRINT_KEY"] = key
					}
					return configuration.LoadFromMap(values)
				},
				func(configuration.Config) error {
					started = true
					return nil
				},
			)
			if err == nil || !strings.Contains(err.Error(), "PROFILE_FINGERPRINT_KEY") {
				t.Fatalf("unsafe startup error: %v", err)
			}
			if started {
				t.Fatal("database/runtime startup proceeded without the fingerprint key")
			}
		})
	}
}
