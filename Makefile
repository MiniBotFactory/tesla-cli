SHELL := /bin/bash

BIN          := tesla
PKG          := github.com/wmango/tesla-cli
META_PKG     := $(PKG)/internal/meta
VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
COMMIT       ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE   ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(META_PKG).Version=$(VERSION) \
	-X $(META_PKG).Commit=$(COMMIT) \
	-X $(META_PKG).BuildDate=$(BUILD_DATE)

.PHONY: build run test lint tidy clean install completions manpages

build:
	go build -ldflags '$(LDFLAGS)' -o bin/$(BIN) ./cmd/tesla

run: build
	./bin/$(BIN) $(ARGS)

test:
	go test ./... -count=1

lint:
	go vet ./...
	gofmt -l . | tee /tmp/gofmt.out; test ! -s /tmp/gofmt.out

tidy:
	go mod tidy

clean:
	rm -rf bin manpages dist

install: build
	install -d $$HOME/.local/bin
	install -m 0755 bin/$(BIN) $$HOME/.local/bin/$(BIN)
	@echo "installed to $$HOME/.local/bin/$(BIN)"

completions: build
	mkdir -p completions
	./bin/$(BIN) completion bash       > completions/$(BIN).bash
	./bin/$(BIN) completion zsh        > completions/_$(BIN)
	./bin/$(BIN) completion fish       > completions/$(BIN).fish
	./bin/$(BIN) completion powershell > completions/$(BIN).ps1

manpages: build
	./bin/$(BIN) man --out ./manpages
