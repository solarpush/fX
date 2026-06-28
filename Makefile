.PHONY: build build-server build-all clean test run

# Variables
BINARY_NAME=fx
VERSION=0.1.0
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

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Tests complete! Coverage report: coverage.html"

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
	@go fmt ./...
	@echo "Format complete!"

# Lint
lint:
	@echo "Running linter..."
	@golangci-lint run
	@echo "Lint complete!"
