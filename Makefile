.PHONY: help test lint fmt check pre-commit install-hooks clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

test: ## Run all tests
	go test ./pkg/... -v -race -coverprofile=coverage.txt

test-short: ## Run tests without race detector
	go test ./pkg/... -short

fmt: ## Format all Go code
	gofmt -w -s .
	goimports -w .

lint: ## Run linter
	golangci-lint run --config=.golangci.yml ./...

lint-fix: ## Run linter and auto-fix issues
	golangci-lint run --config=.golangci.yml --fix ./...

check: fmt lint test-short ## Run format, lint, and quick tests (pre-commit check)

pre-commit: check ## Same as check - run before committing
	@echo "✅ Pre-commit checks passed!"

install-hooks: ## Install git pre-commit hooks
	chmod +x .git/hooks/pre-commit
	@echo "✅ Pre-commit hooks installed"

fuzz: ## Run fuzz tests for 1 minute each
	go test ./pkg/bridge -v -fuzz=FuzzWebSocketConnWrite -fuzztime=60s
	go test ./pkg/bridge -v -fuzz=FuzzWebSocketConnRead -fuzztime=60s
	go test ./pkg/bridge -v -fuzz=FuzzBinaryMessage -fuzztime=60s
	go test ./pkg/bridge -v -fuzz=FuzzMessageSizes -fuzztime=60s

e2e: ## Run end-to-end tests
	go test ./e2e/... -v -timeout 5m

build: ## Build example binaries
	go build -v ./examples/direct-bridge
	go build -v ./examples/grpc-server
	cd examples/wasm-client && GOOS=js GOARCH=wasm go build -v .

clean: ## Clean build artifacts and caches
	go clean -cache -testcache -modcache
	rm -f coverage.txt
	rm -f direct-bridge grpc-server
	rm -f examples/wasm-client/main.wasm
