.PHONY: build test lint install clean

build:
	go build -o bin/youtube-analytics-pp-cli ./cmd/youtube-analytics-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/youtube-analytics-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/youtube-analytics-pp-mcp ./cmd/youtube-analytics-pp-mcp

install-mcp:
	go install ./cmd/youtube-analytics-pp-mcp

build-all: build build-mcp
