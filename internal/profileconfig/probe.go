package profileconfig

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

const (
	CheckStatusUp          = "up"
	CheckStatusDown        = "down"
	CheckStatusUnknown     = "unknown"
	CheckStatusUnsupported = "unsupported_check"
)

const configurationProbeTimeout = 4 * time.Second

// HostResolver resolves host string to allowed IP addresses or returns an error.
type HostResolver func(ctx context.Context, host string) ([]netip.Addr, error)

type hysteria2Handshaker func(ctx context.Context, profile *profiles.Profile, addresses []netip.Addr) error

// CheckConfigurationAvailabilityWithResolver performs the strongest bounded
// check currently implemented for the parsed protocol. It retains the legacy
// entry point for callers that do not yet supply a cancellation context.
func CheckConfigurationAvailabilityWithResolver(raw string, resolver HostResolver) (string, string, int64) {
	return CheckConfigurationAvailabilityContext(context.Background(), raw, resolver)
}

// CheckConfigurationAvailabilityContext performs a cancellable health check.
func CheckConfigurationAvailabilityContext(parent context.Context, raw string, resolver HostResolver) (string, string, int64) {
	return checkConfigurationAvailability(parent, raw, resolver, performHysteria2Handshake)
}

func checkConfigurationAvailability(
	parent context.Context,
	raw string,
	resolver HostResolver,
	hy2Handshake hysteria2Handshaker,
) (string, string, int64) {
	scheme := SupportedConfigScheme(raw)
	if scheme == "hysteria" || scheme == "tuic" {
		return CheckStatusUnsupported, "protocol_health_check_unsupported", 0
	}
	if scheme == "hysteria2" || scheme == "hy2" {
		profile, parseErr := profiles.Parse(raw)
		if parseErr != nil {
			return CheckStatusDown, "invalid_configuration", 0
		}
		ctx, cancel := context.WithTimeout(parent, configurationProbeTimeout)
		defer cancel()
		addresses, detail, ok := resolveProbeAddresses(ctx, resolver, profile.Server)
		if !ok {
			return CheckStatusDown, detail, 0
		}
		start := time.Now()
		if err := hy2Handshake(ctx, profile, addresses); err != nil {
			return CheckStatusDown, classifyHysteria2ProbeError(ctx, err), 0
		}
		return CheckStatusUp, "", elapsedMilliseconds(start)
	}
	if scheme == "ss" {
		profile, parseErr := profiles.Parse(raw)
		if parseErr != nil {
			return CheckStatusDown, "invalid_configuration", 0
		}
		ctx, cancel := context.WithTimeout(parent, configurationProbeTimeout)
		defer cancel()
		addresses, detail, ok := resolveProbeAddresses(ctx, resolver, profile.Server)
		if !ok {
			return CheckStatusDown, detail, 0
		}
		if err := probeTCPAddresses(ctx, addresses, profile.Port.Expression); err != nil {
			return CheckStatusDown, "tcp_unreachable", 0
		}
		return CheckStatusUnsupported, "tcp_reachable_authentication_not_performed", 0
	}

	host, port, err := ParseConfigTarget(raw)
	if err != nil {
		return CheckStatusDown, "invalid_configuration", 0
	}
	ctx, cancel := context.WithTimeout(parent, configurationProbeTimeout)
	defer cancel()
	addresses, detail, ok := resolveProbeAddresses(ctx, resolver, host)
	if !ok {
		return CheckStatusDown, detail, 0
	}
	start := time.Now()
	if err := probeTCPAddresses(ctx, addresses, port); err != nil {
		return CheckStatusDown, "tcp_unreachable", 0
	}
	return CheckStatusUp, "", elapsedMilliseconds(start)
}

func resolveProbeAddresses(ctx context.Context, resolver HostResolver, host string) ([]netip.Addr, string, bool) {
	if resolver == nil {
		return nil, "probe_resolver_unavailable", false
	}
	addresses, err := resolver(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, "destination_not_permitted", false
	}
	return addresses, "", true
}

func probeTCPAddresses(ctx context.Context, addresses []netip.Addr, port string) error {
	dialer := &net.Dialer{Timeout: configurationProbeTimeout}
	var lastErr error
	for _, address := range addresses {
		connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(address.String(), port))
		if err == nil {
			_ = connection.Close()
			return nil
		}
		lastErr = err
	}
	return lastErr
}

func elapsedMilliseconds(start time.Time) int64 {
	latency := time.Since(start).Milliseconds()
	if latency < 0 {
		return 0
	}
	return latency
}
