.PHONY: build run test clean

build:
	go build -o wikigraph ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

clean:
	rm -f wikigraph

