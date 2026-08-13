.PHONY: build run dev docker up down clean

build:
	docker build -t codeberg.org/shlks/acuity:latest .

push: build
	docker push codeberg.org/shlks/acuity:latest

dev:
	air

test:
	go test ./...

generate:
	go generate ./...

up:
	docker compose -f docker-dev.yml up -d

down:
	docker compose -f docker-dev.yml down

clean:
	rm -rf bin/
	go clean

