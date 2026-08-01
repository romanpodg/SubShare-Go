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
owner (значение сохраняется в `.env`, но не выводится), собирает образы,
выполняет миграции и ждёт готовности сервисов. По умолчанию панель открывается
на `http://localhost/admin/login`.

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
npm. Для пустой БД запустите backend с `ADMIN_PASSWORD`, соответствующим
политике ниже; если администратор уже существует, переменную можно не задавать.
Каждый прямой backend startup также требует стабильный
`PROFILE_FINGERPRINT_KEY`; сгенерируйте 32 random bytes один раз, сохраните их
как 64 hexadecimal characters в локальном secret environment и не меняйте при
обычном restart.
Затем выполните `npm ci && npm run dev` в `frontend/`. Next.js проксирует
`/api/*` и `/sub/*` на `http://localhost:8080`. Начальный `owner` создаётся
только когда таблица администраторов пуста.

## Переменные окружения

| Переменная | Назначение |
|---|---|
| `ADMIN_USER` | Логин первого owner; по умолчанию `admin` |
| `ADMIN_PASSWORD` | Пароль первого owner: обязателен только при пустой таблице администраторов; 15–256 Unicode code points и не более 1024 UTF-8 bytes; пробелы сохраняются |
| `PROFILE_FINGERPRINT_KEY` | Обязательный отдельный секрет: минимум 32 случайных байта в hexadecimal; используется только для HMAC semantic fingerprint профилей и должен сохраняться между перезапусками |
| `PROFILE_FINGERPRINT_PREVIOUS_KEYS` | Необязательный список прежних fingerprint-ключей через запятую на период ротации; удаляйте старый ключ только после синхронизации всех источников |
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

`PROFILE_FINGERPRINT_KEY` не имеет development/default fallback и не должен
переиспользовать пароль администратора, session/CSRF secret или публичный ID.
Скрипты первого запуска создают 32 случайных байта и сохраняют их только в
приватном `.env`. Для ротации новый ключ становится текущим, а прежний временно
добавляется в `PROFILE_FINGERPRINT_PREVIOUS_KEYS`: importer сопоставляет старые
HMAC и при очередной синхронизации записывает новый. Прежний ключ можно удалить
после успешной синхронизации всех внешних источников; преждевременное удаление
может ослабить semantic deduplication, но не меняет legacy public references.

При обновлении существующей установки обычный backend startup намеренно
завершается с ошибкой до открытия SQLite, миграций, фоновых задач и HTTP
listener, пока администратор не задаст `PROFILE_FINGERPRINT_KEY`. Команды
`scripts/start.sh` и `scripts/start.ps1` являются явным setup/bootstrap путём:
они генерируют ключ только при его отсутствии и сохраняют его в приватном
пользовательском `.env`; настроенное непустое значение не заменяется. Поэтому
перезапуск или rebuild с тем же `.env` сохраняет semantic identity. Старые
ключи ротации должны оставаться настроенными до хотя бы одной успешной
синхронизации каждого внешнего источника новым ключом. После этого их удаление
безопасно для deduplication; удаление раньше может создать новые строки вместо
сопоставления со старыми fingerprint.

Raw URI внешних профилей содержат credentials и сейчас хранятся в plaintext
SQLite column `vless_keys.url`. Файловые permissions не являются
application-level encryption. До первого публичного stable release требуется
отдельная ограниченная задача по шифрованию raw profile storage и миграции
существующих строк без изменения delivery. Audit, preview, warnings и profile
metadata не должны копировать raw URI.

Importer использует пять разных идентификаторов, которые нельзя подменять друг
другом: SQLite `vless_keys.id` адресует конкретную source-owned строку и её
assignments; `external_key_ref` остаётся стабильным legacy/public reference
внутри источника; `url` хранит точный raw input для reparse/delivery;
`profile_fingerprint` — keyed semantic identity, уникальный только внутри
`external_source_id`; preview `ir1_` — краткоживущий HMAC raw bytes вместе с
one-based item index и не является database/public identity.

Base64 subscription input принимает standard и URL-safe alphabet, с padding
или без него; необязательный регистронезависимый префикс `base64:` и ASCII
whitespace разрешены. Encoded и decoded размеры ограничены до parsing, а
decoded body принимается только если явно распознаётся как поддерживаемый URI
body или JSON. Base64-looking malformed input возвращает стабильный
`invalid_base64_subscription`, не попадая в unknown-URI диагностику.

### Генерация подписок и границы совместимости

Выдача выбирает только назначенные пользователю активные записи; real-ключи с
тремя последовательными health failures исключаются, informational-ключи
сохраняют прежнее template-поведение. Порядок остаётся детерминированным:
uncategorized, category sort order, key sort order, source id, row id. Генерация
выполняется синхронно и не сохраняет готовые credential-bearing bodies.

Перед генерацией backend повторно разбирает authoritative `vless_keys.url` и
вычисляет semantic identity текущим `PROFILE_FINGERPRINT_KEY`. Это позволяет
дедуплицировать одинаковое подключение, даже если source-owned строки имеют
fingerprints разных поколений. Побеждает первая запись в delivery order, поэтому
её tag/name используется в результате; label не участвует в identity. Строки БД,
assignments, `external_key_ref` и source lifecycle при этом не объединяются и не
изменяются. Для legacy Xray JSON, который registry не разбирает, применяется
только keyed exact-raw identity — семантическая эквивалентность не угадывается.

