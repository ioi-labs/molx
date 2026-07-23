.PHONY: default help build run clean lint test release

APP_NAME := molx
# Default to amd64 for local builds. Override for other arches, e.g.:
#   make deps OBSCURA_BIN=../obscura/target/release/obscura OBSCURA_ARCH=arm64
OBSCURA_BIN := ../obscura/target/release/obscura
OBSCURA_ARCH := amd64
DEPS_DIR := deps

# Show help menu when running `make` without arguments.
default: help

help: ## Show this help menu
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

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
