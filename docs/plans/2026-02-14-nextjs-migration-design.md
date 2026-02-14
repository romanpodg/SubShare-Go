# Next.js Frontend Migration Design

**Date:** 2026-02-14
**Status:** Approved

## Summary

Migrate the Xray Sub frontend from Go server-rendered HTML templates to a Next.js SPA with Tailwind CSS. The Go backend becomes a pure JSON REST API. UI redesigned as clean minimalistic dark theme. Russian-only text.

## Architecture

**Two-process model:**
- Go API server (`:8080`) — JSON endpoints, session auth, SQLite, VLESS logic
- Next.js frontend (`:3000` in dev, static export in prod) — all UI rendering

The subscription delivery endpoint (`/sub/{subscription_id}`) stays on Go unchanged since VPN clients hit it directly.

## Go API Endpoints

All endpoints under `/api/` return JSON and accept JSON request bodies.

### Auth
| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/auth/login` | Login, returns session token |
| POST | `/api/auth/logout` | Logout |
| GET | `/api/auth/me` | Check session validity |

### Admin — Users
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/admin/users` | List all users with keys, HWIDs |
| POST | `/api/admin/users` | Create user |
| DELETE | `/api/admin/users/{id}` | Delete user |
| PUT | `/api/admin/users/{id}/keys` | Update assigned keys |
| PUT | `/api/admin/users/{id}/subscription` | Update status, dates, blocked reason |
| PUT | `/api/admin/users/{id}/hwid` | Update max devices |
| DELETE | `/api/admin/users/{id}/hwid/{hwid}` | Remove specific HWID |

### Admin — Keys
| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/admin/keys` | List all VLESS keys |
| POST | `/api/admin/keys` | Create key |
| PUT | `/api/admin/keys/{id}` | Edit key |
| DELETE | `/api/admin/keys/{id}` | Delete key |
| POST | `/api/admin/keys/{id}/check` | Health check single key |
| POST | `/api/admin/keys/check-all` | Health check all keys |

### Client
| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/subscription/activate` | Activate code, return subscription URL |
| GET | `/sub/{subscription_id}` | Plaintext VLESS delivery (unchanged) |

### Go Code Changes
- Remove `html/template` and template rendering
- Remove `web/templates/` and `web/static/`
- Replace form-data parsing with `json.Decoder`
- Replace redirect-with-query-params with JSON responses
- CSRF validated via `X-CSRF-Token` header instead of form field
- Session cookie unchanged (browser sends it automatically)
- Add CORS headers for dev (`localhost:3000` -> `localhost:8080`)

## Next.js Frontend Structure

Located in `frontend/` at repo root.

```
frontend/
├── src/
│   ├── app/
│   │   ├── layout.tsx          # Root layout (dark theme, fonts)
│   │   ├── page.tsx            # Redirect to /subscription
│   │   ├── admin/
│   │   │   ├── login/page.tsx  # Login page
│   │   │   └── page.tsx        # Admin panel
│   │   └── subscription/
│   │       └── page.tsx        # Client activation page
│   ├── components/
│   │   ├── ui/                 # Button, Input, Table, Card, Modal, Toast
│   │   ├── admin/              # UsersSection, UserRow, KeysSection, KeyRow, KeyAssigner, HwidManager
│   │   └── subscription/      # ActivationForm
│   ├── lib/
│   │   ├── api.ts              # Fetch wrapper (base URL, cookies, CSRF, errors)
│   │   └── types.ts            # TypeScript types matching Go structs
│   └── hooks/
│       └── useAuth.ts          # Auth state management
├── tailwind.config.ts
├── next.config.ts
├── package.json
└── tsconfig.json
```

**Key decisions:**
- App Router (Next.js 14+)
- Client-side rendering for admin (no SEO need)
- No state management library (React hooks + context sufficient)
- Centralized API wrapper handles auth, CSRF, error handling

## UI Design

### Color Palette
- Background: `#0a0a0f`
- Surface 1: `#13131a` (cards)
- Surface 2: `#1a1a24` (inputs, rows)
- Border: `#ffffff0a` (4% white)
- Text primary: `#e4e4e7` (zinc-200)
- Text secondary: `#71717a` (zinc-500)
- Accent: `#6366f1` (indigo-500)
- Success: `#22c55e`, Danger: `#ef4444`, Warning: `#f59e0b`

### Design Principles
- Minimal borders, generous whitespace
- Hierarchy through background levels, not decoration
- No gradients on surfaces
- Single accent color for interactive elements

### Pages

**Login:** Centered card, two inputs, one button. Error as red text below button.

**Admin panel:**
- Top bar: title left, nav link + logout right
- Users section: collapsible, clean table (name, email, code, status badge, dates, actions dropdown). "Add user" opens modal.
- Keys section: collapsible table (label, truncated URL + copy, status badge, health dot + latency, actions dropdown). "Add key" modal. "Check all" in header.
- Key assigner: modal with two-column drag-drop chips.
- Modals replace inline `<details>` dropdowns.
- Toast notifications: bottom-right, auto-dismiss.

**Subscription:** Centered card, heading, code input, button. Post-activation: monospace URL box with copy button.

## Development Setup

- Go on `:8080`, Next.js dev on `:3000`
- `next.config.ts` rewrites `/api/*` to `http://localhost:8080/api/*`
- No CORS needed in dev (proxied through Next.js)

## Production Deployment

- `next build` with `output: 'export'` for static files
- Go serves exported files from `frontend/out/` or both behind reverse proxy
- `/sub/{subscription_id}` served directly by Go

## What Gets Removed from Go
- Template loading/rendering in `main.go`
- `web/templates/*.html`
- `web/static/styles.css`
- Form-data parsing in handlers
- Redirect-with-query-params pattern

## What Stays Unchanged in Go
- `auth.go` — session logic (adapted for header-based CSRF)
- `repository.go` — all SQL queries
- `bootstrap.go` — schema migrations
- `helpers.go` — VLESS parsing, health checks
- `types.go` — data structures
- `/sub/{subscription_id}` endpoint
