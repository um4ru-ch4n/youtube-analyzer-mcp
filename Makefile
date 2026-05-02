.PHONY: help setup start stop restart logs status \
       ollama-setup ollama-start ollama-stop \
       build test lint run \
       docker-build docker-up docker-down docker-logs \
       clean

# Default
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Full Setup ──────────────────────────────────────────────

setup: ollama-setup docker-build ## First-time setup: install Ollama model + build Docker images
	@echo ""
	@echo "✅ Setup complete. Run 'make start' to launch everything."

start: ollama-start docker-up ## Start everything: Ollama + MCP server + sidecars
	@echo ""
	@echo "✅ All services running."
	@echo "   MCP endpoint: http://localhost:39280/mcp"
	@echo "   Add to ~/.claude.json:"
	@echo '   "youtube-analyzer": {"type": "http", "url": "http://localhost:39280/mcp"}'

stop: docker-down ollama-stop ## Stop everything

restart: stop start ## Restart everything

status: ## Show status of all services
	@echo "=== Docker services ==="
	@docker-compose ps 2>/dev/null || echo "Docker Compose not running"
	@echo ""
	@echo "=== Ollama ==="
	@ollama ps 2>/dev/null || echo "Ollama not running"

logs: ## Follow MCP server logs
	docker-compose logs -f mcp-server

# ── Ollama ──────────────────────────────────────────────────

ollama-setup: ## Install Ollama model (qwen3:8b, ~5GB)
	@command -v ollama >/dev/null 2>&1 || { echo "❌ Ollama not installed. Run: brew install ollama"; exit 1; }
	@echo "Pulling qwen3:8b model..."
	ollama pull qwen3:8b

ollama-start: ## Start Ollama service
	@if ollama ps >/dev/null 2>&1; then \
		echo "Ollama already running"; \
	else \
		echo "Starting Ollama..."; \
		brew services start ollama 2>/dev/null || ollama serve &; \
		sleep 3; \
	fi

ollama-stop: ## Stop Ollama service
	@brew services stop ollama 2>/dev/null || pkill ollama 2>/dev/null || true

# ── Docker ──────────────────────────────────────────────────

docker-build: ## Build Docker images
	docker-compose build

docker-up: ## Start Docker services (MCP server + whisper + clip)
	docker-compose up -d mcp-server whisper clip

docker-down: ## Stop Docker services
	docker-compose down

docker-logs: ## Follow all Docker service logs
	docker-compose logs -f

# ── Development ─────────────────────────────────────────────

build: ## Build Go binary
	go build -o bin/server ./cmd/server

test: ## Run tests
	go test -race -v ./...

lint: ## Run linter
	golangci-lint run ./...

run: ## Run locally (stdio mode, needs config.yaml in cwd)
	go run ./cmd/server

# ── Cleanup ─────────────────────────────────────────────────

clean: ## Remove build artifacts and task data
	rm -rf bin/
	@echo "Build artifacts removed."
	@echo "To also remove task data: docker volume rm youtube-analyzer-mcp_shared-data"
