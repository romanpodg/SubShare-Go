# Xary Sub (MVP)

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

```bash
go mod tidy

export ADMIN_USER=admin
export ADMIN_PASSWORD=change_me_now
export DEVICE_LIMIT_MESSAGE="You have reached the maximum number of allowed devices for your subscription"

go run ./cmd/server
```

Приложение запускается на `http://localhost:8080`.

## Переменные окружения

- `ADMIN_USER` — логин администратора (по умолчанию `admin`)
- `ADMIN_PASSWORD` — пароль администратора (по умолчанию `admin123`, только для локального запуска)
- `DEVICE_LIMIT_MESSAGE` — текст сообщения при превышении лимита устройств

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

## Публикация в GitHub

`.gitignore` уже добавлен и исключает локальные артефакты (`server`, `data/*.db`, `.env`, IDE-файлы).

```bash
git init
git add .
git commit -m "Initial commit: xray-sub MVP"
git branch -M main
git remote add origin <YOUR_GITHUB_REPO_URL>
git push -u origin main
```

## Примечания

- Для production обязательно задайте сильный `ADMIN_PASSWORD`
- Рекомендуется запускать за reverse proxy с HTTPS
- Перед продом настройте бэкапы `data/app.db`
