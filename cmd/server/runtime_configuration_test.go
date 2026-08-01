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
