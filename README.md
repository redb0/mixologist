# База рецептов домашнего бара

[![codecov](https://codecov.io/gh/redb0/mixologist/graph/badge.svg?token=DQS2DJ597M)](https://codecov.io/gh/redb0/mixologist)

Веб-приложение ведения рецептов домашнего бара предназначено для управления
рецептами коктейлей, а также для поиска коктейлей с нужными ингредиентами.

Подробнее об архитектуре и API — в [ARCH.md](ARCH.md).

## Зависимости

| Зависимость                                                                         | Назначение                              |
|-------------------------------------------------------------------------------------|-----------------------------------------|
| Go 1.25+                                                                            | Сборка и запуск backend                 |
| PostgreSQL 17+                                                                      | Основная база данных                    |
| [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) | Применение SQL-миграций                 |
| Docker                                                                              | Для тестов или локального развертывания |

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

### 3. Миграции

Примените миграции (см. раздел [Миграции](#миграции)).

### 4. Запуск API

```bash
cd backend
go run ./cmd/api
```

Сервер поднимается по адресу `http://localhost:8080`.

Проверка:

```bash
curl -X POST http://localhost:8080/ingredients \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Джин",
    "description": "London dry gin",
    "unit_measurement": "мл",
    "abv": "крепкий",
    "ingredient_type": "крепкая часть"
  }'
```

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

## Тесты

Из каталога `backend`:

```bash
cd backend
go test ./...
```

Интеграционные тесты в `internal/repository/` поднимают PostgreSQL через **testcontainers** — нужен запущенный **Docker**.

## TODO

- [ ] GET для ингридиента:
  - [ ] API
  - [ ] Тесты
- [ ] PATCH для ингридиента:
  - [ ] API
  - [ ] Тесты
- [ ] DELETE для ингридиента
  - [ ] API
  - [ ] Тесты
- [ ] Эндпоинт добавления иконки:
  - [ ] API
  - [ ] Тесты
- [ ] Добавить поле `has_icon` в ингредиент для обозначения наличия иконки
- [ ] Маппинг ошибок в HTTP
- [ ] Логирование ошибок и отдача клиенту ошибок без внутренних деталей
- [ ] Тесты на парсинг ошибок
- [ ] Интеграционные тесты на HTTP обработчик
- [ ] Тесты сервиса ингредиентов
- [ ] Тесты репозитория ингредиентов
- [x] Тесты валидации перечислений

Тех. долг:

- [ ] При получении ингредиента иконка загружается целиком, хотя нужен только флаг
- [ ] Добавить таймзону UTC к полю `created_at` ингридиента
