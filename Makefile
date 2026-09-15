.PHONY: dev test generate clean

dev:
	air

test:
	go test ./...

generate:
	go generate ./...

clean:
	rm -rf bin/
	go clean

