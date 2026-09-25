package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

type ctxKey string

// CtxKeyRequestID is the context key used to store the request ID.
const CtxKeyRequestID ctxKey = "request_id"

var trustedProxyConfiguration struct {
	sync.RWMutex
	networks []*net.IPNet
}

// ConfigureTrustedProxyNetworks applies startup-validated proxy networks.
// It must be called before handlers start accepting requests.
func ConfigureTrustedProxyNetworks(prefixes []netip.Prefix) {
	networks := make([]*net.IPNet, 0, len(prefixes))
	for _, prefix := range prefixes {
		if _, network, err := net.ParseCIDR(prefix.String()); err == nil {
			networks = append(networks, network)
		}
	}
	trustedProxyConfiguration.Lock()
	trustedProxyConfiguration.networks = networks
	trustedProxyConfiguration.Unlock()
}

// RateLimiter implements a sliding-window per-IP rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
	maxPeers int
}

// NewRateLimiter creates a new RateLimiter with the given limit and window.
// It starts a background goroutine that periodically cleans up expired entries.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		maxPeers: 10000,
	}
	go rl.cleanup(5 * time.Minute)
	return rl
}

// Allow checks whether the given IP is allowed to make a request within the rate limit.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	attempts := rl.attempts[ip]
	if rl.maxPeers > 0 && len(attempts) == 0 && len(rl.attempts) >= rl.maxPeers {
		rl.evictLocked(cutoff)
	}
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.attempts[ip] = valid
		return false
	}

	rl.attempts[ip] = append(valid, now)
	return true
}

// evictLocked frees a slot for an unseen peer: first every entry whose newest
// attempt has aged out, then the least recently active one if the table is
// still full. Rejecting the newcomer instead would let anyone able to fill the
// table lock out every client it does not already contain until the periodic
// cleanup runs. Callers must hold rl.mu.
func (rl *RateLimiter) evictLocked(cutoff time.Time) {
	oldestIP := ""
	var oldest time.Time
	for ip, attempts := range rl.attempts {
		if len(attempts) == 0 {
			delete(rl.attempts, ip)
			continue
		}
		newest := attempts[len(attempts)-1]
		if !newest.After(cutoff) {
			delete(rl.attempts, ip)
			continue
		}
		if oldestIP == "" || newest.Before(oldest) {
			oldestIP, oldest = ip, newest
		}
	}
	if rl.maxPeers > 0 && len(rl.attempts) >= rl.maxPeers && oldestIP != "" {
		delete(rl.attempts, oldestIP)
	}
}

// Wrap wraps an http.HandlerFunc with rate limiting. It uses writeErrorFunc to
// write error responses, allowing callers to provide their own error handler.
func (rl *RateLimiter) Wrap(writeErrorFunc func(http.ResponseWriter, *http.Request, int, string), next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(ClientIP(r)) {
			writeErrorFunc(w, r, http.StatusTooManyRequests, "too many requests, try again later")
			return
		}
		next(w, r)
	}
}

func (rl *RateLimiter) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-rl.window)
		rl.mu.Lock()
		for ip, attempts := range rl.attempts {
			valid := attempts[:0]
			for _, t := range attempts {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.attempts, ip)
			} else {
				rl.attempts[ip] = valid
			}
		}
		rl.mu.Unlock()
	}
}

// TrustedProxy reports whether the immediate peer is explicitly listed in the
// startup-validated trusted proxy configuration.
func TrustedProxy(r *http.Request) bool {
	return trustedProxy(r, configuredTrustedProxyNetworks())
}

// ClientIP extracts the client IP from the request. Forwarding headers are
// accepted only when the immediate peer is explicitly trusted.
func ClientIP(r *http.Request) string {
	return clientIP(r, configuredTrustedProxyNetworks())
}

func configuredTrustedProxyNetworks() []*net.IPNet {
	trustedProxyConfiguration.RLock()
	defer trustedProxyConfiguration.RUnlock()
	return append([]*net.IPNet(nil), trustedProxyConfiguration.networks...)
}

func parseTrustedProxyNetworks(raw string) []*net.IPNet {
	networks := []*net.IPNet{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		if _, network, err := net.ParseCIDR(item); err == nil {
			networks = append(networks, network)
		}
	}
	return networks
}

