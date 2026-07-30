package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---------- RateLimiter ----------

func TestRateLimiter_AllowUpToLimitThenBlock(t *testing.T) {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    3,
		window:   1 * time.Second,
	}

	ip := "1.2.3.4"
	for i := 0; i < 3; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rl.Allow(ip) {
		t.Error("4th request should be blocked")
	}
	if rl.Allow(ip) {
		t.Error("5th request should also be blocked")
	}
}

func TestRateLimiter_DifferentIPsAreIndependent(t *testing.T) {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    2,
		window:   1 * time.Second,
	}

	ip1 := "10.0.0.1"
	ip2 := "10.0.0.2"

	// Exhaust limit for ip1
	rl.Allow(ip1)
	rl.Allow(ip1)
	if rl.Allow(ip1) {
		t.Error("ip1 should be rate limited after 2 requests")
	}

	// ip2 should still be allowed
	if !rl.Allow(ip2) {
		t.Error("ip2 should be allowed, it has a separate counter")
	}
	if !rl.Allow(ip2) {
		t.Error("ip2 second request should be allowed")
	}
	if rl.Allow(ip2) {
		t.Error("ip2 third request should be blocked")
	}
}

func TestRateLimiter_BoundsPeerMemory(t *testing.T) {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    2,
		window:   time.Minute,
		maxPeers: 2,
	}
	if !rl.Allow("203.0.113.1") || !rl.Allow("203.0.113.2") {
		t.Fatal("initial peers should be allowed")
	}
	if rl.Allow("203.0.113.3") {
		t.Fatal("new peer should be rejected after capacity is reached")
	}
	if len(rl.attempts) != 2 {
		t.Fatalf("peer map size = %d, want 2", len(rl.attempts))
	}
}

func TestRateLimiter_WindowExpiration(t *testing.T) {
	window := 50 * time.Millisecond
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    2,
		window:   window,
	}

	ip := "5.5.5.5"
	rl.Allow(ip)
	rl.Allow(ip)
	if rl.Allow(ip) {
		t.Error("should be blocked after reaching limit")
	}

	// Wait for the window to expire
	time.Sleep(window + 10*time.Millisecond)

	if !rl.Allow(ip) {
		t.Error("should be allowed again after window expires")
	}
}

func TestRateLimiter_Wrap(t *testing.T) {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    1,
		window:   1 * time.Second,
	}

	var errorCode int
	var errorMsg string
	writeErr := func(w http.ResponseWriter, code int, msg string) {
		errorCode = code
		errorMsg = msg
		w.WriteHeader(code)
	}

	called := false
	handler := rl.Wrap(writeErr, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// First request should pass through
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	rec := httptest.NewRecorder()
	handler(rec, req)
	if !called {
		t.Error("handler should have been called for first request")
	}

	// Second request should be rate limited
	called = false
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	rec = httptest.NewRecorder()
	handler(rec, req)
	if called {
		t.Error("handler should not have been called when rate limited")
	}
	if errorCode != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, errorCode)
	}
	if errorMsg != "too many requests, try again later" {
		t.Errorf("unexpected error message: %q", errorMsg)
	}
}

// ---------- ClientIP ----------

