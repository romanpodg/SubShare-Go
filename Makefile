.PHONY: build run clean frontend backend all lint

BUILD_DIR ?= .cache
SERVER_BIN ?= $(BUILD_DIR)/subshare

all: frontend backend

backend:
	mkdir -p $(BUILD_DIR)
	go build -trimpath -o $(SERVER_BIN) ./cmd/server

frontend:
	cd frontend && npm ci && npm run build

run: backend
	ADMIN_PASSWORD="$(ADMIN_PASSWORD)" $(SERVER_BIN)

clean:
	rm -f $(SERVER_BIN)
	rm -rf frontend/out frontend/.next-validation frontend/.next-e2e

tidy:
	go mod tidy
	go vet ./cmd/server/...

lint:
	golangci-lint run ./...
