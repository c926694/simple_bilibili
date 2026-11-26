.DEFAULT_GOAL := build

CONFIG ?= configs/config.yaml
COMPOSE_FILE ?= ../docker/docker-compose.yml

.PHONY: docker-up migrate-up migrate-down run-api run-worker build test lint

docker-up:
	docker compose -f "$(COMPOSE_FILE)" up -d

migrate-up:
	migrate -path migrations -database "$${MIGRATE_DATABASE_URL:?请设置 MIGRATE_DATABASE_URL}" up

migrate-down:
	migrate -path migrations -database "$${MIGRATE_DATABASE_URL:?请设置 MIGRATE_DATABASE_URL}" down 1

run-api:
	go run ./cmd/api -config "$(CONFIG)"

run-worker:
	go run ./cmd/worker -config "$(CONFIG)"

build:
	mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

test:
	go test -race ./...

lint:
	go vet ./...
	@unformatted="$$(gofmt -l cmd internal)" || exit $$?; if [ -n "$$unformatted" ]; then printf '%s\n' "$$unformatted"; exit 1; fi
