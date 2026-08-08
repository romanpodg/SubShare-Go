package profileconfig

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

// HostResolver resolves host string to allowed IP addresses or returns an error.
type HostResolver func(ctx context.Context, host string) ([]netip.Addr, error)

// CheckConfigurationAvailabilityWithResolver probes TCP reachability or DNS resolution using the provided resolver.
func CheckConfigurationAvailabilityWithResolver(raw string, resolver HostResolver) (string, string, int64) {
	scheme := SupportedConfigScheme(raw)
	if scheme == "hysteria2" || scheme == "hy2" || scheme == "tuic" {
		profile, parseErr := profiles.Parse(raw)
		if parseErr != nil {
			return "down", "invalid_configuration", 0
		}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if resolver != nil {
			if _, resolveErr := resolver(ctx, profile.Server); resolveErr != nil {
				return "down", "destination_not_permitted", 0
			}
		}
		return "unknown", "dns_resolved_udp_quic_probe_unsupported", 0
	}
	if scheme == "ss" {
		profile, parseErr := profiles.Parse(raw)
		if parseErr != nil {
			return "down", "invalid_configuration", 0
		}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		var addresses []netip.Addr
		var resolveErr error
		if resolver != nil {
			addresses, resolveErr = resolver(ctx, profile.Server)
			if resolveErr != nil {
				return "down", "destination_not_permitted", 0
			}
		}
		dialer := &net.Dialer{Timeout: 4 * time.Second}
		start := time.Now()
		var connection net.Conn
		for _, address := range addresses {
			connection, parseErr = dialer.DialContext(ctx, "tcp", net.JoinHostPort(address.String(), profile.Port.Expression))
			if parseErr == nil {
				break
			}
		}
		if parseErr != nil {
			return "down", "tcp_unreachable", 0
		}
		_ = connection.Close()
		return "unknown", "tcp_reachable_authentication_not_performed", time.Since(start).Milliseconds()
	}
	host, port, err := ParseConfigTarget(raw)
	if err != nil {
		return "down", "invalid configuration", 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var addresses []netip.Addr
	if resolver != nil {
		addresses, err = resolver(ctx, host)
		if err != nil {
			return "down", "destination is not a permitted public address", 0
		}
	}

	dialer := &net.Dialer{Timeout: 4 * time.Second}
	start := time.Now()
	var conn net.Conn
	var lastErr error
	for _, address := range addresses {
		conn, lastErr = dialer.DialContext(ctx, "tcp", net.JoinHostPort(address.String(), port))
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return "down", lastErr.Error(), 0
	}
	_ = conn.Close()

	latency := time.Since(start).Milliseconds()
	if latency < 0 {
		latency = 0
	}
	return "up", "", latency
}
