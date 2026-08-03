.DEFAULT_GOAL := help

.PHONY: help migrate-create migrate-up migrate-down lint lint-go lint-frontend gofmt golangci-lint

NAME ?=
DOWN_STEPS ?= 1
MIGRATIONS_DIR := ./backend/migrations
BACKEND_DIR := backend
FRONTEND_DIR := frontend
GOLANGCI_LINT_VERSION := v2.4.0

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
	@echo ""
	@echo "  make lint"
	@echo "      Проверки линтеров backend и frontend (как в CI)."
	@echo ""
	@echo "  make lint-go"
	@echo "      gofmt и golangci-lint для backend."
	@echo ""
	@echo "  make lint-frontend"
	@echo "      npm run lint и npm run typecheck для frontend."

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

lint: lint-go lint-frontend

lint-go: gofmt golangci-lint

gofmt:
	@cd "$(BACKEND_DIR)" && \
		unformatted="$$(gofmt -l -s .)" && \
		if [ -n "$$unformatted" ]; then \
			echo "These files are not gofmt-formatted:"; \
			printf '%s\n' "$$unformatted"; \
			exit 1; \
		fi

golangci-lint:
	cd "$(BACKEND_DIR)" && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

lint-frontend:
	cd "$(FRONTEND_DIR)" && npm run lint
	cd "$(FRONTEND_DIR)" && npm run typecheck