Plain output выдаёт byte-exact original URI после проверки CR/LF/NUL/control
characters. Base64 output применяет standard Base64 с padding ко всему уже
отфильтрованному plain body. TUIC v4 разрешён только в этих raw-delivery форматах.
Если после eligibility и raw validation не осталось ни одной записи, backend
возвращает `503` вместо двусмысленного пустого или Base64-представления пустой
строки. Если все eligible записи исключены только structured generator-ом,
backend возвращает `422 Unprocessable Content` с кодом `all_profiles_excluded`,
форматом, общими eligible/excluded counts и агрегированным отображением безопасных
reason codes. Имена, hosts, row ids и URI в этот response не включаются.
Unsupported structured entries исключаются частично; безопасные reason codes и
их количество возвращаются в `SubShare-Exclusion-Codes` и
`SubShare-Excluded-Count`, а агрегированные количества — в
`SubShare-Exclusion-Counts`, без URI или credentials.

Структурированные generators закреплены за следующими schema targets:

- Mihomo `1.19.28` — точный target: VLESS, VMess, Trojan; supported Shadowsocks methods/plugins;
  Hysteria 2 salamander/gecko, port hopping, TLS pin; TUIC v5 common и
  Mihomo-native fields. ECH/unknown extensions, contradictory duplicates и
  TUIC v4 исключаются.
- sing-box `1.13.12` — точный target: VLESS, VMess, Trojan; Shadowsocks; Hysteria 2 salamander и
  ordered server ports; TUIC v5 common и sing-box-native fields. URI certificate
  SHA-256 pin не подменяется несовместимым SPKI pin. Gecko относится к 1.14 и не
  генерируется для выбранного stable target.
- Xray-core `26.3.27` — минимальный target; `26.7.28` — текущий target. Одинаковая
  representation прошла syntax validation обоими targets для прежних
  VLESS/VMess/Trojan, plugin-free Shadowsocks и
  representable Hysteria 2 (auth, TLS, certificate pin, port hopping,
  Salamander FinalMask). Gecko без packet-size, TUIC и Shadowsocks с SIP003
  plugin не конвертируются в другой protocol и возвращают exclusion reason.
  Hysteria `insecure=1` не преобразуется в удалённый Xray `allowInsecure` и
  исключается как `field_not_representable`. При port hopping генератор не
  записывает отсутствующий в URI interval: оба target сами применяют свой
  официальный default 30 секунд.

`official_binary` в capability matrix означает только, что полный synthetic
config принят официальной командой проверки синтаксиса. Это не означает сетевой
handshake, authentication или runtime interoperability; последнее явно
публикуется как `not_tested`.

Read-only capability matrix доступна в `GET /api/v1/subscription-delivery-settings`.
Она является backend source of truth; присланное клиентом поле `capabilities`
не входит в update DTO и отклоняется как неизвестное при update. Тот же response публикует read-only catalog
`generation_exclusion_reason_codes`; конкретная выдача возвращает только
безопасные count/codes headers. Подробный frontend profile editor остаётся
отдельной задачей.

Опциональные проверки официальными binaries не входят в обычный test suite и
ничего не скачивают. Maintainer сначала вручную скачивает exact tagged official
release assets, проверяет опубликованные upstream SHA-256 и затем задаёт пути:

```powershell
$env:MIHOMO_BIN = 'C:\validators\mihomo-v1.19.28.exe'
$env:SING_BOX_BIN = 'C:\validators\sing-box-1.13.12.exe'
$env:XRAY_26327_BIN = 'C:\validators\xray-v26.3.27.exe'
$env:XRAY_CURRENT_BIN = 'C:\validators\xray-v26.7.28.exe'
go test -count=1 ./cmd/server -run '^TestOfficial(Mihomo|SingBox|Xray)' -v
```

Harness использует соответственно `mihomo -t -d <tmp> -f <config>`,
`sing-box check --disable-color -D <tmp> -c <config>` и
`xray run -test -c <config>`, подавляет credential-bearing client output и
удаляет временные configs. При отсутствии переменной соответствующая проверка
явно пропускается.

### Пароли администраторов

Новый или изменённый пароль должен содержать от 15 до 256 Unicode code points
и занимать не более 1024 байт в UTF-8. Пробелы и Unicode разрешены. Backend и
панель не обрезают, не нормализуют и не преобразуют пароль; ограничения на
классы символов не применяются. Эта политика действует при bootstrap, создании
администратора и смене пароля, но не блокирует вход существующих учётных записей
с более коротким паролем.

Новые значения хешируются Argon2id и сохраняются в самодостаточном PHC-формате
`$argon2id$v=19$m=32768,t=3,p=2$<salt>$<digest>`. Параметры: 32 MiB памяти,
3 итерации, parallelism 2, соль 16 байт и результат 32 байта. После успешного
входа существующий bcrypt-хеш заменяется Argon2id через условное обновление.
Ошибка необязательной записи миграции фиксируется безопасным предупреждением,
но не отменяет успешный вход; неуспешная аутентификация миграцию не запускает.

Проверка по внешней базе утёкших или распространённых паролей намеренно не
добавлена. Её можно внедрить позже как отдельный слой с надёжным источником и
ясной политикой доступности, а не как короткий встроенный список слов.

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
Backup содержит ту же plaintext колонку `vless_keys.url`, включая пароли,
UUID, auth values и другие profile credentials. Храните и передавайте backup
как credential-bearing secret; ограничьте доступ, не публикуйте его и удаляйте
ненужные копии безопасным способом.

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
