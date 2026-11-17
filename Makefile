.PHONY: help up down reset db run build test test-integration swagger fmt

.DEFAULT_GOAL := help

BINARY := bin/pingpong
ifeq ($(OS),Windows_NT)
BINARY := bin/pingpong.exe
endif

help:
	@echo "up                приложение, Postgres и MinIO (docker compose up --build)"
	@echo "down              остановить контейнеры, данные оставить"
	@echo "reset             остановить и удалить тома базы и файлов"
	@echo "db                только Postgres и миграции, для локального запуска"
	@echo "run               go run ./cmd/app, читает .env"
	@echo "build             собрать $(BINARY)"
	@echo "test              go test -race ./..."
	@echo "test-integration  SQL-тест репозитория, нужен DATABASE_URL"
	@echo "swagger           пересобрать docs из аннотаций"
	@echo "fmt               go fmt ./..."

up:
	docker compose up --build

down:
	docker compose down

reset:
	docker compose down -v

db:
	docker compose up -d pingpong-postgres migrate

run:
	go run ./cmd/app

build:
	go build -o $(BINARY) ./cmd/app

test:
	go test -race ./...

test-integration:
	go test -tags integration ./internal/messages/ -count=1

swagger:
	swag init -g cmd/app/main.go -o docs --parseInternal

fmt:
	go fmt ./...
