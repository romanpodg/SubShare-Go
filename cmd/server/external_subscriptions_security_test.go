package main

import (
	"github.com/romanpodg/SubShare-Go/internal/sources"
	"net/http"
	"net/netip"
	"net/url"
	"testing"
)

func TestValidateExternalSourceURLRejectsInternalTargets(t *testing.T) {
	t.Parallel()

	rejected := []string{
		"http://localhost/sub",
		"http://service.localhost/sub",
		"http://127.0.0.1/sub",
		"http://10.0.0.1/sub",
		"http://172.16.0.1/sub",
		"http://192.168.1.1/sub",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/sub",
		"https://user:password@example.com/sub",
	}
	for _, raw := range rejected {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			if _, err := sources.ValidateURL(raw); err == nil {
				t.Fatalf("ValidateURL(%q) unexpectedly succeeded", raw)
			}
		})
	}
}

func TestValidateExternalSourceURLAcceptsPublicHTTPURL(t *testing.T) {
	t.Parallel()

	got, err := sources.ValidateURL("https://example.com/subscription")
	if err != nil {
		t.Fatalf("ValidateURL: %v", err)
	}
	if got != "https://example.com/subscription" {
		t.Fatalf("unexpected normalized URL: %q", got)
	}
}

func TestIsForbiddenExternalIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw       string
		forbidden bool
	}{
		{raw: "127.0.0.1", forbidden: true},
		{raw: "10.20.30.40", forbidden: true},
		{raw: "169.254.1.1", forbidden: true},
		{raw: "::1", forbidden: true},
		{raw: "fc00::1", forbidden: true},
		{raw: "1.1.1.1", forbidden: false},
		{raw: "2606:4700:4700::1111", forbidden: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.raw, func(t *testing.T) {
			t.Parallel()
			address := netip.MustParseAddr(test.raw)
			if got := sources.IsForbiddenIP(address); got != test.forbidden {
				t.Fatalf("IsForbiddenIP(%s) = %v, want %v", address, got, test.forbidden)
			}
		})
	}
}

func TestExternalClientRevalidatesRedirects(t *testing.T) {
	t.Parallel()
	client := sources.NewHTTPClient()
	privateURL, _ := url.Parse("http://127.0.0.1/private")
	request := &http.Request{URL: privateURL}
	if err := client.CheckRedirect(request, []*http.Request{{}, {}}); err == nil {
		t.Fatal("redirect to loopback was accepted")
	}

	publicURL, _ := url.Parse("https://example.com/sub")
	request.URL = publicURL
	via := make([]*http.Request, 5)
	if err := client.CheckRedirect(request, via); err == nil {
		t.Fatal("redirect chain longer than the limit was accepted")
	}
}
