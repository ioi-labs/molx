.PHONY: default help setup build run clean lint test release

APP_NAME := molx
# Default to amd64 for local builds. Override for other arches, e.g.:
#   make deps OBSCURA_BIN=../obscura/target/release/obscura OBSCURA_ARCH=arm64
OBSCURA_BIN := ../obscura/target/release/obscura
OBSCURA_ARCH := amd64
DEPS_DIR := deps

# Obscura upstream release used by setup and CI.
OBSCURA_VERSION := v0.1.10
OBSCURA_RELEASE_URL := https://github.com/h4ckf0r0day/obscura/releases/download/$(OBSCURA_VERSION)

# Show help menu when running `make` without arguments.
default: help

help: ## Show this help menu
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

setup: ## Download Obscura binaries for the current OS/arch into deps/obscura
	@echo "Setting up Obscura $(OBSCURA_VERSION) for local development..."
	@os=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	arch=$$(uname -m); \
	case $$os in \
		linux)   goos="linux"; asset_os="linux" ;; \
		darwin)  goos="darwin"; asset_os="macos" ;; \
		*)       echo "Unsupported OS: $$os"; exit 1 ;; \
	esac; \
	case $$arch in \
		x86_64|amd64)  goarch="amd64"; asset="obscura-x86_64-$$asset_os.tar.gz" ;; \
		arm64|aarch64) goarch="arm64"; asset="obscura-aarch64-$$asset_os.tar.gz" ;; \
		*)             echo "Unsupported architecture: $$arch"; exit 1 ;; \
	esac; \
	target_dir="$(DEPS_DIR)/obscura/$$goos/$$goarch"; \
	mkdir -p "$$target_dir"; \
	if [ -f "$$target_dir/obscura" ] && [ -f "$$target_dir/obscura-worker" ]; then \
		echo "Obscura binaries already present in $$target_dir"; \
	else \
		echo "Downloading $$asset..."; \
		curl -fsSL -o /tmp/obscura.tar.gz "$(OBSCURA_RELEASE_URL)/$$asset"; \
		tar -xzf /tmp/obscura.tar.gz -C "$$target_dir"; \
		chmod +x "$$target_dir/obscura" "$$target_dir/obscura-worker"; \
		rm -f /tmp/obscura.tar.gz; \
		echo "Obscura binaries ready in $$target_dir"; \
	fi

build: deps ## Build the Go binary
	go build -o $(APP_NAME) .

run: build ## Build and run the server (loads .env if present)
	@if [ -f .env ]; then \
		set -a; . ./.env; set +a; \
	fi; \
	./$(APP_NAME)

deps: ## Copy Obscura binary into deps/obscura/linux/<arch>/
	@echo "Copying Obscura binary for $(OBSCURA_ARCH)..."
	@mkdir -p $(DEPS_DIR)/obscura/linux/$(OBSCURA_ARCH)
	@if [ -f "$(OBSCURA_BIN)" ]; then \
		cp "$(OBSCURA_BIN)" $(DEPS_DIR)/obscura/linux/$(OBSCURA_ARCH)/obscura; \
	else \
		echo "Warning: $(OBSCURA_BIN) not found"; \
	fi
	@if [ -f "$(OBSCURA_BIN)-worker" ]; then \
		cp "$(OBSCURA_BIN)-worker" $(DEPS_DIR)/obscura/linux/$(OBSCURA_ARCH)/obscura-worker; \
	fi
	@chmod +x $(DEPS_DIR)/obscura/linux/$(OBSCURA_ARCH)/obscura || true
	@chmod +x $(DEPS_DIR)/obscura/linux/$(OBSCURA_ARCH)/obscura-worker || true

clean: ## Remove built binary and copied dependencies
	@echo "Cleaning..."
	@rm -f $(APP_NAME)
	@rm -f $(DEPS_DIR)/obscura

lint: ## Run go vet and static analysis
	go vet ./...

test: ## Run Go tests
	go test ./...

release: clean deps ## Build an optimized release binary
	go build -ldflags="-s -w" -o $(APP_NAME) .
