.DEFAULT_GOAL := help
.PHONY: help setup install-staticcheck fmt vet test test-race cover build clean lint hooks lint-fix check

GO ?= go
BINARY ?= dots
STATICCHECK ?= staticcheck
STATICCHECK_VERSION := v0.8.2

setup: ## Check development dependencies and install local git hooks
	@command -v "$(GO)" >/dev/null 2>&1 || { printf '%s\n' 'Install Go from https://go.dev/dl/ before running make setup' >&2; exit 1; }
	@command -v prek >/dev/null 2>&1 || { printf '%s\n' 'Install prek first: https://prek.j178.dev/installation/' >&2; exit 1; }
	@command -v "$(STATICCHECK)" >/dev/null 2>&1 || { printf '%s\n' "Staticcheck $(STATICCHECK_VERSION) is missing; run make install-staticcheck and add GOBIN (or GOPATH/bin) to PATH" >&2; exit 1; }
	prek install
	prek install --hook-type commit-msg

install-staticcheck: ## Install the pinned Staticcheck release into the Go binary directory
	GOTOOLCHAIN=auto $(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)

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

lint: ## Run Staticcheck across the entire Go module without fixes
	@command -v "$(STATICCHECK)" >/dev/null 2>&1 || { printf '%s\n' "Staticcheck $(STATICCHECK_VERSION) is missing; run make install-staticcheck and add GOBIN (or GOPATH/bin) to PATH" >&2; exit 1; }
	"$(STATICCHECK)" ./...

hooks: ## Run all configured prek hooks against the repository
	prek run --all-files

lint-fix: ## Run auto-fixing whitespace hooks
	prek run trailing-whitespace --all-files
	prek run end-of-file-fixer --all-files

check: fmt vet test build lint hooks ## Run all local quality checks

clean: ## Remove locally generated build and coverage outputs
	rm -f $(BINARY) $(BINARY).exe coverage.out coverage.html

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
