package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"subshare/internal/middleware"
	"subshare/internal/model"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
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

	db, err := initializeSQLite(dbPath)
	if err != nil {
		if !isRecoverableSQLiteIO(err) {
			return err
		}

		log.Printf("SQLite startup failed (%v). Cleaning up WAL sidecars and retrying once...", err)
		if cleanupErr := cleanupSQLiteSidecars(dbPath); cleanupErr != nil {
			return fmt.Errorf("recover sqlite sidecars: %w", cleanupErr)
		}

		db, err = initializeSQLite(dbPath)
		if err != nil {
			return fmt.Errorf("initialize sqlite after sidecar cleanup: %w", err)
		}
	}
	defer db.Close()

	adminUser := strings.TrimSpace(os.Getenv("ADMIN_USER"))
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPass := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	if adminPass == "" {
		slog.Error("ADMIN_PASSWORD environment variable is required but not set")
		os.Exit(1)
	}

	adminPassHash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	// Seed root admin if no admins exist
	var adminCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&adminCount); err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if adminCount == 0 {
		if _, err := db.Exec(`INSERT INTO admins (username, password_hash, role) VALUES (?, ?, 'super_admin')`, adminUser, string(adminPassHash)); err != nil {
			return fmt.Errorf("seed root admin: %w", err)
		}
		log.Printf("Seeded root admin account: %s (role: super_admin)", adminUser)
	}

	deviceLimitMessage := strings.TrimSpace(os.Getenv("DEVICE_LIMIT_MESSAGE"))
	if deviceLimitMessage == "" {
		deviceLimitMessage = model.DefaultDeviceLimitMessage
	}

	baseURL := strings.TrimSpace(os.Getenv("BASE_URL"))
	baseURL = strings.TrimRight(baseURL, "/")
	happCryptoAPIURL := strings.TrimSpace(os.Getenv("HAPP_CRYPTO_API_URL"))
	if happCryptoAPIURL == "" {
		happCryptoAPIURL = "https://crypto.happ.su/api-v2.php"
	}

	subscriptionBodyEncoding := strings.ToLower(strings.TrimSpace(os.Getenv("SUBSCRIPTION_BODY_ENCODING")))
	if subscriptionBodyEncoding == "" {
		subscriptionBodyEncoding = "base64"
	}
	if subscriptionBodyEncoding != "base64" {
		subscriptionBodyEncoding = "plain"
	}

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
		db:                       db,
		deviceLimitMessage:       deviceLimitMessage,
		baseURL:                  baseURL,
		happCryptoAPIURL:         happCryptoAPIURL,
		subscriptionBodyEncoding: subscriptionBodyEncoding,
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

	// Admins Management API (Super Admin only)
	mux.Handle("GET /api/admin/admins", app.requireSuperAdmin(http.HandlerFunc(app.apiListAdmins)))
	mux.Handle("POST /api/admin/admins", app.requireSuperAdmin(http.HandlerFunc(app.apiCreateAdmin)))
	mux.Handle("PUT /api/admin/admins/{id}", app.requireSuperAdmin(http.HandlerFunc(app.apiUpdateAdmin)))
	mux.Handle("DELETE /api/admin/admins/{id}", app.requireSuperAdmin(http.HandlerFunc(app.apiDeleteAdmin)))

	// Users API
	mux.Handle("GET /api/admin/users", app.requireAdmin(http.HandlerFunc(app.apiListUsers)))
	mux.Handle("POST /api/admin/users", app.requireAdmin(http.HandlerFunc(app.apiCreateUser)))
	mux.Handle("DELETE /api/admin/users/{id}", app.requireAdmin(http.HandlerFunc(app.apiDeleteUser)))
	mux.Handle("PUT /api/admin/users/{id}/keys", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserKeys)))
	mux.Handle("PUT /api/admin/users/{id}/subscription", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserSubscription)))
	mux.Handle("GET /api/admin/users/{id}/subscription-urls", app.requireAdmin(http.HandlerFunc(app.apiGetUserSubscriptionURLs)))
	mux.Handle("PUT /api/admin/users/{id}/settings", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserSettings)))
	mux.Handle("PUT /api/admin/users/{id}/hwid", app.requireAdmin(http.HandlerFunc(app.apiUpdateUserHWID)))
	mux.Handle("DELETE /api/admin/users/{id}/hwid/{hwid}", app.requireAdmin(http.HandlerFunc(app.apiDeleteUserHWID)))

	// Keys API
	mux.Handle("GET /api/admin/keys", app.requireAdmin(http.HandlerFunc(app.apiListKeys)))
	mux.Handle("GET /api/admin/key-categories", app.requireAdmin(http.HandlerFunc(app.apiListKeyCategories)))
	mux.Handle("POST /api/admin/key-categories", app.requireAdmin(http.HandlerFunc(app.apiCreateKeyCategory)))
	mux.Handle("PUT /api/admin/key-categories", app.requireAdmin(http.HandlerFunc(app.apiUpdateKeyCategory)))
	mux.Handle("PUT /api/admin/key-categories/order", app.requireAdmin(http.HandlerFunc(app.apiReorderKeyCategories)))
	mux.Handle("PUT /api/admin/key-categories/rename", app.requireAdmin(http.HandlerFunc(app.apiRenameKeyCategory)))
	mux.Handle("POST /api/admin/key-categories/delete", app.requireAdmin(http.HandlerFunc(app.apiDeleteKeyCategory)))
	mux.Handle("POST /api/admin/keys", app.requireAdmin(http.HandlerFunc(app.apiCreateKey)))
	mux.Handle("POST /api/admin/keys/bulk/status", app.requireAdmin(http.HandlerFunc(app.apiBulkUpdateKeyStatus)))
	mux.Handle("POST /api/admin/keys/bulk/delete", app.requireAdmin(http.HandlerFunc(app.apiBulkDeleteKeys)))
	mux.Handle("PUT /api/admin/keys/order", app.requireAdmin(http.HandlerFunc(app.apiReorderKeys)))
	mux.Handle("PUT /api/admin/keys/{id}", app.requireAdmin(http.HandlerFunc(app.apiUpdateKey)))
	mux.Handle("DELETE /api/admin/keys/{id}", app.requireAdmin(http.HandlerFunc(app.apiDeleteKey)))
	mux.Handle("POST /api/admin/keys/{id}/check", app.requireAdmin(http.HandlerFunc(app.apiCheckKey)))
	mux.Handle("POST /api/admin/keys/check-all", app.requireAdmin(http.HandlerFunc(app.apiCheckAllKeys)))

	// External Sources API (Super Admin only)
	mux.Handle("GET /api/admin/external-sources", app.requireSuperAdmin(http.HandlerFunc(app.apiListExternalSources)))
	mux.Handle("GET /api/admin/external-sources/categories", app.requireSuperAdmin(http.HandlerFunc(app.apiListExternalSourceCategories)))
	mux.Handle("POST /api/admin/external-sources/categories", app.requireSuperAdmin(http.HandlerFunc(app.apiCreateExternalSourceCategory)))
	mux.Handle("PUT /api/admin/external-sources/categories/rename", app.requireSuperAdmin(http.HandlerFunc(app.apiRenameExternalSourceCategory)))
	mux.Handle("POST /api/admin/external-sources/preview", app.requireSuperAdmin(http.HandlerFunc(app.apiPreviewExternalSource)))
	mux.Handle("POST /api/admin/external-sources/import", app.requireSuperAdmin(http.HandlerFunc(app.apiImportExternalSource)))
	mux.Handle("PUT /api/admin/external-sources/{id}", app.requireSuperAdmin(http.HandlerFunc(app.apiUpdateExternalSource)))
	mux.Handle("DELETE /api/admin/external-sources/{id}", app.requireSuperAdmin(http.HandlerFunc(app.apiDeleteExternalSource)))
	mux.Handle("POST /api/admin/external-sources/{id}/sync", app.requireSuperAdmin(http.HandlerFunc(app.apiSyncExternalSource)))

	// Export API
	mux.Handle("GET /api/admin/export/users", app.requireAdmin(http.HandlerFunc(app.apiExportUsers)))
	mux.Handle("GET /api/admin/export/keys", app.requireAdmin(http.HandlerFunc(app.apiExportKeys)))
	mux.Handle("GET /api/admin/subscription-settings", app.requireAdmin(http.HandlerFunc(app.apiGetSubscriptionSettings)))
	mux.Handle("PUT /api/admin/subscription-settings", app.requireSuperAdmin(http.HandlerFunc(app.apiUpdateSubscriptionSettings)))
	mux.Handle("GET /api/admin/routing-settings", app.requireAdmin(http.HandlerFunc(app.apiGetRoutingSettings)))
	mux.Handle("PUT /api/admin/routing-settings", app.requireSuperAdmin(http.HandlerFunc(app.apiUpdateRoutingSettings)))

	// Panel settings API (GET is public so login/subscription pages can load branding)
	mux.HandleFunc("GET /api/panel-settings", app.apiGetPanelSettings)
	mux.HandleFunc("GET /api/subscription-page-config", app.apiGetSubscriptionPageConfig)
	mux.Handle("PUT /api/admin/panel-settings", app.requireSuperAdmin(http.HandlerFunc(app.apiUpdatePanelSettings)))
	mux.Handle("GET /api/admin/subscription-page-config", app.requireAdmin(http.HandlerFunc(app.apiGetSubscriptionPageConfig)))
	mux.Handle("PUT /api/admin/subscription-page-config", app.requireSuperAdmin(http.HandlerFunc(app.apiUpdateSubscriptionPageConfig)))

	// Subscription API
	mux.HandleFunc("POST /api/subscription/activate", activationLimiter.Wrap(writeError, app.apiActivateSubscription))

	// Subscription delivery (VPN clients hit this directly)
	mux.HandleFunc("GET /sub/{subscription_id}", subscriptionLimiter.Wrap(writeError, app.handleSubscription))
	mux.HandleFunc("GET /sub/{subscription_id}/subbody", subscriptionLimiter.Wrap(writeError, app.handleSubscriptionSubBody))
	mux.HandleFunc("GET /sub/{subscription_id}/subbody/plain", subscriptionLimiter.Wrap(writeError, app.handleSubscriptionSubBodyPlain))
	mux.HandleFunc("GET /api/sub/{subscription_id}/info", subscriptionLimiter.Wrap(writeError, app.apiGetSubscriptionInfo))

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

