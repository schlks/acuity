.PHONY: build run dev docker up down clean

build:
	go build -ldflags="-s -w" -o bin/acuity ./cmd/acuity

run: build
	./bin/acuity

dev:
	go run ./cmd/acuity

test:
	go test ./...

docker:
	docker build -t acuity:latest .

up:
	docker compose up -d

down:
	docker compose down

clean:
	rm -rf bin/
	go clean