func TestClientIP(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")
	tests := []struct {
		name       string
		xff        string
		xri        string
		remoteAddr string
		want       string
	}{
		{
			name:       "X-Forwarded-For single IP",
			xff:        "203.0.113.50",
			remoteAddr: "127.0.0.1:8080",
			want:       "203.0.113.50",
		},
		{
			name:       "X-Forwarded-For multiple IPs returns first",
			xff:        "203.0.113.50, 70.41.3.18, 150.172.238.178",
			remoteAddr: "127.0.0.1:8080",
			want:       "203.0.113.50",
		},
		{
			name:       "X-Real-IP used when no X-Forwarded-For",
			xri:        "198.51.100.22",
			remoteAddr: "127.0.0.1:8080",
			want:       "198.51.100.22",
		},
		{
			name:       "RemoteAddr used as fallback",
			remoteAddr: "192.0.2.1:54321",
			want:       "192.0.2.1",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "192.0.2.1",
			want:       "192.0.2.1",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			xff:        "10.0.0.1",
			xri:        "10.0.0.2",
			remoteAddr: "127.0.0.1:8080",
			want:       "10.0.0.1",
		},
		{
			name:       "X-Forwarded-For with spaces trimmed",
			xff:        "  203.0.113.50  ",
			remoteAddr: "127.0.0.1:8080",
			want:       "203.0.113.50",
		},
		{
			name:       "forwarding headers from an untrusted peer are ignored",
			xff:        "10.0.0.1",
			xri:        "10.0.0.2",
			remoteAddr: "198.51.100.25:443",
			want:       "198.51.100.25",
		},
		{
			name:       "private peer is not implicitly a trusted proxy",
			xff:        "203.0.113.50",
			remoteAddr: "10.10.10.10:443",
			want:       "10.10.10.10",
		},
		{
			name:       "invalid forwarding header is ignored",
			xff:        "not-an-ip",
			remoteAddr: "127.0.0.1:8080",
			want:       "127.0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			got := ClientIP(req)
			if got != tt.want {
				t.Errorf("ClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientIPUsesExplicitTrustedProxyNetwork(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.10.10.10:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.50")
	networks := parseTrustedProxyNetworks("10.0.0.0/8")
	if got := clientIP(request, networks); got != "203.0.113.50" {
		t.Fatalf("clientIP() = %q, want forwarded client", got)
	}
}

// ---------- SecurityHeaders ----------

func TestSecurityHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := SecurityHeaders(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
	}

	for header, want := range expectedHeaders {
		got := rec.Header().Get(header)
		if got != want {
			t.Errorf("header %q = %q, want %q", header, got, want)
		}
	}
}

func TestSanitizeLogPathRemovesSubscriptionAndHWIDSecrets(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"/sub/raw-subscription-id":                     "/sub/{subscription_id}",
		"/sub/raw-subscription-id/subbody/plain":       "/sub/{subscription_id}/subbody/plain",
		"/api/sub/raw-subscription-id/info":            "/api/sub/{subscription_id}/info",
		"/api/admin/users/12/hwid/raw-device-identity": "/api/admin/users/12/hwid/{hwid}",
		"/api/v1/users/12":                             "/api/v1/users/12",
	}
	for path, want := range tests {
		if got := sanitizeLogPath(path); got != want {
			t.Fatalf("sanitizeLogPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDeprecateLegacyAdminAPI(t *testing.T) {
	handler := DeprecateLegacyAdminAPI(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("Deprecation") != "true" {
		t.Fatal("legacy admin API was not marked deprecated")
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("Deprecation") != "" {
		t.Fatal("v1 API was incorrectly marked deprecated")
	}
}

// ---------- RequestID ----------

func TestRequestID_Generated(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request ID is in context
		id, ok := r.Context().Value(CtxKeyRequestID).(string)
		if !ok || id == "" {
			t.Error("expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := RequestID(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
	if len(id) != 32 {
		t.Errorf("generated request ID length = %d, want 32 hex chars", len(id))
	}

	// Verify it is valid hex
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("request ID contains non-hex character: %c", c)
			break
		}
	}
}

func TestRequestID_ProvidedByClient(t *testing.T) {
	customID := "my-custom-request-id-12345"

	var contextID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextID, _ = r.Context().Value(CtxKeyRequestID).(string)
		w.WriteHeader(http.StatusOK)
	})
	handler := RequestID(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", customID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	gotHeader := rec.Header().Get("X-Request-ID")
	if gotHeader != customID {
		t.Errorf("X-Request-ID header = %q, want %q", gotHeader, customID)
	}
	if contextID != customID {
		t.Errorf("context request ID = %q, want %q", contextID, customID)
	}
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequestID(inner)

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	id1 := rec1.Header().Get("X-Request-ID")

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	id2 := rec2.Header().Get("X-Request-ID")

	if id1 == id2 {
		t.Errorf("two requests should have different IDs, both got %q", id1)
	}
}

// ---------- CorsMiddleware ----------

func TestCorsMiddleware_AllowedOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CorsMiddleware([]string{"http://localhost:3000", "https://example.com"}, inner)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:3000")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods should be set")
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Errorf("Access-Control-Max-Age = %q, want %q", got, "86400")
	}
}

func TestCorsMiddleware_DisallowedOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CorsMiddleware([]string{"http://localhost:3000"}, inner)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	req.Header.Set("Origin", "http://evil.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin should be empty for disallowed origin, got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (non-OPTIONS request should still be served)", rec.Code, http.StatusOK)
	}
}

func TestCorsMiddleware_OptionsRequest(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := CorsMiddleware([]string{"https://example.com"}, inner)

	req := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if called {
		t.Error("inner handler should not be called for OPTIONS preflight")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://example.com")
	}
}

func TestCorsMiddleware_OptionsWithDisallowedOrigin(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	handler := CorsMiddleware([]string{"https://allowed.com"}, inner)

	req := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
	req.Header.Set("Origin", "https://notallowed.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// OPTIONS always returns 204 regardless of origin match
	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if called {
		t.Error("inner handler should not be called for OPTIONS")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("CORS headers should not be set for disallowed origin, got Allow-Origin = %q", got)
	}
}

func TestCorsMiddleware_NoOriginHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CorsMiddleware([]string{"http://localhost:3000"}, inner)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	// No Origin header set
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin should be empty when no Origin header, got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// ---------- StatusRecorder ----------

func TestStatusRecorder_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &StatusRecorder{ResponseWriter: rec, Status: 200}

	sr.WriteHeader(http.StatusNotFound)
	if sr.Status != http.StatusNotFound {
		t.Errorf("StatusRecorder.Status = %d, want %d", sr.Status, http.StatusNotFound)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("underlying recorder code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestStatusRecorder_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &StatusRecorder{ResponseWriter: rec, Status: 200}

	// Without calling WriteHeader, status should remain at default
	if sr.Status != 200 {
		t.Errorf("default StatusRecorder.Status = %d, want 200", sr.Status)
	}
}

func TestStatusRecorder_MultipleStatusCodes(t *testing.T) {
	statusCodes := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusBadRequest,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	}
	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			rec := httptest.NewRecorder()
			sr := &StatusRecorder{ResponseWriter: rec, Status: 200}
			sr.WriteHeader(code)
			if sr.Status != code {
				t.Errorf("StatusRecorder.Status = %d, want %d", sr.Status, code)
			}
		})
	}
}
