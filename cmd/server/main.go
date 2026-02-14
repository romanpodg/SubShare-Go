package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"xary-sub/internal/middleware"
	"xary-sub/internal/model"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dbPath := strings.TrimSpace(os.Getenv("DB_PATH"))
	if dbPath == "" {
		dbPath = "data/app.db"
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -4000",
		"PRAGMA mmap_size = 268435456",
		"PRAGMA temp_store = MEMORY",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("exec %s: %w", p, err)
		}
	}

	if err := migrate(db); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}

	adminUser := strings.TrimSpace(os.Getenv("ADMIN_USER"))
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPass := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	if adminPass == "" {
		log.Fatal("ADMIN_PASSWORD environment variable is required but not set")
	}

	adminPassHash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	deviceLimitMessage := strings.TrimSpace(os.Getenv("DEVICE_LIMIT_MESSAGE"))
	if deviceLimitMessage == "" {
		deviceLimitMessage = model.DefaultDeviceLimitMessage
	}

	baseURL := strings.TrimSpace(os.Getenv("BASE_URL"))
	baseURL = strings.TrimRight(baseURL, "/")

	var corsOrigins []string
	if raw := strings.TrimSpace(os.Getenv("CORS_ORIGINS")); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				corsOrigins = append(corsOrigins, o)
			}
		}
	}

	app := &App{
		db:                 db,
		adminUser:          adminUser,
		adminPassHash:      adminPassHash,
		deviceLimitMessage: deviceLimitMessage,
		baseURL:            baseURL,
		sessions:           make(map[string]model.AdminSession),
	}

	go app.cleanupExpiredSessions(5 * time.Minute)
	app.startBackup()

	loginLimiter := middleware.NewRateLimiter(5, 1*time.Minute)
	activationLimiter := middleware.NewRateLimiter(10, 1*time.Minute)
	subscriptionLimiter := middleware.NewRateLimiter(30, 1*time.Minute)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database unreachable")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	// Auth API
	mux.HandleFunc("POST /api/auth/login", loginLimiter.Wrap(writeError, app.apiLogin))
	mux.Handle("POST /api/auth/logout", app.requireAdmin(http.HandlerFunc(app.apiLogout)))
	mux.Handle("GET /api/auth/me", app.requireAdmin(http.HandlerFunc(app.apiMe)))

	// Users API
	mux.Handle("GET /api/admin/users", app.requireAdmin(http.HandlerFunc(app.apiListUsers)))
	mux.Handle("POST /api/admin/users", app.requireAdmin(http.HandlerFunc(app.apiCreateUser)))
	mux.Handle("DELETE /api/admin/users/{id}", app.requireAdmin(http.HandlerFunc(app.apiDeleteUser)))
	mux.Handle("PUT /api/admin/users/{id}/keys", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserKeys)))
	mux.Handle("PUT /api/admin/users/{id}/subscription", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserSubscription)))
	mux.Handle("PUT /api/admin/users/{id}/hwid", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserHWID)))
	mux.Handle("DELETE /api/admin/users/{id}/hwid/{hwid}", app.requireAdmin(http.HandlerFunc(app.apiDeleteUserHWID)))

	// Keys API
	mux.Handle("GET /api/admin/keys", app.requireAdmin(http.HandlerFunc(app.apiListKeys)))
	mux.Handle("POST /api/admin/keys", app.requireAdmin(http.HandlerFunc(app.apiCreateKey)))
	mux.Handle("PUT /api/admin/keys/{id}", app.requireAdmin(http.HandlerFunc(app.apiUpdateKey)))
	mux.Handle("DELETE /api/admin/keys/{id}", app.requireAdmin(http.HandlerFunc(app.apiDeleteKey)))
	mux.Handle("POST /api/admin/keys/{id}/check", app.requireAdmin(http.HandlerFunc(app.apiCheckKey)))
	mux.Handle("POST /api/admin/keys/check-all", app.requireAdmin(http.HandlerFunc(app.apiCheckAllKeys)))

	// Export API
	mux.Handle("GET /api/admin/export/users", app.requireAdmin(http.HandlerFunc(app.apiExportUsers)))
	mux.Handle("GET /api/admin/export/keys", app.requireAdmin(http.HandlerFunc(app.apiExportKeys)))

	// Subscription API
	mux.HandleFunc("POST /api/subscription/activate", activationLimiter.Wrap(writeError, app.apiActivateSubscription))

	// Subscription delivery (VPN clients hit this directly)
	mux.HandleFunc("GET /sub/{subscription_id}", subscriptionLimiter.Wrap(writeError, app.handleSubscription))

	// Serve frontend static files in production (if frontend/out exists)
	frontendDir := "frontend/out"
	if info, err := os.Stat(frontendDir); err == nil && info.IsDir() {
		log.Printf("Serving frontend from %s", frontendDir)
		mux.Handle("/", middleware.SPAFileServer(os.DirFS(frontendDir)))
	}

	addr := os.Getenv("PORT")
	if addr == "" {
		addr = ":8080"
	} else if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}

	var handler http.Handler = middleware.SecurityHeaders(middleware.RequestID(middleware.LogRequest(mux)))
	if len(corsOrigins) > 0 {
		handler = middleware.CorsMiddleware(corsOrigins, handler)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}()

	log.Printf("Server listening on %s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

func (a *App) startBackup() {
	backupPath := strings.TrimSpace(os.Getenv("BACKUP_PATH"))
	if backupPath == "" {
		return
	}

	intervalStr := strings.TrimSpace(os.Getenv("BACKUP_INTERVAL"))
	interval, err := time.ParseDuration(intervalStr)
	if err != nil || interval <= 0 {
		interval = 1 * time.Hour
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := a.db.Exec(`VACUUM INTO ?`, backupPath); err != nil {
				log.Printf("backup failed: %v", err)
			} else {
				log.Printf("backup completed to %s", backupPath)
			}
		}
	}()
}
