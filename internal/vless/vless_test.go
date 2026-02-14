package vless

import (
	"net"
	"strings"
	"testing"
)

func TestParseVLESSParts(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantUUID     string
		wantHost     string
		wantPort     string
		wantQuery    string
		wantFragment string
		wantErr      bool
	}{
		{
			name:         "valid full VLESS URL",
			input:        "vless://abcd-1234@example.com:443?type=tcp&security=tls#MyServer",
			wantUUID:     "abcd-1234",
			wantHost:     "example.com",
			wantPort:     "443",
			wantQuery:    "type=tcp&security=tls",
			wantFragment: "MyServer",
			wantErr:      false,
		},
		{
			name:         "missing port defaults to 443",
			input:        "vless://uuid-here@example.com?type=tcp#name",
			wantUUID:     "uuid-here",
			wantHost:     "example.com",
			wantPort:     "443",
			wantQuery:    "type=tcp",
			wantFragment: "name",
			wantErr:      false,
		},
		{
			name:    "non-vless scheme returns error",
			input:   "https://example.com",
			wantErr: true,
		},
		{
			name:    "empty string returns error",
			input:   "",
			wantErr: true,
		},
		{
			name:         "URL without user info has empty uuid",
			input:        "vless://example.com:8443?flow=xtls#tag",
			wantUUID:     "",
			wantHost:     "example.com",
			wantPort:     "8443",
			wantQuery:    "flow=xtls",
			wantFragment: "tag",
			wantErr:      false,
		},
		{
			name:         "port 8080",
			input:        "vless://abc@host.net:8080#frag",
			wantUUID:     "abc",
			wantHost:     "host.net",
			wantPort:     "8080",
			wantQuery:    "",
			wantFragment: "frag",
			wantErr:      false,
		},
		{
			name:         "whitespace around URL is trimmed",
			input:        "  vless://id@server.com:443?q=1#f  ",
			wantUUID:     "id",
			wantHost:     "server.com",
			wantPort:     "443",
			wantQuery:    "q=1",
			wantFragment: "f",
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid, host, port, query, fragment, err := ParseVLESSParts(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseVLESSParts(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVLESSParts(%q) unexpected error: %v", tt.input, err)
			}
			if uuid != tt.wantUUID {
				t.Errorf("uuid = %q, want %q", uuid, tt.wantUUID)
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("port = %q, want %q", port, tt.wantPort)
			}
			if query != tt.wantQuery {
				t.Errorf("query = %q, want %q", query, tt.wantQuery)
			}
			if fragment != tt.wantFragment {
				t.Errorf("fragment = %q, want %q", fragment, tt.wantFragment)
			}
		})
	}
}

