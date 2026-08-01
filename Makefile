
.DEFAULT_GOAL := help

CMDS := $(notdir $(wildcard cmd/*))
BINS := $(addprefix bin/,$(CMDS))

COMPOSE := docker compose --env-file .env.development -f deploy/compose/docker-compose.yaml

.PHONY: up down down-clean test lint build tidy help $(BINS)

up:
	@$(COMPOSE) up -d

down:
	@$(COMPOSE) down

down-clean:
	@$(COMPOSE) down -v

test:
	@echo "Testing..."
	@go test -race -cover ./...

lint:
	@echo "Linting..."
	@golangci-lint run

build: $(BINS)

$(BINS): bin/%: cmd/%/
	go build -o $@ ./cmd/$*/

tidy:
	@echo "Tidying..."
	@go mod tidy

help:
	@echo "Usage:"
	@echo "  make up        - Start the application"
	@echo "  make down      - Stop the application"
	@echo "  make down-clean - Stop and remove volumes"
	@echo "  make test      - Run tests"
	@echo "  make lint      - Run linter"
	@echo "  make build     - Build the application"
	@echo "  make tidy      - Tidy up dependencies"
	@echo "  make help      - Show this help message"
