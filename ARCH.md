# Домашняя база коктейлей

Сервис ведения рецептов коктейлей и ингредиентов домашнего бара, а также поиска
того, что можно приготовить из имеющихся дома продуктов. Проект изучает Go,
HTTP-серверы, работу с PostgreSQL, сложные SQL-запросы и конкурентность.

## Технологический стек

- Go 1.25+ в качестве языка программирования
- Gin в качестве HTTP фреймворка
- PostgreSQL в качестве основной базы данных
- `migrate` для миграций
- sqlx для доступа к БД
- React, TypeScript, Vite и MUI для административного frontend
- TanStack Query для серверного состояния frontend
- Nginx как раздатчик SPA и reverse proxy `/api`
- Docker Compose для PostgreSQL, миграций, backend и frontend
- Redis — запланирован как кеш (пока не используется)

## Архитектура backend

Приложение построено по слоям с направлением зависимостей сверху вниз:

```mermaid
flowchart TB
    Client[HTTP-клиент]
    Handler[handlers — HTTP, DTO]
    Service[services — бизнес-логика]
    Repository[repository — sqlx, Postgres]
    Domain[domain — сущности и ошибки]

    Client --> Handler
    Handler --> Service
    Service --> Repository
    Handler -.-> Domain
    Service -.-> Domain
    Repository -.-> Domain
    Repository --> DB[(PostgreSQL)]
```

### Структура каталогов

```text
backend/
├── cmd/api/main.go              # config, DB, router
├── internal/
│   ├── config/                  # DB, HTTP, auth/OAuth/cookie/CSRF настройки
│   ├── middleware/              # request ID, access log, recovery, auth, CSRF
│   ├── httperr/                 # единый HTTP error envelope
│   ├── router/                  # /health и group /api/v1
│   ├── contract/                # OpenAPI contract-тесты
│   ├── domain/                  # сущности и domain-ошибки
│   ├── handlers/                # HTTP-контроллеры, DTO, structured errors
│   ├── models/                  # модели базы данных
│   ├── repository/              # реализация доступа к БД
│   ├── services/                # бизнес-логика
│   └── testutil/                # общие хелперы для интеграционных тестов
└── migrations/                  # SQL-миграции
frontend/
├── src/shared/api/generated.ts  # OpenAPI DTO
├── src/app/                     # тема, ErrorBoundary, AdminLayout, UserLayout
├── src/features/auth/           # AuthProvider, route guards, UserMenu
├── src/features/ingredients/    # API, listState, форма
├── src/pages/                   # login, auth callback/error, server-mode DataGrid
└── nginx.conf                   # SPA fallback и reverse proxy /api
```

## Сущности

### Ингредиент

| Атрибут            | Описание              | Обязательность | Ограничения                                                                                                               |
|--------------------|-----------------------|----------------|---------------------------------------------------------------------------------------------------------------------------|
| `id`               | Идентификатор         | Да             | Идентификатор                                                                                                             |
| `name`             | Имя                   | Да             | Уникальное (case-insensitive через `LOWER(name)`), 3–512 символов                                                         |
| `description`      | Описание              | Нет            | до 1024 символов                                                                                                          |
| `unit_measurement` | Единица измерения     | Да             | `мл`, `гр`, `шт`, `дэш`                                                                                                   |
| `abv`              | Крепость              | Да             | `безалкогольный`, `слабоалкогольный`, `крепкий`                                                                           |
| `ingredient_type`  | Тип                   | Да             | `крепкая часть`, `безалкогольная часть`, `вермут`, `вино`, `ликер`, `биттер`, `сироп`, `другое`, `фрукт`, `овощ`, `ягода` |
| `icon`             | Иконка                | Нет            | бинарные данные (BYTEA), загружается отдельно                                                                             |
| `created_at`       | Дата и время создания | Да             | TIMESTAMPTZ, UTC в API                                                                                                    |
| `updated_at`       | Дата обновления       | Да             | TIMESTAMPTZ                                                                                                               |
| `version`          | Версия записи         | Да             | Optimistic locking, default `1`                                                                                           |

В JSON API вместо `icon` отдаётся вычисляемое поле `has_icon` (`true`, если `len(icon) > 0`).

Значения enum хранятся в PostgreSQL как отдельные типы и дублируются в Go как
типизированные строки.

Работа с иконками будет выполняться отдельно от CRUD операций с ингридиентом.

