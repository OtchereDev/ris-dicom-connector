.PHONY: help build run test clean docker-up docker-down lint install-deps

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

install-deps: ## Install Go dependencies
	go mod download
	go mod tidy

build: ## Build the application
	go build -o bin/dicom-connector cmd/server/main.go

run: ## Run the application
	go run cmd/server/main.go

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests with coverage report
	go tool cover -html=coverage.out

clean: ## Clean build artifacts
	rm -rf bin/ coverage.out

docker-up: ## Start docker services
	docker-compose -f deployments/docker-compose.yml up -d

docker-down: ## Stop docker services
	docker-compose -f deployments/docker-compose.yml down

docker-logs: ## View docker logs
	docker-compose -f deployments/docker-compose.yml logs -f

lint: ## Run linter
	golangci-lint run ./...

dev: docker-up ## Start development environment
	@echo "Waiting for services to be ready..."
	@sleep 10
	$(MAKE) run

.DEFAULT_GOAL := help