.PHONY: build run dev docker up down clean

build:
	mkdir -p ./bin
	go build -ldflags="-s -w" -o ./bin/acuity ./cmd/acuity

run: build
	./bin/acuity

dev:
	go run ./cmd/acuity

test:
	go test ./...

generate:
	go generate ./...

docker:
	docker build -t acuity:latest .

weaviate:
	docker compose up weaviate multi2vec-clip -d

debug: build
	docker compose up

up:
	docker compose up

down:
	docker compose down

clean:
	rm -rf bin/
	go clean

