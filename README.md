# SubShare

SubShare представляет собой лёгкий self-hosted Subscription Hub для управления общей подпиской, внутри которой присутствуют VPN-конфигурации, которые можно распределять между польхователями. Проект состоит из Go API, SQLite и маршрутизируемой панели на Next.js. 

Репозиторий: `https://github.com/romanpodg/SubShare-Go`.

SubShare не управляет Xray-нодами и не измеряет фактический VPN-трафик. Его задача состоит в хранении, агрегации и выдаче конфигурации подходящего формата конкретному клиенту.

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
- Docker Compose, Caddy с автоматическим HTTPS

## Быстрый запуск

Для production-запуска нужны только Git и Docker Compose v2. Go, Node.js,
Caddy и SQLite устанавливаются внутри контейнеров.

Ubuntu:

```bash
git clone https://github.com/romanpodg/SubShare-Go.git
cd SubShare-Go
bash scripts/start.sh
```

Windows 10/11 с Docker Desktop в режиме Linux containers:

```powershell
git clone https://github.com/romanpodg/SubShare-Go.git
Set-Location SubShare-Go
powershell -ExecutionPolicy Bypass -File .\scripts\start.ps1
```

При первом запуске скрипт создаёт закрытый `.env`, генерирует стойкий пароль
owner, собирает образы, выполняет миграции и ждёт готовности сервисов. По
умолчанию панель открывается на `http://localhost/admin/login`.

Для публичного HTTPS укажите при первом запросе адрес вида
`https://vpn.example.com`. DNS домена должен указывать на сервер, а порты 80 и
443 должны быть открыты. Caddy самостоятельно получает и обновляет сертификаты.

Повторный запуск тем же скриптом сохраняет `.env`, БД и сертификаты. Основные
операции:

```bash
docker compose logs -f
docker compose restart
docker compose down
```

`docker compose down` сохраняет named volumes. Не используйте `down -v`, если
не хотите безвозвратно удалить БД и TLS-состояние.

### Локальная разработка

Для разработки без production-контейнеров требуются Go 1.24+, Node.js 22+ и
npm. Запустите backend с `ADMIN_PASSWORD`, затем `npm ci && npm run dev` в
`frontend/`. Next.js проксирует `/api/*` и `/sub/*` на `http://localhost:8080`.
Начальный `owner` создаётся только когда таблица администраторов пуста.

## Переменные окружения

| Переменная | Назначение |
|---|---|
| `ADMIN_USER` | Логин первого owner; по умолчанию `admin` |
| `ADMIN_PASSWORD` | Обязательный пароль первого owner; значение не выводится в startup-ошибках |
| `APP_ENV` | `development` (по умолчанию) или `production`; production требует корректный `BASE_URL` |
| `PORT` | `8080` по умолчанию; номер `1–65535` или валидный `host:port` |
| `DB_PATH` | Файл SQLite; по умолчанию `data/app.db`; в Compose — `/app/data/app.db` |
| `DB_JOURNAL_MODE` | `WAL` по умолчанию; только `WAL`, `DELETE`, `TRUNCATE`, `PERSIST`, `MEMORY`, `OFF` |
| `BASE_URL` | Необязательный публичный HTTP(S) origin без пути/query/fragment; обязателен в production |
| `CORS_ORIGINS` | HTTP(S) origins через запятую, без пути; пусто — CORS выключен |
| `TRUSTED_PROXIES` | IP/CIDR доверенных reverse proxy через запятую; пусто — forwarded-заголовки игнорируются |
| `DEVICE_LIMIT_MESSAGE` | Сообщение при превышении HWID-лимита; пусто — встроенное значение |
| `SUBSCRIPTION_BODY_ENCODING` | Legacy fallback: только `base64` (по умолчанию) или `plain`; response rules имеют приоритет |
| `HAPP_CRYPTO_API_URL` | Необязательный абсолютный HTTP(S) endpoint шифрования Happ без credentials; получает полную subscription URL |
| `BACKUP_PATH` | Необязательный путь консистентной резервной копии; пусто — backup-job выключен |
| `BACKUP_INTERVAL` | Положительная Go duration, например `1h`; по умолчанию `1h` |

Значения окружения нормализуются по пробелам и регистру только там, где это
документировано для enum. Любое непустое неподдерживаемое enum-значение,
некорректный порт, URL, IP/CIDR или duration останавливает запуск до открытия
SQLite, запуска фоновых задач и HTTP listener. Startup-ошибки называют
переменную, но не выводят пароли, URL с credentials, query-параметры или другие
секреты.

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

Backend немедленно создаёт и затем раз в `BACKUP_INTERVAL` атомарно обновляет
проверенную SQLite-копию внутри persistent volume. Экспортируйте её на хост:

```bash
bash scripts/backup.sh
```

```powershell
.\scripts\backup.ps1
```

Копии создаются в `backups/` и игнорируются Git. Перед восстановлением
остановите stack и сохраните текущую БД отдельно. Не копируйте активный
`app.db` напрямую: используйте только экспортированный backup. После
восстановления проверьте `/health`, вход и существующую subscription URL.

При первом переходе со старой bind-mount конфигурации start-скрипт обнаруживает
`data/app.db` и копирует его вместе с WAL sidecars в новый named volume, только
если volume ещё не содержит БД. Исходные файлы не изменяются и не удаляются.

## Модель угроз

- Внешние источники принимают только HTTP(S). Loopback, private, link-local, unspecified и другие специальные адреса запрещены после DNS resolution и на каждом redirect.
- Ответ источника имеет лимит размера и времени; redirects ограничены.
- `X-Forwarded-*` и `X-Real-IP` учитываются только от явно заданных `TRUSTED_PROXIES`.
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
