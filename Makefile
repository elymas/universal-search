.DEFAULT_GOAL := help
.PHONY: help install-py tidy fmt lint build test-go test-go-integration test-py test-node test compose-up compose-down compose-logs dev clean

help: ## Show available targets
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-22s %s\n", $$1, $$2}'

install-py: ## Install Python workspace dependencies (uv sync)
	uv sync

tidy: ## Run go mod tidy
	go mod tidy

fmt: ## Format Go, Python, and web code
	gofmt -s -w .
	ruff format services/
	pnpm -C web prettier --write .

lint: ## Lint Go, Python, and web code
	go vet ./...
	golangci-lint run
	ruff check services/
	pnpm -C web lint

build: ## Build usearch binary to cmd/usearch/usearch
	go build -o cmd/usearch/usearch ./cmd/usearch

test-go: ## Run Go unit tests with race detector and coverage
	go test -race -cover ./...

test-go-integration: ## Run Go integration tests (requires docker)
	go test -race -tags=integration ./...

test-py: ## Run Python tests via uv
	uv run pytest services/

test-node: ## Run Node.js tests via pnpm
	pnpm -C web test

test: test-go test-py test-node ## Run all test suites

compose-up: ## Start docker-compose dev stack (blocks until healthy)
	docker compose -f deploy/docker-compose.yml up -d --wait

compose-down: ## Stop and remove docker-compose dev stack
	docker compose -f deploy/docker-compose.yml down

compose-logs: ## Follow docker-compose logs
	docker compose -f deploy/docker-compose.yml logs -f

dev: compose-up ## Start dev stack then run usearch
	go run ./cmd/usearch

clean: ## Remove build artifacts
	rm -rf cmd/usearch/usearch dist/ node_modules/ .venv/
