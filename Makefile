BINARY := loopctl
MODULE := github.com/loop-eng/loopctl
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X '$(MODULE)/internal/cli.version=$(VERSION)' \
	-X '$(MODULE)/internal/cli.commit=$(COMMIT)' \
	-X '$(MODULE)/internal/cli.date=$(DATE)'

.PHONY: build test lint run clean install fuzz

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/loopctl

run: build
	./bin/$(BINARY)

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

fuzz:
	go test -fuzz=Fuzz -fuzztime=30s ./internal/parser/
	go test -fuzz=Fuzz -fuzztime=30s ./internal/source/

clean:
	rm -rf bin/ dist/

install: build
	cp bin/$(BINARY) $(GOPATH)/bin/$(BINARY) 2>/dev/null || cp bin/$(BINARY) ~/go/bin/$(BINARY)

.DEFAULT_GOAL := build
