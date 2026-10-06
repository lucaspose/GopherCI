BIN := bin/server

.DEFAULT_GOAL := help

.PHONY: help build run test vet fmt check db-up up down logs clean

help:
	@echo "make build   Build the server into $(BIN)"
	@echo "make run     Run the server locally (needs PostgreSQL, see make db-up)"
	@echo "make test    Run the tests with the race detector"
	@echo "make vet     Run go vet"
	@echo "make fmt     Format the code"
	@echo "make check   vet + tests (what the CI runs)"
	@echo "make db-up   Start only PostgreSQL with Docker"
	@echo "make up      Start PostgreSQL + the server with Docker"
	@echo "make down    Stop the Docker stack"
	@echo "make logs    Follow the server logs"
	@echo "make clean   Remove build output"

build:
	go build -o $(BIN) ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -race -count=1

vet:
	go vet ./...

fmt:
	gofmt -w .

check: vet test

db-up:
	docker compose up -d postgres

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f server

clean:
	rm -rf bin
