# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Xray Sub — a Go-based VLESS VPN subscription management service with an admin panel. Manages users, VLESS keys, and personal subscriptions. End users activate subscriptions with a one-time code and retrieve their subscription URL. UI text is in Russian.

## Commands

```bash
# Install dependencies
go mod tidy

# Run the server (listens on :8080)
export ADMIN_USER=admin
export ADMIN_PASSWORD=admin
go run ./cmd/server

# Build
go build -o server ./cmd/server
```

There are no tests, linting, or CI configured.

## Environment Variables

- `ADMIN_USER` — admin login (default: `admin`)
- `ADMIN_PASSWORD` — admin password (default: `admin123`, unsafe for production)
- `DEVICE_LIMIT_MESSAGE` — custom message when HWID device limit is exceeded

## Architecture

Single-binary Go server using stdlib `net/http`, SQLite via `modernc.org/sqlite` (pure Go, no CGO), and server-side HTML templates.

All application code lives in `cmd/server/`:

| File | Role |
|------|------|
| `main.go` | Server setup, routing, static file serving |
| `handlers.go` | All HTTP handlers and business logic (~800 lines) |
| `repository.go` | SQLite queries and data access |
| `bootstrap.go` | Schema creation and migrations (column existence checks before ALTER TABLE) |
| `auth.go` | Session-based admin auth, CSRF token validation |
| `helpers.go` | VLESS URL parsing/validation, formatting utilities |
| `types.go` | Structs: `App`, `User`, `VLESSKey`, `UserDevice`, `AdminSession` |

The `App` struct is the central context — holds DB connection, templates, admin credentials, and an in-memory session map protected by `sync.Mutex`.

### Database

SQLite with WAL mode. Four tables: `users`, `vless_keys`, `user_keys` (M2M), `user_devices` (HWID tracking). Migrations run at startup in `bootstrap.go` and are idempotent (check column existence before adding).

### Key Patterns

- **Post-Redirect-Get** for all admin form submissions
- **CSRF tokens** validated on every POST/PUT/PATCH/DELETE via `auth.go`
- **HWID device limiting**: reads from `?hwid=`, `X-HWID`, or `X-Device-ID` header; enforces per-user `max_devices`
- **VLESS URL format**: `vless://uuid@host:port?params#fragment` — parsed and validated in `helpers.go`
- **Key health checks**: TCP connect with 4-second timeout to detect up/down status

### Route Groups

- `/admin/*` — protected by `requireAdmin` middleware; user/key CRUD, subscription management
- `/subscription` — client-facing activation page (one-time activation code → subscription URL)
- `/sub/{subscription_id}` — subscription delivery endpoint returning `text/plain` VLESS URLs
