.PHONY: dev test generate clean icons

dev:
	wails dev

test:
	go test ./...

generate:
	go generate ./...

icons:
	python3 ui/scripts/subset-icons.py

clean:
	rm -rf bin/
	go clean

