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

.PHONY: build run test lint tidy clean install completions manpages publish release-tag

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

# ---------- 发布 ----------
# 二进制:由 .github/workflows/release.yml 在 git tag 推送时自动跑 GoReleaser。
# npm:    手动跑(2FA 限制),用下面的 publish target。
#
# 注意:这两个 target 用 RELEASE_VERSION,刻意区别于顶部 ldflags 的 VERSION
# (后者是 git describe 的自动值,可能含 -dirty 后缀,不能拿来 npm publish)。
#
# 用法:make publish      RELEASE_VERSION=0.1.0
publish:
	@if [ -z "$(RELEASE_VERSION)" ]; then \
		echo "Usage: make publish RELEASE_VERSION=0.1.0"; exit 2; \
	fi
	npm version $(RELEASE_VERSION) --no-git-tag-version --allow-same-version
	npm publish --access public

# 用法:make release-tag  RELEASE_VERSION=0.2.0
# 创建 annotated tag → push 触发 GH Actions(GoReleaser)。
release-tag:
	@if [ -z "$(RELEASE_VERSION)" ]; then \
		echo "Usage: make release-tag RELEASE_VERSION=0.2.0"; exit 2; \
	fi
	git tag -a v$(RELEASE_VERSION) -m "v$(RELEASE_VERSION)"
	git push origin v$(RELEASE_VERSION)
	@echo
	@echo "GitHub Actions 已开始构建 → https://github.com/MiniBotFactory/tesla-cli/actions"
	@echo "构建结束后,跑:make publish RELEASE_VERSION=$(RELEASE_VERSION)"
