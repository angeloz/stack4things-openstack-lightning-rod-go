# Makefile for Lightning-rod (Go)
# Cross-compilation support for embedded devices

BINARY_NAME=lightning-rod
VERSION=1.0.0
BUILD_DIR=build
CMD_DIR=cmd/lightning-rod

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build flags
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"
CGO_ENABLED=0

.PHONY: all build clean test deps help

# Default target
all: build

# Build for current platform
build:
	@echo "Building for current platform..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for Linux AMD64
build-linux-amd64:
	@echo "Building for Linux AMD64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

# Build for Linux ARM (Raspberry Pi, etc.)
build-linux-arm:
	@echo "Building for Linux ARM..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm"

# Build for Linux ARM64 (Raspberry Pi 3/4)
build-linux-arm64:
	@echo "Building for Linux ARM64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64"

# Build for OpenWRT MIPS (common router platform)
build-openwrt-mips:
	@echo "Building for OpenWRT MIPS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=mips GOMIPS=softfloat CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-openwrt-mips $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-openwrt-mips"

# Build for OpenWRT MIPSLE (little-endian)
build-openwrt-mipsle:
	@echo "Building for OpenWRT MIPSLE..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=mipsle GOMIPS=softfloat CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-openwrt-mipsle $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-openwrt-mipsle"

# Build for OpenWRT ARM (many modern routers)
build-openwrt-arm:
	@echo "Building for OpenWRT ARM..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=$(CGO_ENABLED) $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-openwrt-arm $(CMD_DIR)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-openwrt-arm"

# Build all embedded targets
build-all-embedded: build-linux-arm build-linux-arm64 build-openwrt-mips build-openwrt-mipsle build-openwrt-arm
	@echo "All embedded builds complete!"

# Build all targets
build-all: build-linux-amd64 build-all-embedded
	@echo "All builds complete!"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies downloaded"

# Install binary to system
install: build
	@echo "Installing $(BINARY_NAME)..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installation complete"

# Uninstall binary from system
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstall complete"

# Run the binary
run: build
	@echo "Running $(BINARY_NAME)..."
	$(BUILD_DIR)/$(BINARY_NAME)

# Create release archives
release: build-all
	@echo "Creating release archives..."
	@mkdir -p $(BUILD_DIR)/release
	@cd $(BUILD_DIR) && tar czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd $(BUILD_DIR) && tar czf release/$(BINARY_NAME)-$(VERSION)-linux-arm.tar.gz $(BINARY_NAME)-linux-arm
	@cd $(BUILD_DIR) && tar czf release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@cd $(BUILD_DIR) && tar czf release/$(BINARY_NAME)-$(VERSION)-openwrt-mips.tar.gz $(BINARY_NAME)-openwrt-mips
	@cd $(BUILD_DIR) && tar czf release/$(BINARY_NAME)-$(VERSION)-openwrt-mipsle.tar.gz $(BINARY_NAME)-openwrt-mipsle
	@cd $(BUILD_DIR) && tar czf release/$(BINARY_NAME)-$(VERSION)-openwrt-arm.tar.gz $(BINARY_NAME)-openwrt-arm
	@echo "Release archives created in $(BUILD_DIR)/release/"

# Display size of binaries (useful for embedded targets)
size:
	@echo "Binary sizes:"
	@ls -lh $(BUILD_DIR)/ | grep $(BINARY_NAME) | awk '{print $$9 " - " $$5}'

# Help target
help:
	@echo "Lightning-rod (Go) - Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  build              - Build for current platform"
	@echo "  build-linux-amd64  - Build for Linux AMD64"
	@echo "  build-linux-arm    - Build for Linux ARM (Raspberry Pi 2/3)"
	@echo "  build-linux-arm64  - Build for Linux ARM64 (Raspberry Pi 3/4)"
	@echo "  build-openwrt-mips - Build for OpenWRT MIPS"
	@echo "  build-openwrt-mipsle - Build for OpenWRT MIPSLE"
	@echo "  build-openwrt-arm  - Build for OpenWRT ARM"
	@echo "  build-all-embedded - Build all embedded targets"
	@echo "  build-all          - Build all targets"
	@echo "  test               - Run tests"
	@echo "  clean              - Remove build artifacts"
	@echo "  deps               - Download dependencies"
	@echo "  install            - Install binary to /usr/local/bin"
	@echo "  uninstall          - Remove binary from /usr/local/bin"
	@echo "  run                - Build and run"
	@echo "  release            - Create release archives"
	@echo "  size               - Display binary sizes"
	@echo "  help               - Display this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  VERSION            - Version string (default: $(VERSION))"
	@echo "  BUILD_DIR          - Build output directory (default: $(BUILD_DIR))"
