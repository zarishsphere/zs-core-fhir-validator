.PHONY: all build test lint security clean dev docker-build help
GO := go
BINARY := zs-fhir-validator
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REGISTRY := ghcr.io/zarishsphere
IMAGE := $(REGISTRY)/zs-core-fhir-validator
all: lint test build
build: ## Build binary
	CGO_ENABLED=0 $(GO) build -ldflags="-w -s" -o $(BINARY) ./cmd/validator
build-arm64: ## Build for ARM64 (Raspberry Pi 5)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o $(BINARY)-arm64 ./cmd/validator
test: ## Run tests
	$(GO) test ./... -race -coverprofile=coverage.out -timeout=60s
lint: ## Lint
	golangci-lint run ./... --timeout=5m
security: ## Trivy scan
	trivy fs --severity CRITICAL,HIGH .
docker-build: ## Multi-arch Docker build
	docker buildx build --platform linux/amd64,linux/arm64 -f deploy/Dockerfile -t $(IMAGE):$(VERSION) --push .
dev: ## Run locally
	ZS_VAL_ENV=development $(GO) run ./cmd/validator
clean: ## Clean artifacts
	rm -f $(BINARY) $(BINARY)-arm64 coverage.out
help: ## Help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
