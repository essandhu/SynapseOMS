.DEFAULT_GOAL := help

build: ## Build gateway binary and dashboard
	cd gateway && go build ./cmd/gateway
	cd dashboard && npm run build

test: ## Run gateway and dashboard test suites
	cd gateway && go test ./...
	cd dashboard && npm test

lint: ## Run go vet and tsc --noEmit
	cd gateway && go vet ./...
	cd dashboard && npx tsc --noEmit

proto: ## Generate protobuf code
	./scripts/proto-gen.sh

docker: ## Build Docker images via compose
	docker compose -f deploy/docker-compose.yml build

up: ## Start services via compose
	docker compose -f deploy/docker-compose.yml up -d

down: ## Stop services via compose
	docker compose -f deploy/docker-compose.yml down

seed: ## Seed reference data
	./scripts/seed-instruments.sh

e2e: ## Run Playwright end-to-end tests
	cd e2e && npm ci && npx playwright install --with-deps chromium && npx playwright test

clean: ## Remove build artifacts
	rm -rf gateway/gateway gateway/gateway.exe
	rm -rf dashboard/dist dashboard/build dashboard/node_modules/.cache

help: ## Print this help message
	@echo "SynapseOMS Makefile targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

.PHONY: build test lint proto docker up down seed e2e clean help
