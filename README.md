# База рецептов домашнего бара

[![codecov](https://codecov.io/gh/redb0/mixologist/graph/badge.svg?token=DQS2DJ597M)](https://codecov.io/gh/redb0/mixologist)

Веб-приложение ведения рецептов домашнего бара предназначено для управления
рецептами коктейлей, а также для поиска коктейлей с нужными ингредиентами.

Подробнее об архитектуре и API — в [ARCH.md](ARCH.md).

## Зависимости

| Зависимость                                                                         | Назначение                              |
|-------------------------------------------------------------------------------------|-----------------------------------------|
| Go 1.25+                                                                            | Сборка и запуск backend                 |
| Node.js 24+ и npm                                                                   | Локальная разработка frontend           |
| PostgreSQL 17+                                                                      | Основная база данных                    |
| [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) | Применение SQL-миграций                 |
| Docker                                                                              | Для тестов или локального развертывания |

## Запуск полного стека через Docker

```bash
cp .env.example .env
docker compose up --build
```

После прохождения healthcheck административная панель доступна по адресу
`http://localhost:8080`, API — через единый origin `http://localhost:8080/api`.
Миграции применяются одноразовым сервисом `migrate` до запуска backend.

Остановить стек:

```bash
docker compose down
```

Удалить также данные PostgreSQL:

```bash
docker compose down -v
```

## Локальное развертывание

### 1. PostgreSQL

Поднимите PostgreSQL и создайте базу данных, например через Docker:

```bash
docker run -d --name mixologist-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=mixologist \
  -p 5432:5432 \
  postgres:17-alpine
```

### 2. Переменные окружения

Скопируйте шаблон и при необходимости отредактируйте значения:

```bash
cp .env.example .env
```

В `.env` задаётся `DB_URL` — строка подключения для миграций (используется `make`).

Для auth (Google OAuth, cookie-сессии, CSRF) также нужны переменные из
[`.env.example`](.env.example): `GOOGLE_OAUTH_*`, `AUTH_ADMIN_EMAILS`,
`SESSION_COOKIE_*`, `SESSION_TTL`, `CSRF_*`. Реальные секреты не коммитьте;
в Docker Compose используются dev-placeholder значения, если переменные не заданы.

Подробнее о ролях, cookies и sequence flow — в [ARCH.md](ARCH.md#авторизация).

### Настройка Google OAuth

1. Создайте OAuth 2.0 Client (Web application) в [Google Cloud Console](https://console.cloud.google.com/).
2. Добавьте **Authorized redirect URI**:
   - Docker / production-like: `http://localhost:8080/api/v1/auth/google/callback`
   - Vite dev: `http://localhost:5173/api/v1/auth/google/callback`
3. Скопируйте Client ID и Client Secret в `.env`.
4. Задайте `AUTH_ADMIN_EMAILS` — список email администраторов разделенных запятыми.
5. Сгенерируйте случайные `SESSION_COOKIE_SECRET` и `CSRF_SECRET` (минимум 32 символа каждый).

После входа admin попадает в `/ingredients`, обычный пользователь — на `/`.

### 3. Миграции

Примените миграции (см. раздел [Миграции](#миграции)).

### 4. Запуск API

```bash
cd backend
go run ./cmd/api
```

Сервер поднимается по адресу `http://localhost:8080`.

Проверка (публичный endpoint):

```bash
curl http://localhost:8080/api/v1/ingredients
```

Создание и изменение ингредиентов требуют admin-сессию (HttpOnly cookie) и заголовок
`X-CSRF-Token` — см. [ARCH.md](ARCH.md#авторизация).

### 5. Запуск административного frontend

Frontend использует Vite proxy `/api` на локальный backend `http://localhost:8080` (пути `/api/v1/...` проксируются без rewrite):

```bash
cd frontend
npm install
npm run dev
```

Интерфейс доступен по адресу `http://localhost:5173`.

## Миграции

Миграции лежат в `backend/migrations/` в формате `*.up.sql` / `*.down.sql`.

### Установка golang-migrate

Инструкция по установке см. в [документации migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate).

### Команды через Makefile

Для запуска миграций можно использовать `make` файл.

Применить все недостающие миграции:

```bash
make migrate-up
```

Откатить последнюю миграцию:

```bash
make migrate-down
```

Создать шаблон новой миграции

```bash
make migrate-create NAME=add_cocktails_table
```

Явная передача строки подключения:

```bash
make migrate-up DB_URL='postgres://postgres:postgres@localhost:5432/mixologist?sslmode=disable'
```

Справка по всем целям:

```bash
make help
```

## Тесты и локальные проверки

Из каталога `backend`:

```bash
cd backend
go test ./...
```

Интеграционные тесты в `internal/repository/` и `internal/contract/` поднимают PostgreSQL через **testcontainers** — нужен запущенный **Docker**.

Локальный набор проверок (как в CI):

```bash
make ci-local
```

Frontend:

```bash
cd frontend
npm run format:check
npm run lint
npm run typecheck
npm test
npm run build
npm run api:check
```
