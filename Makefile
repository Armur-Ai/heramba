BINARY := heramba
PKG := github.com/Armur-Ai/heramba
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X $(PKG)/internal/version.Version=$(VERSION) \
           -X $(PKG)/internal/version.Commit=$(COMMIT) \
           -X $(PKG)/internal/version.Date=$(DATE)

.PHONY: build test lint vet fmt install demos clean

build: ## build the heramba binary into ./bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/heramba

test: ## run the test suite
	go test ./...

vet: ## go vet
	go vet ./...

fmt: ## format all Go sources
	gofmt -w .

lint: ## check formatting + vet (CI uses this)
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)
	go vet ./...

install: ## install heramba to GOBIN
	go install -ldflags "$(LDFLAGS)" ./cmd/heramba

demos: ## render the vhs terminal demos (requires: vhs, ttyd, ffmpeg)
	@command -v vhs >/dev/null 2>&1 || { echo "vhs not installed — see docs/demos/README.md"; exit 1; }
	@for tape in docs/demos/*.tape; do echo "rendering $$tape"; vhs $$tape; done

clean: ## remove build artifacts
	rm -rf bin dist
