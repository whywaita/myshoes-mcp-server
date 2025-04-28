.PHONY: help
.DEFAULT_GOAL := help

BUILD_LDFLAGS ?= \
	-X main.version=$(shell git describe --tags --match='v*' --abbrev=0) \
	-X main.commit=$(shell git rev-parse --short HEAD) \
	-X main.date=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

help:
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

bin:
	mkdir -p $@

.PHONY: bin/myshoes-mcp-server
bin/myshoes-mcp-server: bin ## Build myshoes-mcp-server
	go build -o $@ -ldflags "$(BUILD_LDFLAGS)" cmd/myshoes-mcp-server/main.go

.PHONY: go-build
go-build: bin/myshoes-mcp-server ## Build all
	@echo "Build complete"

.PHONY: go-mod
go-mod: ## Update go.mod and go.sum
	go mod tidy
	git diff --exit-code go.sum

.PHONY: go-test
go-test: ## Exec test
	go test -ldflags="$(BUILD_LDFLAGS)" -cover ./...

.PHONY: docker-build
docker-build: bin/myshoes-mcp-server ## Build docker image
	docker build -t myshoes-mcp-server:latest .