func clientIP(r *http.Request, trustedNetworks []*net.IPNet) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	if trustedProxy(r, trustedNetworks) {
		if forwarded := forwardedClientIP(r, trustedNetworks); forwarded != "" {
			return forwarded
		}
	}
	return strings.TrimSpace(remoteHost)
}

// forwardedClientIP resolves the client address advertised by a trusted proxy,
// or "" when the proxy headers carry nothing usable.
func forwardedClientIP(r *http.Request, trustedNetworks []*net.IPNet) string {
	// Walk X-Forwarded-For right-to-left. Each trusted proxy appends the
	// peer it saw, so the rightmost entries are the ones we can vouch for
	// and the first non-trusted hop is the real client. Anything further
	// left was supplied by the client and is forgeable, so reading the
	// leftmost entry would let a caller choose its own rate-limit bucket.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for index := len(parts) - 1; index >= 0; index-- {
			candidate := strings.TrimSpace(parts[index])
			parsed := net.ParseIP(candidate)
			if parsed == nil {
				break
			}
			if ipInNetworks(parsed, trustedNetworks) {
				continue
			}
			return candidate
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(xri) != nil {
		return xri
	}
	return ""
}

func ipInNetworks(ip net.IP, networks []*net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func trustedProxy(r *http.Request, trustedNetworks []*net.IPNet) bool {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	remoteIP := net.ParseIP(strings.TrimSpace(remoteHost))
	if remoteIP == nil {
		return false
	}
	return ipInNetworks(remoteIP, trustedNetworks)
}

// SecurityHeaders adds standard security headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// StatusRecorder wraps http.ResponseWriter to capture the status code.
type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

// WriteHeader captures the status code before delegating to the wrapped ResponseWriter.
func (r *StatusRecorder) WriteHeader(code int) {
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
}

// RequestID adds a unique request ID to each request context and response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if !validRequestID(id) {
			buf := make([]byte, 16)
			_, _ = rand.Read(buf)
			id = hex.EncodeToString(buf)
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), CtxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validRequestID reports whether an inbound X-Request-ID is safe to echo back
// and to copy into every log line for the request. An unbounded caller-supplied
// value is reflected and logged verbatim, so a large one turns a cheap request
// into a large amount of log output.
func validRequestID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for index := 0; index < len(id); index++ {
		char := id[index]
		switch {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			char == '-', char == '_', char == '.':
		default:
			return false
		}
	}
	return true
}

// LogRequest logs each request with its method, path, duration, client IP, and request ID.
func LogRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &StatusRecorder{ResponseWriter: w, Status: 200}
		next.ServeHTTP(rec, r)
		reqID, _ := r.Context().Value(CtxKeyRequestID).(string)
		slog.Info("request completed",
			"status", rec.Status,
			"method", r.Method,
			"path", sanitizeLogPath(r.URL.Path),
			"duration", time.Since(start).String(),
			"ip", ClientIP(r),
			"req_id", reqID,
		)
	})
}

func sanitizeLogPath(path string) string {
	segments := strings.Split(path, "/")
	for index := 1; index < len(segments); index++ {
		switch {
		case segments[index] == "sub" && index+1 < len(segments):
			segments[index+1] = "{subscription_id}"
		case segments[index] == "hwid" && index+1 < len(segments):
			segments[index+1] = "{hwid}"
		}
	}
	return strings.Join(segments, "/")
}

// CorsMiddleware adds CORS headers for the specified origins.
func CorsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Set unconditionally: the response body and headers depend on Origin
		// whether or not this particular origin matched, so a shared cache must
		// key on it either way.
		w.Header().Add("Vary", "Origin")
		if _, ok := allowed[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// DeprecateLegacyAdminAPI marks the compatibility API without changing its
// payloads. Clients should migrate to /api/v1.
func DeprecateLegacyAdminAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/admin/") {
			w.Header().Set("Deprecation", "true")
			w.Header().Set("Link", `</api/v1/openapi.yaml>; rel="successor-version"`)
		}
		next.ServeHTTP(w, r)
	})
}

// SPAFileServer serves static files from root, falling back to index.html
// for paths that don't match a real file (single-page app behavior).
func SPAFileServer(root fs.FS) http.Handler {
	fileServer := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		f, err := root.Open(path)
		if err != nil {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}
