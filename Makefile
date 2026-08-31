.PHONY: all build install clean test run daemon dev deps fmt lint release help

# Build variables
BINARY_NAME=qube
INSTALL_PATH=/usr/local/bin
SERVICE_FILE=qubed.service
SERVICE_PATH=/etc/systemd/system

# Qube uses Linux namespaces/cgroups, so builds target Linux by default.
TARGET_OS?=linux
TARGET_ARCH?=amd64
CGO_ENABLED?=0
HOST_OS:=$(shell go env GOHOSTOS)

# Go build flags
GO=go
GOFLAGS=-ldflags="-s -w"
GOCMD=CGO_ENABLED=$(CGO_ENABLED) GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) $(GO) build $(GOFLAGS)

ifeq ($(HOST_OS),windows)
TESTCMD=GOOS=linux $(GO) test ./... -exec=true
else
TESTCMD=$(GO) test -v -covermode=count -coverprofile=coverage.out ./...
endif

all: build

# Build the binary
build:
	@echo "Building Qube for $(TARGET_OS)/$(TARGET_ARCH)..."
	@$(GOCMD) -o $(BINARY_NAME) ./
	@echo "OK Build complete: ./$(BINARY_NAME)"
	@if [ "$(HOST_OS)" = "windows" ]; then \
		echo "  Note: Qube is Linux-only; this is a Linux binary for WSL/Linux hosts."; \
	fi

# Install the binary and service
install: build
	@echo "Installing Qube..."
	@sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@sudo chmod u+s $(INSTALL_PATH)/$(BINARY_NAME)
	@ls -la $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "OK Installed to $(INSTALL_PATH)/$(BINARY_NAME)"
	@if [ -f $(SERVICE_FILE) ]; then \
		sudo cp $(SERVICE_FILE) $(SERVICE_PATH)/$(SERVICE_FILE); \
		sudo systemctl daemon-reload; \
		echo "OK Service file installed to $(SERVICE_PATH)/$(SERVICE_FILE)"; \
		echo "  Run 'sudo systemctl start qubed' to start the daemon"; \
	fi

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -rf bin/
	@echo "OK Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@$(TESTCMD)

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
	@$(GO) build -race -o $(BINARY_NAME) ./

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "OK Dependencies updated"

# Format code
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...
	@echo "OK Code formatted"

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
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./
	@echo "OK Release binaries built in ./bin/"

# Help
help:
	@echo "Qube Makefile Commands:"
	@echo "  make build    - Build the Linux binary"
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
