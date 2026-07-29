# SubShare

SubShare — лёгкий self-hosted Subscription Hub для управления пользователями, VPN-конфигурациями и персональными ссылками подписок. Проект состоит из Go API, SQLite и маршрутизируемой панели на Next.js. Интерфейс построен как тёмная техническая операционная система: связанная модульная сетка, инфраструктурные схемы, телеметрия и единый signal-green акцент.

Приватный upstream: `https://github.com/romanpodg/SubShare-Go`.

SubShare не управляет Xray-нодами и не измеряет фактический VPN-трафик. Его задача — безопасно хранить, агрегировать и выдавать конфигурации подходящего формата конкретному клиенту.

## Возможности

- Пользователи, сроки подписки и effective status: `active`, `expired`, `paused`, `blocked`, `limited`.
- Одноразовая активация, постоянные `/sub/{subscription_id}` и HWID-лимиты.
- VPN-ключи и информационные ключи, категории, порядок, массовые операции и проверка доступности ключей.
- Безопасный импорт внешних подписок с preview, историей синхронизаций и защитой от SSRF.
- Шаблоны `base64`, `plain`, `xray-json`, `mihomo` и `sing-box`.
- Ordered response rules: first match wins; условия по HTTP-заголовкам и User-Agent, regex, custom response headers и fallback.
- Dashboard, отдельные маршруты Users, Keys, Sources, Templates, Response Rules, Settings, Admins и Audit.
- Роли `owner`, `operator`, `viewer` и API-токены со scopes.
- Структурированный аудит без ключей, activation codes, subscription IDs и HWID.
- Версионные атомарные миграции SQLite и build-info.

## Стек и структура

```text
cmd/server/                 Go HTTP API, delivery pipeline и миграции
internal/                   модели, middleware и helpers
frontend/src/app/           Next.js App Router
frontend/src/components/    UI и admin-компоненты
frontend/out/               production static export (не хранится в Git)
data/app.db                 локальная SQLite БД (не хранится в Git)
cmd/server/openapi.yaml     OpenAPI 3.1 для /api/v1
```

- Go 1.24, `net/http`
- SQLite через `modernc.org/sqlite`
- Next.js 16, React 19, TypeScript, Tailwind CSS
- Docker Compose, nginx

## Быстрый запуск

Требуются Go 1.24+, Node.js 22+ и npm.

PowerShell:

```powershell
$env:ADMIN_USER = "admin"
$env:ADMIN_PASSWORD = "admin"
go run ./cmd/server
```

В отдельном терминале:

```powershell
Set-Location frontend
npm ci
npm run dev
```

Откройте `http://localhost:3000/admin/login`. Next.js dev server проксирует `/api/*` и `/sub/*` на `http://localhost:8080`.

Backend не запускается без `ADMIN_PASSWORD`. Начальный `owner` создаётся только когда таблица администраторов пуста.

## Docker

```bash
cp .env.example .env
# задайте ADMIN_PASSWORD, BASE_URL, DOMAIN и SSL_EMAIL
docker compose up -d --build
docker compose logs -f
```

Frontend публикуется на портах 80/443; backend доступен только внутри compose-сети. Каталог `data/` монтируется в контейнер и должен храниться на надёжном диске.

## Переменные окружения

| Переменная | Назначение |
|---|---|
| `ADMIN_USER` | Логин первого owner, по умолчанию `admin` |
| `ADMIN_PASSWORD` | Обязательный пароль первого owner |
| `PORT` | Порт Go API, по умолчанию `8080` |
| `DB_PATH` | Файл SQLite, по умолчанию `data/app.db` |
| `DB_JOURNAL_MODE` | `WAL` по умолчанию; также поддерживаются режимы SQLite из `.env.example` |
| `BASE_URL` | Публичный origin без завершающего `/` |
| `CORS_ORIGINS` | Разрешённые origins через запятую; пусто — CORS выключен |
| `TRUSTED_PROXIES` | IP/CIDR доверенных reverse proxy; loopback доверен автоматически |
| `DEVICE_LIMIT_MESSAGE` | Сообщение при превышении HWID-лимита |
| `SUBSCRIPTION_BODY_ENCODING` | Legacy fallback: `base64` или `plain`; response rules имеют приоритет |
| `HAPP_CRYPTO_API_URL` | Опциональный доверенный endpoint шифрования Happ; выключен по умолчанию, так как получает полную subscription URL |
| `BACKUP_PATH` | Путь вне репозитория для периодического `VACUUM INTO` |
| `BACKUP_INTERVAL` | Go duration, например `1h` |

