package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/httpapi"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "modernc.org/sqlite"

	"github.com/romanpodg/SubShare-Go/internal/middleware"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/platform/configuration"
	adminpassword "github.com/romanpodg/SubShare-Go/internal/security/password"
	"github.com/romanpodg/SubShare-Go/internal/security/profilestorage"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if handled, err := handleCLI(os.Args); handled {
		if err != nil {
			slog.Error("cli command failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := run(); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	return bootstrapRuntime(func() (configuration.Config, error) {
		return configuration.Load(os.Environ())
	}, runConfigured)
}

func bootstrapRuntime(load func() (configuration.Config, error), start func(configuration.Config) error) error {
	config, err := load()
	if err != nil {
		return err
	}
	return start(config)
}

func runConfigured(config configuration.Config) error {
	db, err := openDatabase(config)
	if err != nil {
		return err
	}
	defer db.Close()

	app, err := buildApp(config, db)
	if err != nil {
		return err
	}
	app.startBackgroundWorkers(config)

	mux := http.NewServeMux()
	app.registerRoutes(mux)
	serveFrontend(mux)

	var handler http.Handler = middleware.SecurityHeaders(middleware.RequestID(middleware.LogRequest(middleware.DeprecateLegacyAdminAPI(mux))))
	if len(config.CORSOrigins) > 0 {
		handler = middleware.CorsMiddleware(config.CORSOrigins, handler)
	}
	return serve(config.ListenAddress, handler)
}

// openDatabase creates the DB directory and opens SQLite, retrying once after
// clearing WAL sidecars when the first attempt fails with a recoverable I/O
// error.
func openDatabase(config configuration.Config) (*sql.DB, error) {
	dbPath := config.DBPath

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create DB_PATH directory")
	}

	db, err := initializeSQLiteWithJournalMode(dbPath, config.SQLiteJournalMode, config.ProfileKeyring)
	if err == nil {
		return db, nil
	}
	if !storage.IsRecoverableSQLiteIO(err) {
		return nil, err
	}

	log.Printf("SQLite startup failed (%v). Cleaning up WAL sidecars and retrying once...", err)
	if cleanupErr := storage.CleanupSQLiteSidecars(dbPath); cleanupErr != nil {
		return nil, fmt.Errorf("recover sqlite sidecars: %w", cleanupErr)
	}

	db, err = initializeSQLiteWithJournalMode(dbPath, config.SQLiteJournalMode, config.ProfileKeyring)
	if err != nil {
		return nil, fmt.Errorf("initialize sqlite after sidecar cleanup: %w", err)
	}
	return db, nil
}

// buildApp verifies the database, seeds the owner account and assembles the
// App with its runtime configuration.
func buildApp(config configuration.Config, db *sql.DB) (*App, error) {
	if err := storage.VerifyStartupEnvelopesAndInvariants(context.Background(), db, config.ProfileKeyring); err != nil {
		return nil, fmt.Errorf("startup verification: %w", err)
	}

	passwordHasher := adminpassword.NewDefault()
	createdOwner, err := ensureBootstrapOwner(context.Background(), db, config.AdminUser, config.AdminPassword, passwordHasher)
	if err != nil {
		return nil, err
	}
	if createdOwner {
		log.Printf("Seeded root admin account: %s (role: owner)", config.AdminUser)
	}

	deviceLimitMessage := config.DeviceLimitMessage
	if deviceLimitMessage == "" {
		deviceLimitMessage = model.DefaultDeviceLimitMessage
	}

	middleware.ConfigureTrustedProxyNetworks(config.TrustedProxyNetworks)

	app := &App{
		db:                        db,
		dbPath:                    config.DBPath,
		backupPath:                config.BackupPath,
		deviceLimitMessage:        deviceLimitMessage,
		baseURL:                   config.BaseURL,
		happCryptoAPIURL:          config.HappCryptoAPIURL,
		subscriptionBodyEncoding:  config.SubscriptionBodyEncoding,
		adminPasswordHasher:       passwordHasher,
		profileFingerprintKey:     append([]byte(nil), config.ProfileFingerprintKey...),
		profileFingerprintOldKeys: cloneByteSlices(config.ProfileFingerprintOldKeys),
		profileKeyring:            config.ProfileKeyring,
	}
	app.recoverInterruptedJobs()
	return app, nil
}

