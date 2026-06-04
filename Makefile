# Universal Search — Makefile (REQ-BOOT-010)
# Compatible with macOS and Linux (POSIX sh; no bash-only constructs).
# Usage: make <target>

BINARY     := cmd/usearch/usearch
COMPOSE    := docker compose --env-file .env -f deploy/docker-compose.yml
SERVICES   := researcher storm embedder

.PHONY: help dev test test-go test-py test-node lint build clean \
        compose-up compose-down compose-logs fmt tidy install-py

# Default target: print help
help: ## Show this help message
	@printf "Universal Search — available targets:\n\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

# ── Development ────────────────────────────────────────────────────────────────

dev: compose-up ## Start compose stack and echo ready message
	@echo "Dev environment ready. Stack is running."
	@echo "Run './$(BINARY) --version' after 'make build' to verify."

# ── Testing ────────────────────────────────────────────────────────────────────

test-go: ## Run Go tests with race detector and coverage
	go test ./... -race -cover

test-py: ## Run pytest for all Python services
	@for svc in $(SERVICES); do \
		echo "==> pytest services/$$svc"; \
		uv run --directory services/$$svc pytest; \
	done

test-node: ## Run Next.js typecheck (no test runner configured yet; SPEC-UI-001 adds tests)
	pnpm --dir web typecheck

test: test-go test-py test-node ## Run all tests (Go + Python + Node)

# ── Linting ────────────────────────────────────────────────────────────────────

lint: ## Run all linters (golangci-lint + ruff + eslint + hadolint)
	go vet ./...
	golangci-lint run ./...
	@for svc in $(SERVICES); do \
		echo "==> ruff services/$$svc"; \
		uv run --directory services/$$svc ruff check .; \
	done
	pnpm --dir web lint
	@echo "==> hadolint Dockerfiles"
	@find services -name Dockerfile | xargs hadolint

# ── Build ──────────────────────────────────────────────────────────────────────

build: ## Build usearch binary to $(BINARY)
	go build -o $(BINARY) ./cmd/usearch

# ── Cleanup ────────────────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	rm -rf cmd/usearch/usearch dist/ .next/ coverage/

# ── Docker Compose ─────────────────────────────────────────────────────────────

compose-up: ## Start all compose services in detached mode (--wait for healthchecks)
	$(COMPOSE) up -d --wait

compose-down: ## Stop and remove compose containers
	$(COMPOSE) down

compose-logs: ## Tail compose service logs
	$(COMPOSE) logs -f

# ── Formatting ─────────────────────────────────────────────────────────────────

fmt: ## Format Go, Python, and web files
	gofmt -s -w .
	@for svc in $(SERVICES); do \
		echo "==> ruff format services/$$svc"; \
		uv run --directory services/$$svc ruff format .; \
	done
	pnpm --dir web format

# ── Go module ──────────────────────────────────────────────────────────────────

tidy: ## Run go mod tidy
	go mod tidy

# ── Python workspace ───────────────────────────────────────────────────────────

install-py: ## Install Python workspace dependencies with uv
	uv sync