### Пользователь и сессия

| Атрибут          | Описание                         | Хранение                          |
|------------------|----------------------------------|-----------------------------------|
| `google_subject` | Идентификатор Google             | `users.google_subject` (unique)   |
| `email`          | Email (unique case-insensitive)  | `users.email`                     |
| `role`           | `user` или `admin`               | `users.role`; admin — allowlist   |
| session token    | Opaque token                     | HttpOnly cookie; в БД только hash |

Роль `admin` назначается по email allowlist (`AUTH_ADMIN_EMAILS`) при входе через Google OAuth.

## Авторизация

Backend выполняет Google OAuth и выдаёт opaque HttpOnly cookie-сессию. Frontend не хранит Google tokens и не кладёт session token в `localStorage`/`sessionStorage`.

### Роли и модель доступа

| Роль    | Назначение                                                 | Примеры доступа                                                                    |
|---------|------------------------------------------------------------|------------------------------------------------------------------------------------|
| `user`  | Обычный аккаунт после входа через Google                   | `/`, `/logout`, `GET /auth/me`; admin routes недоступны                            |
| `admin` | Администратор каталога; email входит в `AUTH_ADMIN_EMAILS` | Всё, что доступно `user`, плюс CRUD ингредиентов и mutating `/api/v1/ingredients*` |

Правила:

- Роль назначается при каждом входе: email из allowlist → `admin`, иначе → `user`.
- Ownership берётся только из auth context (session cookie), а не из параметров запроса.
- Чтение каталога ингредиентов публичное; mutating endpoints требуют `admin` + CSRF.
- `401 Unauthorized` — нет или просрочена сессия; `403 Forbidden` — сессия есть, но роли недостаточно или CSRF невалиден.

### Конфигурация auth

Полный шаблон — в [`.env.example`](.env.example). Для Docker Compose обязательны `SESSION_COOKIE_SECRET` и `CSRF_SECRET` (минимум 32 символа).

| Переменная                  | Обязательность | Назначение                                                                         |
|-----------------------------|----------------|------------------------------------------------------------------------------------|
| `GOOGLE_OAUTH_CLIENT_ID`    | да             | Client ID из Google Cloud Console                                                  |
| `GOOGLE_OAUTH_CLIENT_SECRET`| да             | Client secret OAuth-приложения                                                     |
| `GOOGLE_OAUTH_CALLBACK_URL` | да             | Redirect URI backend, например `http://localhost:8080/api/v1/auth/google/callback` |
| `AUTH_ADMIN_EMAILS`         | нет            | Comma-separated allowlist admin email; пустое значение → только роль `user`        |
| `SESSION_COOKIE_SECRET`     | да             | HMAC-секрет для oauth state cookie и hashing session token (≥ 32 символов)         |
| `SESSION_COOKIE_NAME`       | нет            | Имя session cookie, default `session`                                              |
| `SESSION_COOKIE_DOMAIN`     | нет            | Domain cookie; пусто — текущий host                                                |
| `SESSION_COOKIE_SECURE`     | нет            | `true`/`false`; в `GIN_MODE=release` принудительно `true`                          |
| `SESSION_TTL`               | нет            | TTL сессии, default `168h`                                                         |
| `CSRF_SECRET`               | да             | HMAC-секрет CSRF token (≥ 32 символов, отдельно от session secret)                 |

Имена CSRF cookie (`csrf_token`) и заголовка (`X-CSRF-Token`) — фиксированный контракт с frontend, не настраиваются через env.

Для Vite dev (`:5173`) callback URL должен быть same-origin со SPA, например `http://localhost:5173/api/v1/auth/google/callback`, чтобы OAuth redirect и cookie оставались на одном origin.

### OAuth и session flow

