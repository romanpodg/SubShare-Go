# SubShare (MVP)

Лёгкий сервис подписок для VLESS-ключей с веб‑админкой: управление пользователями, VLESS-ключами и персональными подписками.
## Возможности

- Управление пользователями и VLESS-ключами
- Привязка ключей к пользователю (drag & drop)
- Одноразовая активация подписки по `activation_code`
- Статусы подписки: `active` / `paused` / `blocked`
- Ограничение количества устройств по HWID
- Проверка доступности VLESS-ключей
- Авторизация администратора + CSRF-защита POST-форм

## Стек

- Go (stdlib HTTP)
- SQLite (`modernc.org/sqlite`)
- HTML templates + CSS

## Быстрый старт

### 1. Локально: `go run` + `npm run dev`

Backend:

```bash
go mod tidy

export ADMIN_USER=admin
export ADMIN_PASSWORD=admin
export DEVICE_LIMIT_MESSAGE="You have reached the maximum number of allowed devices for your subscription"

go run ./cmd/server
```

По умолчанию backend запускается на `http://localhost:8080`.

Frontend:

```bash
cd frontend
npm install
npm run dev
```

Frontend dev-сервер будет доступен на `http://localhost:3000`.

Если фронтенд в dev-режиме должен ходить в backend на другом адресе, убедитесь, что запросы к `/api/*` и `/sub/*` проксируются или backend доступен с того же origin.

### 2. Через Docker

1. Создайте `.env` из примера:

```bash
cp .env.example .env
```

2. Отредактируйте `.env` и задайте как минимум:

```env
ADMIN_PASSWORD=change-me
DOMAIN=localhost
SSL_EMAIL=admin@example.com
```

3. Запустите контейнеры:

```bash
docker compose up -d --build
```

4. Проверьте логи при необходимости:

```bash
docker compose logs -f
```

5. Остановите проект:

```bash
docker compose down
```

При запуске через Docker:

- frontend публикуется на `http://localhost` и `https://localhost`
- backend внутри compose-сети доступен контейнеру frontend и отдельно наружу не публикуется по умолчанию

## Переменные окружения

- `ADMIN_USER` — логин администратора (по умолчанию `admin`)
- `ADMIN_PASSWORD` — пароль администратора (по умолчанию `admin123`, только для локального запуска)
- `DEVICE_LIMIT_MESSAGE` — текст сообщения при превышении лимита устройств
- `HAPP_CRYPTO_API_URL` — API для шифрования subscription-ссылки (по умолчанию `https://crypto.happ.su/api-v2.php`)
- `SUBSCRIPTION_BODY_ENCODING` — формат выдачи тела подписки: `base64` (по умолчанию) или `plain`

## Основные маршруты

- `GET /subscription` — страница активации для клиента
- `GET /sub/{subscription_id}` — выдача подписки (`text/plain`)
- `GET /admin/login` / `POST /admin/login` — вход в админку
- `GET /admin` — админ-панель
- `POST /admin/users` — создание пользователя
- `POST /admin/users/{id}/subscription` — управление статусом/сроками
- `POST /admin/users/{id}/keys` — обновление ключей пользователя
- `POST /admin/users/{id}/hwid` — лимит устройств
- `POST /admin/keys` — создание VLESS-ключа
- `POST /admin/keys/{id}/edit|update|check|delete` — операции с ключом

## Поведение лимита устройств

- HWID читается из `?hwid=...`, `X-HWID` или `X-Device-ID`
- Доп. метаданные устройства (для читаемого отображения в HWID-панели) можно передавать через query/header:
	- `device_name` / `X-Device-Name`
	- `device_model` / `X-Device-Model`
	- `platform` / `X-Device-Platform`
	- `os_version` / `X-OS-Version`
	- `app_name` / `X-App-Name`
	- `app_version` / `X-App-Version`
- При превышении лимита endpoint `/sub/{subscription_id}` возвращает `200 text/plain` без ключей
- Текст берется из `DEVICE_LIMIT_MESSAGE`
- Дополнительно отправляется заголовок `X-Device-Limit-Message`

## Структура проекта

```text
cmd/server/        # HTTP-сервер и бизнес-логика
web/templates/     # HTML-шаблоны
web/static/        # CSS/статические файлы
data/              # SQLite база (локально)
docs/              # документация
```

## Примечания

- Для production обязательно задайте сильный `ADMIN_PASSWORD`
- Рекомендуется запускать за reverse proxy с HTTPS
- Перед продом настройте бэкапы `data/app.db`
