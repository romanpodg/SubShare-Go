package sources

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

func ValidateURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("source_url is required")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid source_url")
	}
	if err := validateSourceURLShape(parsed); err != nil {
		return "", err
	}
	if err := validateSourceHost(parsed.Hostname()); err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func validateSourceURLShape(parsed *url.URL) error {
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("source_url must use http:// or https://")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return fmt.Errorf("source_url must contain host")
	}
	if parsed.User != nil {
		return fmt.Errorf("source_url must not contain credentials")
	}
	return nil
}

// validateSourceHost rejects hosts that can only ever name this machine or a
// forbidden network without a DNS lookup.
func validateSourceHost(hostname string) error {
	host := strings.ToLower(strings.TrimSuffix(hostname, "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("source_url points to a forbidden host")
	}
	if address, err := netip.ParseAddr(host); err == nil && IsForbiddenIP(address) {
		return fmt.Errorf("source_url points to a forbidden network")
	}
	return nil
}

func IsForbiddenIP(address netip.Addr) bool {
	address = address.Unmap()
	return !address.IsValid() ||
		address.IsUnspecified() ||
		address.IsLoopback() ||
		address.IsPrivate() ||
		address.IsLinkLocalUnicast() ||
		address.IsLinkLocalMulticast() ||
		address.IsMulticast()
}

func ResolveHost(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if host == "" {
		return nil, fmt.Errorf("empty destination host")
	}
	if address, err := netip.ParseAddr(host); err == nil {
		if IsForbiddenIP(address) {
			return nil, fmt.Errorf("destination resolves to a forbidden network")
		}
		return []netip.Addr{address.Unmap()}, nil
	}

	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve destination host: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("destination host has no addresses")
	}
	for _, address := range addresses {
		if IsForbiddenIP(address) {
			return nil, fmt.Errorf("destination resolves to a forbidden network")
		}
	}
	return addresses, nil
}

func NewHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid destination address: %w", err)
			}
			addresses, err := ResolveHost(ctx, host)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, resolved := range addresses {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			return nil, fmt.Errorf("connect to destination: %w", lastErr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			_, err := ValidateURL(req.URL.String())
			return err
		},
	}
}
