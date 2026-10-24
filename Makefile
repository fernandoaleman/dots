.DEFAULT_GOAL := help
.PHONY: help setup fmt vet test test-race cover build clean lint lint-fix check

GO ?= go
BINARY ?= dots

setup: ## Install local git hooks (requires prek on PATH)
	@command -v prek >/dev/null 2>&1 || { echo 'Install prek first: https://prek.j178.dev/'; exit 1; }
	prek install
	prek install --hook-type commit-msg

fmt: ## Format Go code
	$(GO) fmt ./...

vet: ## Run Go static analysis
	$(GO) vet ./...

test: ## Run Go unit tests
	$(GO) test ./...

test-race: ## Run race-enabled Go tests (requires CGO compiler)
	$(GO) test -race ./...

cover: ## Generate test coverage report
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

build: ## Build dots for the host OS/architecture
	$(GO) build -o $(BINARY) .

lint: ## Run prek hooks against the repository
	prek run --all-files

lint-fix: ## Run auto-fixing whitespace hooks
	prek run trailing-whitespace --all-files
	prek run end-of-file-fixer --all-files

check: fmt vet test lint ## Run local quality checks

clean: ## Remove locally generated build and coverage outputs
	rm -f $(BINARY) $(BINARY).exe coverage.out coverage.html

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