Build metadata передаётся линкером:

```bash
go build -ldflags "-X main.version=1.2.0 -X main.commit=$(git rev-parse --short HEAD) -X main.buildTime=$(date -u +%FT%TZ)" ./cmd/server
```

## Маршруты

Панель:

```text
/admin/overview
/admin/users
/admin/keys
/admin/sources
/admin/templates
/admin/response-rules
/admin/settings/subscription
/admin/settings/branding
/admin/settings/security
/admin/admins
/admin/audit
```

Публичные endpoints:

- `POST /api/subscription/activate`
- `GET /sub/{subscription_id}` — основной обратно совместимый endpoint выдачи
- `GET /api/sub/{subscription_id}/info`
- `GET /health`

Основной admin API расположен под `/api/v1`:

- `GET /api/v1/dashboard`
- `GET|POST /api/v1/users`, `GET|DELETE /api/v1/users/{id}` и вложенные операции
- `GET|POST /api/v1/keys`, `PUT|DELETE /api/v1/keys/{id}`
- CRUD `/api/v1/sources`, `/templates`, `/response-rules`, `/api-tokens`
- `GET /api/v1/jobs`, `/audit-events`, `/build-info`
- `GET /api/v1/openapi.yaml`

Старые `/api/admin/*` пока сохранены как compatibility API на переходный релиз. Новая разработка должна использовать `/api/v1`.

## Шаблоны и response rules

Правила выполняются по `priority`, затем по `id`. Выигрывает первое совпадение. Пустой список условий соответствует любому запросу и подходит только для последнего fallback.

Поддерживаются операторы:

```text
EQUALS, NOT_EQUALS, CONTAINS, NOT_CONTAINS,
STARTS_WITH, NOT_STARTS_WITH, ENDS_WITH, NOT_ENDS_WITH,
REGEX, NOT_REGEX
```

Шаблон должен совпадать по формату с `response_type`. В custom content доступны `{{subscription}}` и `{{title}}`. Сервер отклоняет CRLF, опасные response headers и некорректные regex.

## Роли и API-токены

- `owner` — безопасность, администраторы, настройки и все операции.
- `operator` — ежедневная работа с пользователями и ключами.
- `viewer` — только чтение.

Секрет API-токена показывается один раз. В БД сохраняется SHA-256 hash. Доступные scopes: `read`, `users:write`, `keys:write`, `settings:write`.

```bash
curl -H "Authorization: Bearer ss_..." https://example.com/api/v1/dashboard
```

## Backup и восстановление

Не храните резервные копии в Git. Для консистентного backup используйте штатный `BACKUP_PATH` или SQLite CLI:

```bash
sqlite3 data/app.db ".backup '/secure/path/subshare-$(date +%F).db'"
sqlite3 /secure/path/subshare-2026-07-28.db "PRAGMA integrity_check;"
```

Перед восстановлением остановите backend, сохраните текущую БД отдельно, замените `data/app.db`, удалите только её `-wal`/`-shm` sidecars и запустите сервис. Проверьте `/health`, вход и существующую subscription URL.

## Модель угроз

- Внешние источники принимают только HTTP(S). Loopback, private, link-local, unspecified и другие специальные адреса запрещены после DNS resolution и на каждом redirect.
- Ответ источника имеет лимит размера и времени; redirects ограничены.
- `X-Forwarded-For` и `X-Real-IP` учитываются только от loopback или явно заданных `TRUSTED_PROXIES`.
- Unsafe browser-запросы требуют CSRF; cookies имеют HttpOnly/SameSite.
- Rate limiter ограничивает число хранимых IP.
- Audit и метрики не должны содержать subscription IDs, activation codes, ключи или HWID.

Если репозиторий с ранее отслеживаемыми бинарниками или `backups/*.db` когда-либо публиковался, простого удаления файлов недостаточно: очистите Git history и смените пароль owner, activation codes и subscription IDs.

## Проверки

```bash
go test ./...
go vet ./...

cd frontend
npm ci
npm run lint
npx tsc --noEmit
npm run build
```

CI также запрещает tracked runtime binaries, `.env` и SQLite backups. Критерий совместимости: все существующие `/sub/{subscription_id}` продолжают работать после миграций.
