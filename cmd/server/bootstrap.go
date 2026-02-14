package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if err := os.MkdirAll("data", 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", filepath.ToSlash("data/app.db"))
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}

	tpls, err := template.ParseGlob(filepath.ToSlash("web/templates/*.html"))
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	adminUser := strings.TrimSpace(os.Getenv("ADMIN_USER"))
	if adminUser == "" {
		adminUser = "admin"
	}
	adminPass := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	if adminPass == "" {
		adminPass = "admin123"
		log.Printf("warning: ADMIN_PASSWORD is not set, using temporary default")
	}

	deviceLimitMessage := strings.TrimSpace(os.Getenv("DEVICE_LIMIT_MESSAGE"))
	if deviceLimitMessage == "" {
		deviceLimitMessage = defaultDeviceLimitMessage
	}

	app := &App{
		db:                 db,
		templates:          tpls,
		adminUser:          adminUser,
		adminPass:          adminPass,
		deviceLimitMessage: deviceLimitMessage,
		sessions:           make(map[string]AdminSession),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleRoot)
	mux.HandleFunc("/admin/login", app.handleAdminLogin)
	mux.Handle("/admin/logout", app.requireAdmin(http.HandlerFunc(app.handleAdminLogout)))
	mux.Handle("/admin", app.requireAdmin(http.HandlerFunc(app.handleAdmin)))
	mux.Handle("/admin/users", app.requireAdmin(http.HandlerFunc(app.handleCreateUser)))
	mux.Handle("/admin/users/", app.requireAdmin(http.HandlerFunc(app.handleUserActions)))
	mux.Handle("/admin/keys", app.requireAdmin(http.HandlerFunc(app.handleCreateKey)))
	mux.Handle("/admin/keys/check", app.requireAdmin(http.HandlerFunc(app.handleCheckAllKeys)))
	mux.Handle("/admin/keys/", app.requireAdmin(http.HandlerFunc(app.handleKeyActions)))
	mux.HandleFunc("/subscription", app.handleSubscriptionPage)
	mux.HandleFunc("/sub/", app.handleSubscription)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	addr := ":8080"
	log.Printf("xary-sub listening on %s", addr)
	return http.ListenAndServe(addr, logRequest(mux))
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
