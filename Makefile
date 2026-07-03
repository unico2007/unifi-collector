BINARY      := collector
PKG         := ./...
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
IMAGE       := unifi-collector:$(VERSION)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the binary
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/collector

.PHONY: run
run: ## Run against configs/config.yaml
	go run ./cmd/collector --config configs/config.yaml

.PHONY: test
test: ## Run all tests with the race detector
	go test -race -count=1 $(PKG)

.PHONY: cover
cover: ## Run tests and print coverage
	go test -cover $(PKG)

.PHONY: vet
vet: ## go vet
	go vet $(PKG)

.PHONY: lint
lint: ## Run golangci-lint (must be installed)
	golangci-lint run

.PHONY: tidy
tidy: ## Tidy modules
	go mod tidy

.PHONY: docker
docker: ## Build the docker image
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE) .

.PHONY: up
up: ## Start the full stack (collector + loki + prometheus + grafana)
	docker compose up -d --build

.PHONY: down
down: ## Stop the stack
	docker compose down

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin/
