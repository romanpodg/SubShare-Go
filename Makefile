.PHONY: build run clean frontend backend all

all: frontend backend

backend:
	go build -o server ./cmd/server

frontend:
	cd frontend && npm ci && npm run build

run: backend
	ADMIN_PASSWORD=admin ./server

clean:
	rm -f server
	rm -rf frontend/out frontend/.next

tidy:
	go mod tidy
	go vet ./cmd/server/...
