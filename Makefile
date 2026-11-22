.PHONY: all build build-server build-all build-rust run-daemon build-web build-full dev up down logs ai-bridge clean test test-unit bench run deps fmt lint

# Variables
BINARY_NAME=fx
VERSION=2.0.0
BUILD_DIR=dist

# Build flags
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Default target
all: build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	@go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/fx

build-server:
	@echo "Building $(BINARY_NAME) server..."
	@mkdir -p bin
	@go build $(LDFLAGS) -o bin/$(BINARY_NAME)-server ./cmd/server

# Build Rust Turbo Daemon (typst-renderd)
build-rust:
	@echo "Building typst-renderd (Rust Turbo Daemon)..."
	@mkdir -p bin
	@cd rust/typst-renderd && cargo build --release
	@cp rust/typst-renderd/target/release/typst-renderd bin/typst-renderd
	@chmod +x bin/typst-renderd
	@echo "Build complete: bin/typst-renderd"

# Run Rust Turbo Daemon locally on Unix socket
run-daemon:
	@echo "Starting typst-renderd on /tmp/typst-renderd.sock..."
	@./bin/typst-renderd --socket /tmp/typst-renderd.sock

# Run entire stack locally in 1 terminal (typst-renderd + fx-server + Angular dev)
dev:
	@./scripts/dev.sh

# Docker Compose shortcuts (1 command for typst-renderd + fx-server + ng-web)
up:
	@docker compose up --build

down:
	@docker compose down

logs:
	@docker compose logs -f

# Expose local AI CLI (agy, claude, gemini...) over OpenAI-compatible HTTP for Docker Compose
ai-bridge:
	@go run ./cmd/ai-cli-bridge

# Build Angular 21 production web bundle
build-web:
	@echo "Building Angular production bundle..."
	@cd ng_web && npm run build -- --configuration production

# Build Go CLI + Go server + Angular Web UI
build-full: build build-server build-web
	@echo "Full build complete (CLI + server + web UI)!"

# Build for all platforms
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/fx
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/fx
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/fx
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/fx
	@echo "Build complete! Binaries in $(BUILD_DIR)/"

# Clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR) bin/ $(BINARY_NAME) $(BINARY_NAME)-server server
	@go clean
	@echo "Clean complete!"

# Run full tests with coverage
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Tests complete! Coverage report: coverage.html"

# Run fast race-checked unit test suite (sandbox-friendly, no Snap Typst required)
test-unit:
	@echo "Running unit tests with race detector..."
	@XDG_CONFIG_HOME=/tmp go test -race ./pkg/ui/... ./pkg/blueprint/... ./pkg/schema/... ./pkg/template/... ./pkg/render/... ./pkg/registry/... ./pkg/cii/... ./pkg/invoice/... ./pkg/validation/... ./internal/...

# Run fX v2 benchmark CLI
bench:
	@go run ./cmd/bench

# Run
run:
	@go run ./cmd/fx

# Install dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies installed!"

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -s -w .
	@echo "Format complete!"

# Lint
lint:
	@echo "Running linter..."
	@golangci-lint run
	@echo "Lint complete!"
