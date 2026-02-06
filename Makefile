.PHONY: all build install clean test run daemon help

# Build variables
BINARY_NAME=qube
INSTALL_PATH=/usr/local/bin
SERVICE_FILE=qubed.service
SERVICE_PATH=/etc/systemd/system

# Go build flags
GO=go
GOFLAGS=-ldflags="-s -w"
GOCMD=$(GO) build $(GOFLAGS)

all: build

# Build the binary
build:
	@echo "Building Qube..."
	@$(GOCMD) -o $(BINARY_NAME) ./cmd/qube
	@echo "✓ Build complete: ./$(BINARY_NAME)"

# Install the binary and service
install: build
	@echo "Installing Qube..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo chmod u+s $(INSTALL_PATH)/$(BINARY_NAME)
	@ls -la $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "✓ Installed to $(INSTALL_PATH)/$(BINARY_NAME)"
	@if [ -f $(SERVICE_FILE) ]; then \
		sudo cp $(SERVICE_FILE) $(SERVICE_PATH)/$(SERVICE_FILE); \
		sudo systemctl daemon-reload; \
		echo "✓ Service file installed to $(SERVICE_PATH)/$(SERVICE_FILE)"; \
		echo "  Run 'sudo systemctl start qubed' to start the daemon"; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -rf bin/
	@echo "✓ Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@$(GO) test -v -covermode=count -coverprofile=coverage.out ./...

# Run the daemon in debug mode
daemon: build
	@echo "Starting Qube daemon in debug mode..."
	@sudo ./$(BINARY_NAME) daemon --debug

# Run container
run: build
	@echo "Usage: make run CMD='--image Ubuntu24_NODE --cmd \"npm start\"'"

# Development build with race detector
dev:
	@echo "Building with race detector..."
	@$(GO) build -race -o $(BINARY_NAME) ./cmd/qube

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "✓ Dependencies updated"

# Format code
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...
	@echo "✓ Code formatted"

# Lint code
lint:
	@echo "Linting code..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Build for multiple platforms
release:
	@echo "Building release binaries..."
	@mkdir -p bin
	@GOOS=linux GOARCH=amd64 $(GOCMD) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/qube
	@GOOS=linux GOARCH=arm64 $(GOCMD) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/qube
	@echo "✓ Release binaries built in ./bin/"

# Help
help:
	@echo "Qube Makefile Commands:"
	@echo "  make build    - Build the binary"
	@echo "  make install  - Install binary and systemd service"
	@echo "  make clean    - Remove build artifacts"
	@echo "  make test     - Run tests"
	@echo "  make daemon   - Run daemon in debug mode"
	@echo "  make dev      - Build with race detector"
	@echo "  make deps     - Download and tidy dependencies"
	@echo "  make fmt      - Format code"
	@echo "  make lint     - Lint code (requires golangci-lint)"
	@echo "  make release  - Build for multiple platforms"
	@echo "  make help     - Show this help message"