func TestBuildVLESSURL(t *testing.T) {
	tests := []struct {
		name     string
		uuid     string
		host     string
		port     string
		query    string
		fragment string
		wantErr  bool
		contains []string // substrings that must appear in the result
	}{
		{
			name:     "valid full URL",
			uuid:     "abcd-1234",
			host:     "example.com",
			port:     "443",
			query:    "type=tcp",
			fragment: "MyServer",
			wantErr:  false,
			contains: []string{"vless://", "abcd-1234", "example.com", "443", "type=tcp", "MyServer"},
		},
		{
			name:    "empty UUID returns error",
			uuid:    "",
			host:    "example.com",
			port:    "443",
			wantErr: true,
		},
		{
			name:    "empty host returns error",
			uuid:    "abcd",
			host:    "",
			port:    "443",
			wantErr: true,
		},
		{
			name:     "empty port defaults to 443",
			uuid:     "uuid-1",
			host:     "server.com",
			port:     "",
			query:    "",
			fragment: "",
			wantErr:  false,
			contains: []string{"vless://", "uuid-1", "server.com", "443"},
		},
		{
			name:     "with query and fragment",
			uuid:     "test-uuid",
			host:     "host.io",
			port:     "8443",
			query:    "security=tls&flow=xtls",
			fragment: "label",
			wantErr:  false,
			contains: []string{"vless://", "test-uuid", "host.io", "8443", "security=tls", "label"},
		},
		{
			name:     "whitespace trimmed from inputs",
			uuid:     "  my-uuid  ",
			host:     "  host.com  ",
			port:     "  443  ",
			query:    "  q=1  ",
			fragment: "  tag  ",
			wantErr:  false,
			contains: []string{"my-uuid", "host.com", "443"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildVLESSURL(tt.uuid, tt.host, tt.port, tt.query, tt.fragment)
			if tt.wantErr {
				if err == nil {
					t.Errorf("BuildVLESSURL() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildVLESSURL() unexpected error: %v", err)
			}
			for _, sub := range tt.contains {
				if !strings.Contains(got, sub) {
					t.Errorf("BuildVLESSURL() = %q, expected it to contain %q", got, sub)
				}
			}
		})
	}
}

func TestBuildAndParse_Roundtrip(t *testing.T) {
	// Build a URL then parse it to verify roundtrip consistency.
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	host := "example.com"
	port := "443"
	query := "type=tcp&security=tls"
	fragment := "Server1"

	built, err := BuildVLESSURL(uuid, host, port, query, fragment)
	if err != nil {
		t.Fatalf("BuildVLESSURL() error: %v", err)
	}

	pUUID, pHost, pPort, pQuery, pFragment, err := ParseVLESSParts(built)
	if err != nil {
		t.Fatalf("ParseVLESSParts() error: %v", err)
	}
	if pUUID != uuid {
		t.Errorf("roundtrip uuid = %q, want %q", pUUID, uuid)
	}
	if pHost != host {
		t.Errorf("roundtrip host = %q, want %q", pHost, host)
	}
	if pPort != port {
		t.Errorf("roundtrip port = %q, want %q", pPort, port)
	}
	if pQuery != query {
		t.Errorf("roundtrip query = %q, want %q", pQuery, query)
	}
	if pFragment != fragment {
		t.Errorf("roundtrip fragment = %q, want %q", pFragment, fragment)
	}
}

func TestTruncateMiddle(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		maxLen int
		want   string
	}{
		{
			name:   "short string under limit is unchanged",
			value:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "string exactly at limit is unchanged",
			value:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "long string truncated with ellipsis",
			value:  "abcdefghij",
			maxLen: 7,
			// head = (7-1)/2 = 3, tail = 7-3-1 = 3
			// "abc" + ellipsis + "hij"
			want: "abc\u2026hij",
		},
		{
			name:   "maxLen 0 returns string unchanged",
			value:  "test",
			maxLen: 0,
			want:   "test",
		},
		{
			name:   "maxLen 3 returns first 3 chars",
			value:  "abcdefgh",
			maxLen: 3,
			want:   "abc",
		},
		{
			name:   "maxLen 1 returns first char",
			value:  "abcdef",
			maxLen: 1,
			want:   "a",
		},
		{
			name:   "empty string returns empty",
			value:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "negative maxLen returns unchanged",
			value:  "hello",
			maxLen: -5,
			want:   "hello",
		},
		{
			name:   "maxLen 4 truncation",
			value:  "abcdefghij",
			maxLen: 4,
			// head = (4-1)/2 = 1, tail = 4-1-1 = 2
			// "a" + ellipsis + "ij"
			want: "a\u2026ij",
		},
		{
			name:   "whitespace is trimmed before truncation",
			value:  "  hello  ",
			maxLen: 10,
			want:   "hello",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateMiddle(tt.value, tt.maxLen)
			if got != tt.want {
				t.Errorf("TruncateMiddle(%q, %d) = %q, want %q", tt.value, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"10.0.0.1 is private", "10.0.0.1", true},
		{"10.255.255.255 is private", "10.255.255.255", true},
		{"172.16.0.1 is private", "172.16.0.1", true},
		{"172.31.255.255 is private", "172.31.255.255", true},
		{"192.168.1.1 is private", "192.168.1.1", true},
		{"192.168.0.0 is private", "192.168.0.0", true},
		{"127.0.0.1 is loopback", "127.0.0.1", true},
		{"127.0.0.2 is loopback", "127.0.0.2", true},
		{"169.254.1.1 is link-local", "169.254.1.1", true},
		{"8.8.8.8 is public", "8.8.8.8", false},
		{"1.1.1.1 is public", "1.1.1.1", false},
		{"203.0.113.1 is public", "203.0.113.1", false},
		{"172.32.0.1 is public (outside 172.16/12)", "172.32.0.1", false},
		{"::1 IPv6 loopback is private", "::1", true},
		{"fc00::1 IPv6 unique local is private", "fc00::1", true},
		{"fe80::1 IPv6 link-local is private", "fe80::1", true},
		{"2001:db8::1 IPv6 documentation range is public", "2001:db8::1", false},
		{"2606:4700::1 IPv6 Cloudflare is public", "2606:4700::1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", tt.ip)
			}
			got := IsPrivateIP(ip)
			if got != tt.want {
				t.Errorf("IsPrivateIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestCheckVLESSAvailability_ErrorCases(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantStatus string
		wantMsg    string
	}{
		{
			name:       "invalid URL",
			input:      "://bad",
			wantStatus: "down",
			wantMsg:    "invalid URL",
		},
		{
			name:       "non-vless scheme",
			input:      "https://example.com",
			wantStatus: "down",
			wantMsg:    "scheme is not vless",
		},
		{
			name:       "missing host",
			input:      "vless://:443",
			wantStatus: "down",
			wantMsg:    "missing host",
		},
		{
			name:       "private IP host is blocked",
			input:      "vless://uuid@127.0.0.1:443",
			wantStatus: "down",
			wantMsg:    "health check to private addresses is not allowed",
		},
		{
			name:       "private IP 10.0.0.1 is blocked",
			input:      "vless://uuid@10.0.0.1:443",
			wantStatus: "down",
			wantMsg:    "health check to private addresses is not allowed",
		},
		{
			name:       "private IP 192.168.1.1 is blocked",
			input:      "vless://uuid@192.168.1.1:443",
			wantStatus: "down",
			wantMsg:    "health check to private addresses is not allowed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, msg, latency := CheckVLESSAvailability(tt.input)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if !strings.Contains(msg, tt.wantMsg) {
				t.Errorf("msg = %q, want it to contain %q", msg, tt.wantMsg)
			}
			if latency != 0 {
				t.Errorf("latency = %d, want 0", latency)
			}
		})
	}
}
