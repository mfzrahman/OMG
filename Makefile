.PHONY: run build test lint clean migrate-up migrate-down

# Load .env file if it exists (optional).
-include .env
export

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/server

test:
	go test -race -count=1 -v ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

migrate-up:
	migrate -path migrations -database "sqlite3://$(DATABASE_PATH)" up

migrate-down:
	migrate -path migrations -database "sqlite3://$(DATABASE_PATH)" down 1

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v