```mermaid
sequenceDiagram
    participant Browser
    participant Frontend
    participant Backend
    participant Google
    participant DB

    Browser->>Frontend: Открыть /login
    Frontend->>Browser: Ссылка на GET /api/v1/auth/google/login?return_to=/auth/callback
    Browser->>Backend: GET /auth/google/login
    Backend->>Backend: state, nonce, oauth_state cookie
    Backend->>Browser: 302 на Google OAuth
    Browser->>Google: Авторизация
    Google->>Browser: 302 на /auth/google/callback?code&state
    Browser->>Backend: GET /auth/google/callback
    Backend->>Backend: Проверка oauth_state, state, nonce
    Backend->>Google: Обмен code на id_token
    Backend->>Backend: Валидация id_token claims
    Backend->>DB: Upsert user, sync role из allowlist
    Backend->>DB: Create session (hash token)
    Backend->>Browser: Set-Cookie session + csrf_token, 302 return_to
    Browser->>Frontend: /auth/callback
    Frontend->>Backend: GET /auth/me (cookie)
    Backend->>DB: Lookup session by hash
    Backend->>Frontend: CurrentUser JSON
    Frontend->>Browser: Route guard → user или admin layout

    Note over Browser,Backend: Mutating API
    Browser->>Backend: POST/PATCH/DELETE + cookie + X-CSRF-Token
    Backend->>Backend: RequireAuth, RequireRole, RequireCSRF
    Backend->>Browser: 2xx / 401 / 403
```

### Endpoints и cookies

- **Вход:** `GET /api/v1/auth/google/login?return_to=` → перенаправление на Google; `return_to` — относительный путь внутри SPA.
- **Callback:** `GET /api/v1/auth/google/callback` → session cookie + CSRF cookie; ошибки → перенаправление `/auth/error?code=&message=`.
- **Текущий пользователь:** `GET /api/v1/auth/me` (session cookie).
- **Выход:** `POST /api/v1/auth/logout` (session cookie + `X-CSRF-Token`).
- **CSRF:** mutating запросы с session cookie требуют заголовок `X-CSRF-Token` (HMAC от session token и часового bucket). CSRF cookie обновляется на аутентифицированных запросах; `Max-Age` совпадает с `SESSION_TTL`.
- **RBAC:** чтение `/api/v1/ingredients*` публичное; `POST`/`PATCH`/`DELETE`/`PUT .../icon` — только `admin`.

## HTTP API

Базовый URL: `http://localhost:8080`

### Сводка endpoints

Продуктовый API: `/api/v1`. Operational health: `GET /health`.

| Метод    | Путь                              | Описание                    | Доступ        |
|----------|-----------------------------------|-----------------------------|---------------|
| `GET`    | `/health`                         | Состояние backend и БД      | публичный     |
| `GET`    | `/api/v1/auth/google/login`       | Начать Google OAuth         | публичный     |
| `GET`    | `/api/v1/auth/google/callback`    | Callback Google OAuth       | публичный     |
| `GET`    | `/api/v1/auth/me`                 | Текущий пользователь        | auth          |
| `POST`   | `/api/v1/auth/logout`             | Выход                       | auth + CSRF   |
| `GET`    | `/api/v1/ingredients`             | Список ингредиентов         | публичный     |
| `GET`    | `/api/v1/ingredients/:id`         | Ингредиент по ID            | публичный     |
| `GET`    | `/api/v1/ingredients/:id/icon`    | Получить иконку             | публичный     |
| `POST`   | `/api/v1/ingredients`             | Создать ингредиент          | admin + CSRF  |
| `PATCH`  | `/api/v1/ingredients/:id`         | Частичное обновление        | admin + CSRF  |
| `DELETE` | `/api/v1/ingredients/:id`         | Удалить ингредиент          | admin + CSRF  |
| `PUT`    | `/api/v1/ingredients/:id/icon`    | Загрузить иконку (PNG/JPEG) | admin + CSRF  |

Ошибки: envelope `{"error":{"code","message","request_id","details?"}}` + заголовок `X-Request-ID`.
Контракт описан в `api/openapi.yaml`; frontend DTO генерируются через `openapi-typescript`.

Общий response-объект ингредиента:

```json
{
  "id": 1,
  "name": "Джин",
  "description": "London dry gin",
  "unit_measurement": "мл",
  "abv": "крепкий",
  "ingredient_type": "крепкая часть",
  "has_icon": false,
  "version": 1,
  "created_at": "2026-05-31T12:00:00Z",
  "updated_at": "2026-05-31T12:00:00Z"
}
```

### Проверка состояния `GET /health`

Проверяет доступность PostgreSQL через `PingContext`.

- `200 OK`: `{"status":"ok"}`
- `503 Service Unavailable`: `{"status":"unavailable"}`

### Список ингредиентов `GET /api/v1/ingredients`

Query: `pageSize` (default 25), `pageToken`, `sort` (`created_at`|`name`), `order`, фильтры `name`, `abv`, `ingredient_type`.

**Response** `200 OK`:

