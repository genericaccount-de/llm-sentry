.PHONY: build test lint run docker-up docker-down clean

build:
	go build -o bin/sentry ./cmd/sentry

test:
	go test ./...

lint:
	golangci-lint run ./...

run:
	go run ./cmd/sentry

docker-up:
	docker compose up --build

docker-down:
	docker compose down

clean:
	rm -rf bin/
