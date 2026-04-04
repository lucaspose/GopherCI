# =========================

# CONFIG

# =========================

APP_NAME=goci

CMD_DIR=./cmd
BUILD_DIR=./bin

SERVER=$(BUILD_DIR)/server
WORKER=$(BUILD_DIR)/worker
DISPATCHER=$(BUILD_DIR)/dispatcher
CLI=$(BUILD_DIR)/goci

GO=go

# =========================

# DEFAULT

# =========================

.DEFAULT_GOAL := help

# =========================

# HELP

# =========================

help:
	@echo ""
	@echo "make build        Compile tous les binaires"
	@echo "make test         Run tests (race)"
	@echo "make lint         Lint code"
	@echo "make dev          Dev env (docker + air)"
	@echo "make proto        Génère gRPC"
	@echo "make docker-up    Start containers"
	@echo "make docker-down  Stop containers"
	@echo ""

# =========================

# BUILD

# =========================

build: build-server build-worker build-dispatcher build-cli

build-server:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(SERVER) $(CMD_DIR)/server

build-worker:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(WORKER) $(CMD_DIR)/worker

build-dispatcher:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(DISPATCHER) $(CMD_DIR)/dispatcher

build-cli:
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(CLI) $(CMD_DIR)/goci

# =========================

# TEST

# =========================

test:
	$(GO) test ./... -race -count=1

# =========================

# LINT

# =========================

lint:
	golangci-lint run

# =========================

# DEV

# =========================

dev:
	docker-compose up -d postgres redis
	air

# =========================

# PROTO

# =========================

proto:
	protoc --go_out=proto/gen --go-grpc_out=proto/gen proto/dispatcher.proto

# =========================

# DOCKER

# =========================

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# =========================

# CLEAN

# =========================

clean:
	rm -rf $(BUILD_DIR)
