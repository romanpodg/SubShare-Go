package main

import (
	"context"
	"github.com/romanpodg/SubShare-Go/internal/profileconfig"
	"github.com/romanpodg/SubShare-Go/internal/sources"
)

func checkConfigurationAvailability(raw string) (string, string, int64) {
	return profileconfig.CheckConfigurationAvailabilityWithResolver(raw, sources.ResolveHost)
}

func checkConfigurationAvailabilityContext(ctx context.Context, raw string) (string, string, int64) {
	return profileconfig.CheckConfigurationAvailabilityContext(ctx, raw, sources.ResolveHost)
}
