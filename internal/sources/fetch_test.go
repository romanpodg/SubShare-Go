package sources

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// rewriteTransport keeps the public hostname the SSRF guard validated and
// routes the connection to a local test server instead.
type rewriteTransport struct{ target *url.URL }

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

func TestFetchParsesBodyAndHeadersThroughInjectedClient(t *testing.T) {
	var seenHWID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHWID = r.Header.Get("x-hwid")
		w.Header().Set("profile-title", "base64:"+base64.StdEncoding.EncodeToString([]byte("Provider feed")))
		w.Header().Set("profile-update-interval", "12")
		w.Header().Set("announce", "Welcome")
		_, _ = w.Write([]byte("vless://11111111-1111-4111-8111-111111111111@vless.example:443?security=tls#VLESS\n"))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	client := &http.Client{Transport: rewriteTransport{target: target}}
	key := []byte("0123456789abcdef0123456789abcdef")

	parsed, err := Fetch(context.Background(), client, "https://provider.example/sub", NormalizeHWIDProfile(true, "", "", "device-1"), [][]byte{key})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(parsed.Keys) != 1 || parsed.Keys[0].Protocol != "vless" {
		t.Fatalf("keys = %#v", parsed.Keys)
	}
	if parsed.Metadata.Title != "Provider feed" || parsed.Metadata.RefreshHours != 12 || parsed.Metadata.Announce != "Welcome" {
		t.Fatalf("metadata = %#v", parsed.Metadata)
	}
	if seenHWID != "device-1" {
		t.Fatalf("hwid header = %q", seenHWID)
	}
}

func TestFetchRejectsForbiddenHostsBeforeDialing(t *testing.T) {
	dialed := false
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		dialed = true
		return nil, http.ErrNotSupported
	})}
	for _, raw := range []string{"http://localhost/sub", "http://127.0.0.1/sub", "ftp://provider.example/sub", "https://user:pw@provider.example/sub"} {
		if _, err := Fetch(context.Background(), client, raw, HWIDProfile{}, [][]byte{[]byte("k")}); err == nil {
			t.Fatalf("%s: expected rejection", raw)
		}
	}
	if dialed {
		t.Fatal("forbidden URL reached the transport")
	}
}

func TestFetchRejectsOversizedBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", MaxBodyBytes+1)))
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	client := &http.Client{Transport: rewriteTransport{target: target}}
	_, err := Fetch(context.Background(), client, "https://provider.example/sub", HWIDProfile{}, [][]byte{[]byte("k")})
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("err = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