func (a *App) startBackgroundWorkers(config configuration.Config) {
	go a.cleanupExpiredSessions(5 * time.Minute)
	a.startBackup(config.BackupPath, config.BackupInterval)
}

func (a *App) registerRoutes(mux *http.ServeMux) {
	loginLimiter := middleware.NewRateLimiter(5, 1*time.Minute)
	activationLimiter := middleware.NewRateLimiter(10, 1*time.Minute)
	subscriptionLimiter := middleware.NewRateLimiter(30, 1*time.Minute)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := a.db.Ping(); err != nil {
			httpapi.WriteError(w, r, http.StatusServiceUnavailable, "database unreachable")
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	// Auth API
	mux.HandleFunc("POST /api/auth/login", loginLimiter.Wrap(httpapi.WriteError, a.apiLogin))
	mux.Handle("POST /api/auth/logout", a.requireAdmin(http.HandlerFunc(a.apiLogout)))
	mux.Handle("GET /api/auth/me", a.requireAdmin(http.HandlerFunc(a.apiMe)))

	a.registerV1Routes(mux, activationLimiter)
	a.registerLegacyAdminRoutes(mux)

	// Subscription API
	mux.HandleFunc("POST /api/subscription/activate", activationLimiter.Wrap(httpapi.WriteError, a.apiActivateSubscription))

	// Subscription delivery (VPN clients hit this directly)
	mux.HandleFunc("GET /sub/{subscription_id}", subscriptionLimiter.Wrap(httpapi.WriteError, a.handleSubscription))
	mux.HandleFunc("GET /sub/{subscription_id}/subbody", subscriptionLimiter.Wrap(httpapi.WriteError, a.handleSubscriptionSubBody))
	mux.HandleFunc("GET /sub/{subscription_id}/subbody/plain", subscriptionLimiter.Wrap(httpapi.WriteError, a.handleSubscriptionSubBodyPlain))
	mux.HandleFunc("GET /api/sub/{subscription_id}/info", subscriptionLimiter.Wrap(httpapi.WriteError, a.apiGetSubscriptionInfo))
}

// registerV1Routes mounts the versioned admin API. Existing /api/admin routes
// remain available during the frontend migration.
func (a *App) registerV1Routes(mux *http.ServeMux, activationLimiter *middleware.RateLimiter) {
	mux.Handle("GET /api/v1/dashboard", a.requireAdmin(http.HandlerFunc(a.apiV1Dashboard)))
	mux.Handle("GET /api/v1/build-info", a.requireAdmin(http.HandlerFunc(a.apiV1BuildInfo)))
	mux.Handle("GET /api/v1/openapi.yaml", a.requireAdmin(http.HandlerFunc(a.apiV1OpenAPI)))
	mux.Handle("GET /api/v1/users", a.requireAdmin(http.HandlerFunc(a.apiV1ListUsers)))
	mux.Handle("POST /api/v1/users", a.requireAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiCreateUser))))
	mux.Handle("GET /api/v1/users/{id}", a.requireAdmin(http.HandlerFunc(a.apiV1GetUser)))
	mux.Handle("DELETE /api/v1/users/{id}", a.requireAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiDeleteUser))))
	mux.Handle("PUT /api/v1/users/{id}/keys", a.requireAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiUpdateUserKeys))))
	mux.Handle("PUT /api/v1/users/{id}/subscription", a.requireAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiUpdateUserSubscription))))
	mux.Handle("PATCH /api/v1/users/{id}/subscription", a.requireAdmin(http.HandlerFunc(a.apiV1PatchUserSubscription)))
	mux.Handle("PUT /api/v1/users/{id}/key-assignment", a.requireAdmin(http.HandlerFunc(a.apiV1UpdateUserKeyAssignment)))
	mux.Handle("PUT /api/v1/users/{id}/settings", a.requireAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiUpdateUserSettings))))
	mux.Handle("PUT /api/v1/users/{id}/hwid", a.requireAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiUpdateUserHWID))))
	a.registerKeyRoutes(mux)
	mux.Handle("GET /api/v1/sources", a.requireAdmin(http.HandlerFunc(a.apiV1ListSources)))
	mux.Handle("POST /api/v1/sources/preview", a.requireSuperAdmin(http.HandlerFunc(a.apiV1PreviewSource)))
	mux.Handle("POST /api/v1/sources", a.requireSuperAdmin(http.HandlerFunc(a.apiV1CreateSource)))
	mux.Handle("GET /api/v1/sources/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiV1GetSource)))
	mux.Handle("PUT /api/v1/sources/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiV1UpdateSource)))
	mux.Handle("DELETE /api/v1/sources/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiV1DeleteSource)))
	mux.Handle("POST /api/v1/sources/{id}/sync", a.requireSuperAdmin(http.HandlerFunc(a.apiV1QueueSourceSync)))
	mux.Handle("GET /api/v1/source-categories", a.requireAdmin(http.HandlerFunc(a.apiV1ListSourceCategories)))
	mux.Handle("GET /api/v1/audit-events", a.requireAdmin(http.HandlerFunc(a.apiV1ListAuditEvents)))
	mux.Handle("GET /api/v1/admins", a.requireSuperAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiListAdmins))))
	mux.Handle("POST /api/v1/admins", a.requireSuperAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiCreateAdmin))))
	mux.Handle("PUT /api/v1/admins/{id}", a.requireSuperAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiUpdateAdmin))))
	mux.Handle("DELETE /api/v1/admins/{id}", a.requireSuperAdmin(httpapi.V1Envelope(http.HandlerFunc(a.apiDeleteAdmin))))
	mux.Handle("GET /api/v1/templates", a.requireAdmin(http.HandlerFunc(a.apiV1ListTemplates)))
	mux.Handle("POST /api/v1/templates/preview", a.requireSuperAdmin(http.HandlerFunc(a.apiV1PreviewTemplate)))
	mux.Handle("POST /api/v1/templates", a.requireSuperAdmin(http.HandlerFunc(a.apiV1CreateTemplate)))
	mux.Handle("PUT /api/v1/templates/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiV1UpdateTemplate)))
	mux.Handle("DELETE /api/v1/templates/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiV1DeleteTemplate)))
	mux.Handle("GET /api/v1/response-rules", a.requireAdmin(http.HandlerFunc(a.apiV1ListResponseRules)))
	mux.Handle("POST /api/v1/response-rules", a.requireSuperAdmin(http.HandlerFunc(a.apiV1CreateResponseRule)))
	mux.Handle("PUT /api/v1/response-rules/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiV1UpdateResponseRule)))
	mux.Handle("DELETE /api/v1/response-rules/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiV1DeleteResponseRule)))
	mux.Handle("GET /api/v1/subscription-delivery-settings", a.requireAdmin(http.HandlerFunc(a.apiV1GetSubscriptionDeliverySettings)))
	mux.Handle("PUT /api/v1/subscription-delivery-settings", a.requireSuperAdmin(http.HandlerFunc(a.apiV1UpdateSubscriptionDeliverySettings)))
	mux.Handle("GET /api/v1/api-tokens", a.requireSuperAdmin(http.HandlerFunc(a.apiV1ListAPITokens)))
	mux.Handle("POST /api/v1/api-tokens", a.requireSuperAdmin(http.HandlerFunc(a.apiV1CreateAPIToken)))
	mux.Handle("DELETE /api/v1/api-tokens/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiV1RevokeAPIToken)))
	mux.Handle("GET /api/v1/jobs", a.requireAdmin(http.HandlerFunc(a.apiV1ListJobs)))
	mux.Handle("GET /api/v1/jobs/{id}", a.requireAdmin(http.HandlerFunc(a.apiV1GetJob)))
	mux.Handle("POST /api/v1/jobs/{id}/retry", a.requireSuperAdmin(http.HandlerFunc(a.apiV1RetryJob)))
	mux.Handle("GET /api/v1/sources/{id}/sync-runs", a.requireAdmin(http.HandlerFunc(a.apiV1ListSourceSyncRuns)))
	// Transitional v1 adapters for admin screens that still need the richer
	// legacy response shape. No frontend request should depend on /api/admin.
	mux.Handle("GET /api/v1/users/full", a.requireAdmin(http.HandlerFunc(a.apiListUsers)))
	mux.Handle("GET /api/v1/users/{id}/subscription-urls", a.requireAdmin(http.HandlerFunc(a.apiGetUserSubscriptionURLs)))
	mux.Handle("DELETE /api/v1/users/{id}/hwid/{hwid}", a.requireAdmin(http.HandlerFunc(a.apiDeleteUserHWID)))
	mux.Handle("GET /api/v1/subscription-settings", a.requireAdmin(http.HandlerFunc(a.apiGetSubscriptionSettings)))
	mux.Handle("PUT /api/v1/subscription-settings", a.requireSuperAdmin(http.HandlerFunc(a.apiUpdateSubscriptionSettings)))
	mux.Handle("GET /api/v1/routing-settings", a.requireAdmin(http.HandlerFunc(a.apiGetRoutingSettings)))
	mux.Handle("PUT /api/v1/routing-settings", a.requireSuperAdmin(http.HandlerFunc(a.apiUpdateRoutingSettings)))
	mux.HandleFunc("GET /api/v1/panel-settings", a.apiGetPanelSettings)
	mux.Handle("PUT /api/v1/panel-settings", a.requireSuperAdmin(http.HandlerFunc(a.apiUpdatePanelSettings)))
	mux.HandleFunc("GET /api/v1/subscription-page-config", a.apiGetSubscriptionPageConfig)
	mux.Handle("PUT /api/v1/subscription-page-config", a.requireSuperAdmin(http.HandlerFunc(a.apiUpdateSubscriptionPageConfig)))
	mux.HandleFunc("POST /api/v1/subscriptions/activate", activationLimiter.Wrap(httpapi.WriteError, a.apiActivateSubscription))
}

// registerLegacyAdminRoutes mounts the pre-v1 /api/admin surface that the
// frontend migration still depends on.
func (a *App) registerLegacyAdminRoutes(mux *http.ServeMux) {
	// Admins Management API (Super Admin only)
	mux.Handle("GET /api/admin/admins", a.requireSuperAdmin(http.HandlerFunc(a.apiListAdmins)))
	mux.Handle("POST /api/admin/admins", a.requireSuperAdmin(http.HandlerFunc(a.apiCreateAdmin)))
	mux.Handle("PUT /api/admin/admins/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiUpdateAdmin)))
	mux.Handle("DELETE /api/admin/admins/{id}", a.requireSuperAdmin(http.HandlerFunc(a.apiDeleteAdmin)))

	// Users API
	mux.Handle("GET /api/admin/users", a.requireAdmin(http.HandlerFunc(a.apiListUsers)))
	mux.Handle("POST /api/admin/users", a.requireAdmin(http.HandlerFunc(a.apiCreateUser)))
	mux.Handle("DELETE /api/admin/users/{id}", a.requireAdmin(http.HandlerFunc(a.apiDeleteUser)))
	mux.Handle("PUT /api/admin/users/{id}/keys", a.requireAdmin(http.HandlerFunc(a.apiUpdateUserKeys)))
	mux.Handle("PUT /api/admin/users/{id}/subscription", a.requireAdmin(http.HandlerFunc(a.apiUpdateUserSubscription)))
	mux.Handle("GET /api/admin/users/{id}/subscription-urls", a.requireAdmin(http.HandlerFunc(a.apiGetUserSubscriptionURLs)))
	mux.Handle("PUT /api/admin/users/{id}/settings", a.requireAdmin(http.HandlerFunc(a.apiUpdateUserSettings)))
	mux.Handle("PUT /api/admin/users/{id}/hwid", a.requireAdmin(http.HandlerFunc(a.apiUpdateUserHWID)))
	mux.Handle("DELETE /api/admin/users/{id}/hwid/{hwid}", a.requireAdmin(http.HandlerFunc(a.apiDeleteUserHWID)))

	// External Sources API (Super Admin only)
	mux.Handle("GET /api/admin/external-sources/categories", a.requireSuperAdmin(http.HandlerFunc(a.apiListExternalSourceCategories)))
	mux.Handle("POST /api/admin/external-sources/categories", a.requireSuperAdmin(http.HandlerFunc(a.apiCreateExternalSourceCategory)))
	mux.Handle("PUT /api/admin/external-sources/categories/rename", a.requireSuperAdmin(http.HandlerFunc(a.apiRenameExternalSourceCategory)))

	// Export API
	mux.Handle("GET /api/admin/export/users", a.requireAdmin(http.HandlerFunc(a.apiExportUsers)))
	mux.Handle("GET /api/admin/subscription-settings", a.requireAdmin(http.HandlerFunc(a.apiGetSubscriptionSettings)))
	mux.Handle("PUT /api/admin/subscription-settings", a.requireSuperAdmin(http.HandlerFunc(a.apiUpdateSubscriptionSettings)))
	mux.Handle("GET /api/admin/routing-settings", a.requireAdmin(http.HandlerFunc(a.apiGetRoutingSettings)))
	mux.Handle("PUT /api/admin/routing-settings", a.requireSuperAdmin(http.HandlerFunc(a.apiUpdateRoutingSettings)))

	// Panel settings API (GET is public so login/subscription pages can load branding)
	mux.HandleFunc("GET /api/panel-settings", a.apiGetPanelSettings)
	mux.HandleFunc("GET /api/subscription-page-config", a.apiGetSubscriptionPageConfig)
	mux.Handle("PUT /api/admin/panel-settings", a.requireSuperAdmin(http.HandlerFunc(a.apiUpdatePanelSettings)))
	mux.Handle("GET /api/admin/subscription-page-config", a.requireAdmin(http.HandlerFunc(a.apiGetSubscriptionPageConfig)))
	mux.Handle("PUT /api/admin/subscription-page-config", a.requireSuperAdmin(http.HandlerFunc(a.apiUpdateSubscriptionPageConfig)))
}

// serveFrontend serves the built frontend in production (if frontend/out exists).
func serveFrontend(mux *http.ServeMux) {
	frontendDir := "frontend/out"
	if info, err := os.Stat(frontendDir); err == nil && info.IsDir() {
		log.Printf("Serving frontend from %s", frontendDir)
		mux.Handle("/", middleware.SPAFileServer(os.DirFS(frontendDir)))
	}
}

// serve runs the HTTP server until SIGINT/SIGTERM, then shuts it down
// gracefully.
func serve(addr string, handler http.Handler) error {
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
	data, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	if err != nil {
		return nil, err
	}
	kr, err := profilestorage.LoadKeyringJSON(data)
	if err != nil {
		return nil, err
	}
	return initializeSQLiteWithJournalMode(dbPath, configuration.DefaultSQLiteJournalMode, kr)
}

func initializeSQLiteWithJournalMode(dbPath, journalMode string, keyring *profilestorage.Keyring) (*sql.DB, error) {
	return storage.InitializeSQLiteWithJournalMode(dbPath, journalMode, keyring)
}

func handleCLI(args []string) (bool, error) {
	if len(args) < 2 {
		return false, nil
	}
	cmd := args[1]
	switch cmd {
	case "bootstrap":
		if len(args) >= 3 && args[2] == "keyring" {
			targetPath := "data/keyring.json"
			if len(args) >= 4 {
				targetPath = args[3]
			}
			return true, bootstrapKeyring(targetPath)
		}
	case "maintenance":
		if len(args) >= 3 && args[2] == "vacuum" {
			return true, maintenanceVacuum()
		}
	case "validate-backup":
		if len(args) < 3 {
			return true, fmt.Errorf("usage: validate-backup <backup.db>")
		}
		backupPath := args[2]
		cfg, err := configuration.Load(os.Environ())
		if err != nil {
			return true, err
		}
		if cfg.ProfileKeyring == nil {
			return true, profilestorage.ErrMissingKeyring
		}
		return true, runValidateBackup(backupPath, cfg.ProfileKeyring)
	}
	return false, nil
}

// bootstrapKeyring writes a fresh keyring to targetPath unless one already
// exists there.
func bootstrapKeyring(targetPath string) error {
	if _, err := os.Stat(targetPath); err == nil {
		fmt.Printf("Keyring already exists at %s; keeping it unchanged.\n", targetPath)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat keyring file: %w", err)
	}
	data, err := profilestorage.GenerateKeyringJSON("key-1", "bik-1")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		fmt.Printf("Keyring already exists at %s; keeping it unchanged.\n", targetPath)
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	fmt.Printf("Generated keyring at %s\n", targetPath)
	return nil
}

// maintenanceVacuum runs PRAGMA vacuum and a WAL checkpoint on the configured
// database.
func maintenanceVacuum() error {
	cfg, err := configuration.Load(os.Environ())
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", filepath.ToSlash(cfg.DBPath))
	if err != nil {
		return err
	}
	defer db.Close()
	log.Println("Executing PRAGMA vacuum...")
	if _, err := db.Exec("PRAGMA vacuum"); err != nil {
		return fmt.Errorf("vacuum failed: %w", err)
	}
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("checkpoint failed: %w", err)
	}
	log.Println("Maintenance vacuum completed successfully.")
	return nil
}
