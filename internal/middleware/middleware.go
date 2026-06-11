package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ctxKey string

// CtxKeyRequestID is the context key used to store the request ID.
const CtxKeyRequestID ctxKey = "request_id"

// RateLimiter implements a sliding-window per-IP rate limiter.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new RateLimiter with the given limit and window.
// It starts a background goroutine that periodically cleans up expired entries.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
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

// Wrap wraps an http.HandlerFunc with rate limiting. It uses writeErrorFunc to
// write error responses, allowing callers to provide their own error handler.
func (rl *RateLimiter) Wrap(writeErrorFunc func(http.ResponseWriter, int, string), next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(ClientIP(r)) {
			writeErrorFunc(w, http.StatusTooManyRequests, "too many requests, try again later")
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

// ClientIP extracts the client IP from the request, checking
// X-Forwarded-For and X-Real-IP headers before falling back to RemoteAddr.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if parts := strings.SplitN(xff, ",", 2); len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
		if id == "" {
			buf := make([]byte, 16)
			_, _ = rand.Read(buf)
			id = hex.EncodeToString(buf)
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), CtxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
			"path", r.URL.Path,
			"duration", time.Since(start).String(),
			"ip", ClientIP(r),
			"req_id", reqID,
		)
	})
}

// CorsMiddleware adds CORS headers for the specified origins.
func CorsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
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
