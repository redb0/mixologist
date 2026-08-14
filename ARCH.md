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
│   ├── config/                  # HTTP_ADDR, GIN_MODE, LOG_LEVEL
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
├── src/app/                     # тема, ErrorBoundary
├── src/features/ingredients/    # API, listState, форма
├── src/pages/                   # server-mode DataGrid
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

- **Вход:** `GET /api/v1/auth/google/login` → Google OAuth → callback → HttpOnly session cookie + CSRF cookie.
- **Текущий пользователь:** `GET /api/v1/auth/me` (требует session cookie).
- **Выход:** `POST /api/v1/auth/logout` (session cookie + `X-CSRF-Token`).
- **CSRF:** mutating запросы с session cookie требуют заголовок `X-CSRF-Token` (HMAC от session token).
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
