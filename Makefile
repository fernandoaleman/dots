.DEFAULT_GOAL := help

# ── Setup ──────────────────────────────────────────────
# Must work on a bare machine BEFORE anything is installed, so it depends on
# nothing this repo provides.
.PHONY: setup
setup: ## Install prek and activate git hooks
	@command -v prek >/dev/null 2>&1 || { \
		echo "Installing prek..."; \
		if command -v mise >/dev/null 2>&1; then \
			mise use -g prek; \
		elif command -v brew >/dev/null 2>&1; then \
			brew install prek; \
		else \
			curl --proto '=https' --tlsv1.2 -LsSf https://github.com/j178/prek/releases/latest/download/prek-installer.sh | sh; \
		fi; \
	}
	prek install
	prek install --hook-type commit-msg
	@echo "Done. Hooks are active."

.PHONY: ssh
ssh: ## Switch the origin remote from HTTPS to SSH
	@url=$$(git remote get-url origin); \
	case "$$url" in \
		https://github.com/*) \
			new="git@github.com:$${url#https://github.com/}"; \
			git remote set-url origin "$$new"; \
			echo "origin -> $$new" ;; \
		git@github.com:*) \
			echo "origin already SSH: $$url" ;; \
		*) \
			echo "unrecognised origin: $$url" >&2; exit 1 ;; \
	esac

# ── Lint ───────────────────────────────────────────────
.PHONY: lint
lint: ## Run all pre-commit hooks on every file
	prek run --all-files

.PHONY: lint-fix
lint-fix: ## Run fixable hooks (trailing-whitespace, end-of-file-fixer)
	prek run trailing-whitespace end-of-file-fixer --all-files

# ── Help ───────────────────────────────────────────────
.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