func initializeSQLite(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", filepath.ToSlash(dbPath))
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)

	if err := configureSQLitePragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return db, nil
}

func configureSQLitePragmas(db *sql.DB) error {
	journalMode := normalizeSQLiteJournalMode(os.Getenv("DB_JOURNAL_MODE"))

	// Some Docker bind mounts (especially non-native Linux filesystems) do not
	// support SQLite WAL shared-memory file resizing and fail with IOERR_SHMSIZE.
	// In that case we transparently fall back to DELETE mode.
	if _, err := db.Exec(fmt.Sprintf("PRAGMA journal_mode = %s", journalMode)); err != nil {
		if journalMode != "WAL" {
			return fmt.Errorf("set journal mode %s: %w", journalMode, err)
		}
		log.Printf("WAL mode unavailable (%v), falling back to DELETE", err)
		if _, fallbackErr := db.Exec("PRAGMA journal_mode = DELETE"); fallbackErr != nil {
			return fmt.Errorf("set journal mode fallback DELETE: %w", fallbackErr)
		}
	}

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -4000",
		"PRAGMA mmap_size = 268435456",
		"PRAGMA temp_store = MEMORY",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("exec %s: %w", pragma, err)
		}
	}

	return nil
}

func normalizeSQLiteJournalMode(raw string) string {
	mode := strings.ToUpper(strings.TrimSpace(raw))
	switch mode {
	case "", "WAL":
		return "WAL"
	case "DELETE", "TRUNCATE", "PERSIST", "MEMORY", "OFF":
		return mode
	default:
		return "WAL"
	}
}

func cleanupSQLiteSidecars(dbPath string) error {
	for _, suffix := range []string{"-shm", "-wal"} {
		if err := os.Remove(filepath.ToSlash(dbPath) + suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func isRecoverableSQLiteIO(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "disk i/o error") || strings.Contains(msg, "(4874)")
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
