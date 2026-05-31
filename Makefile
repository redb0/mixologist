.DEFAULT_GOAL := help

.PHONY: help migrate-create migrate-up migrate-down

NAME ?=
DOWN_STEPS ?= 1
MIGRATIONS_DIR := ./backend/migrations

# DB_URL: приоритет — аргумент make (DB_URL=...), иначе переменная окружения;
# если не задано, подхватывается .env в корне репозитория (при наличии).
ifeq ($(origin DB_URL),undefined)
-include .env
endif

help:
	@echo "Команды:"
	@echo "  make migrate-create NAME=<имя_миграции>"
	@echo "      Создать шаблон миграции (up/down .sql) через golang-migrate:"
	@echo "      migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq <имя_миграции>"
	@echo "      Пример: make migrate-create NAME=create_ingredient_table"
	@echo ""
	@echo "  make migrate-up [DB_URL=<postgres_url>]"
	@echo "      Применить все недостающие миграции (migrate ... up)."
	@echo "      DB_URL: аргумент make, иначе переменная окружения, иначе .env в корне."
	@echo "      Пример: make migrate-up DB_URL='postgres://…'"
	@echo ""
	@echo "  make migrate-down [DB_URL=<postgres_url>] [DOWN_STEPS=<число>]"
	@echo "      Откатить миграции (по умолчанию последнюю одну)."
	@echo "      Источник DB_URL такой же, как у migrate-up."
	@echo "      Пример: make migrate-down DB_URL='postgres://…' DOWN_STEPS=1"

migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Укажите имя миграции: make migrate-create NAME=<имя_миграции>"; \
		echo "Справка: make help"; \
		exit 1; \
	fi
	migrate create -ext sql -dir "$(MIGRATIONS_DIR)" -seq "$(NAME)"

migrate-up:
	@if [ -z "$(DB_URL)" ]; then \
		echo "Укажите DB_URL (строка подключения к PostgreSQL для migrate)."; \
		echo "Справка: make help"; \
		exit 1; \
	fi
	migrate -path "$(MIGRATIONS_DIR)" -database "$(DB_URL)" up

migrate-down:
	@if [ -z "$(DB_URL)" ]; then \
		echo "Укажите DB_URL (строка подключения к PostgreSQL для migrate)."; \
		echo "Справка: make help"; \
		exit 1; \
	fi
	migrate -path "$(MIGRATIONS_DIR)" -database "$(DB_URL)" down "$(DOWN_STEPS)"
