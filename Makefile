.PHONY: help test test-short fmt lint lint-fix check pre-commit install-hooks fuzz fuzz-quick e2e build quality quality-baseline clean

RUNNER=go run ./tools/runner.go

help: ## Show available targets
	@$(RUNNER) help

test: ## Run all tests
	@$(RUNNER) test

test-short: ## Run tests without race detector
	@$(RUNNER) test-short

fmt: ## Format all Go code
	@$(RUNNER) fmt

lint: ## Run linter
	@$(RUNNER) lint

lint-fix: ## Run linter and auto-fix issues
	@$(RUNNER) lint-fix

check: ## Run format, lint, and quick tests
	@$(RUNNER) check

pre-commit: ## Same as check - run before committing
	@$(RUNNER) pre-commit

install-hooks: ## Install git pre-commit hooks
	@$(RUNNER) install-hooks

fuzz: ## Run fuzz tests for 1 minute each
	@$(RUNNER) fuzz

fuzz-quick: ## Run fuzz tests quickly (5s each) for CI/validation
	@$(RUNNER) fuzz-quick

e2e: ## Run end-to-end tests
	@$(RUNNER) e2e

build: ## Build example binaries
	@$(RUNNER) build

quality: ## Run enforceable quality gates
	@$(RUNNER) quality

quality-baseline: ## Capture benchmark baseline snapshot
	@$(RUNNER) quality-baseline

clean: ## Clean build artifacts and caches
	@$(RUNNER) clean
