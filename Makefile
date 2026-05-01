.PHONY: build test lint run docker-up docker-down

build:
	go build -o bin/server ./cmd/server

test:
	go test -race -v ./...

lint:
	golangci-lint run ./...

run:
	go run ./cmd/server

docker-up:
	docker-compose up --build -d

docker-down:
	docker-compose down