```json
{
  "ingredients": [/* Ingredient */],
  "nextPageToken": "",
  "totalSize": 42
}
```

Сортировка по умолчанию: `created_at DESC`. Пагинация на основе курсора.

### Получить ингредиент `GET /ingredients/:id`

**Response** `200 OK` — `IngredientResponse`.

Поле `has_icon` вычисляется на уровне API: `true`, если у ингредиента загружена иконка
(бинарные данные не возвращаются в JSON).

**Коды ответов:**

| Код   | Условие                                |
|-------|----------------------------------------|
| `200` | Ингредиент найден                      |
| `400` | Невалидный `id` в path                 |
| `404` | Ингредиент не найден                   |
| `500` | Внутренняя ошибка (БД, инфраструктура) |

### Создать ингредиент `POST /ingredients`

**Request** (`application/json`):

```json
{
  "name": "Джин",
  "description": "London dry gin",
  "unit_measurement": "мл",
  "abv": "крепкий",
  "ingredient_type": "крепкая часть"
}
```

`description` опционален. `has_icon` в ответе всегда `false` (иконка загружается отдельно).

**Response** `201 Created` — `IngredientResponse`.

**Коды ответов:**

| Код   | Условие                                  |
|-------|------------------------------------------|
| `201` | Ингредиент создан                        |
| `400` | Ошибка валидации JSON или неверные enum  |
| `409` | Ингредиент с таким `name` уже существует |
| `500` | Внутренняя ошибка (БД, инфраструктура)   |

### Обновить ингредиент `PATCH /ingredients/:id`

Частичное обновление: передаются только изменяемые поля; omitted-поля не меняются.
Пустой body `{}` — ошибка валидации.

**Request** (`application/json`):

```json
{
  "name": "Джин London Dry",
  "description": "Обновлённое описание"
}
```

**Response** `200 OK` — `IngredientResponse`.

**Коды ответов:**

| Код   | Условие                                  |
|-------|------------------------------------------|
| `200` | Ингредиент обновлён                      |
| `400` | Невалидный `id`, JSON, пустой body, enum |
| `404` | Ингредиент не найден                     |
| `409` | Ингредиент с таким `name` уже существует |
| `500` | Внутренняя ошибка (БД, инфраструктура)   |

### Удалить ингредиент `DELETE /ingredients/:id`

**Response** `204 No Content` — тело ответа отсутствует.

**Коды ответов:**

| Код   | Условие                                |
|-------|----------------------------------------|
| `204` | Ингредиент удалён                      |
| `400` | Невалидный `id` в path                 |
| `404` | Ингредиент не найден                   |
| `500` | Внутренняя ошибка (БД, инфраструктура) |

### Получить иконку `GET /ingredients/:id/icon`

**Response** `200 OK` — бинарное тело с `Content-Type: image/png` или
`image/jpeg` и заголовком `Content-Length`.

**Коды ответов:**

| Код   | Условие                              |
|-------|--------------------------------------|
| `200` | Иконка получена                      |
| `400` | Невалидный `id` в path               |
| `404` | Ингредиент или его иконка не найдены |
| `500` | Внутренняя ошибка                    |

### Загрузить иконку `PUT /ingredients/:id/icon`

Иконка загружается отдельно от CRUD. Бинарные данные не возвращаются в JSON-ответах
ингредиента — только флаг `has_icon`.

**Request:** `application/octet-stream` — сырое тело файла (PNG или JPEG, до 512 KB).

**Response** `204 No Content` — тело ответа отсутствует.

**Коды ответов:**

| Код   | Условие                                                       |
|-------|---------------------------------------------------------------|
| `204` | Иконка сохранена                                              |
| `400` | Невалидный `id`, пустое тело, размер > 512 KB или не PNG/JPEG |
| `404` | Ингредиент не найден                                          |
| `500` | Внутренняя ошибка (БД, инфраструктура)                        |

## Запуск проекта (локально)

1. Поднять PostgreSQL и создать базу.
2. Применить миграции из `backend/migrations/` (см. [README.md](README.md)).
3. Задать переменную окружения:

    ```bash
    export DB_URL="postgres://postgres:postgres@localhost:5432/mixologist?sslmode=disable"
    ```

4. Запустить API:

    ```bash
    cd backend
    go run ./cmd/api
    ```

Сервер слушает `:8080`.

## Тестирование

```bash
cd backend
go test ./...
```

Интеграционные тесты поднимают контейнер Postgres!
