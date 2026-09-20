package main

import (
	"context"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
)

func checkConfigurationAvailability(raw string) (string, string, int64) {
	return profileconfig.CheckConfigurationAvailabilityWithResolver(raw, resolveExternalHost)
}

func checkConfigurationAvailabilityContext(ctx context.Context, raw string) (string, string, int64) {
	return profileconfig.CheckConfigurationAvailabilityContext(ctx, raw, resolveExternalHost)
}
