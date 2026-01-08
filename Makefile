# Makefile for LinyapsManager
# Builds server binary and client with symlinks for allowed commands

.PHONY: all server client symlinks release clean test install uninstall help

# Build configuration
BUILD_DIR := build
CLIENT_BINARY := linyapsctl
SERVER_BINARY := linyaps-dbus-server
CMD_SERVER := ./cmd/server
CMD_CLIENT := ./cmd/client

# Allowed command symlinks
SYMLINKS := ll-cli killall kill pkexec

# Go build flags
GO := go
GOFLAGS := -v
GOMODFLAGS ?= -mod=vendor
TRIMPATH ?=
# Strip debug info (-s) and DWARF (-w)
LDFLAGS := -s -w
# Release build flags (same as regular build for consistent hashes)
RELEASE_LDFLAGS := -s -w
RELEASE_TAGS :=

# Default target
all: server client symlinks
	@echo ""
	@echo "=== Build complete ==="
	@echo "Server:  $(BUILD_DIR)/$(SERVER_BINARY)"
	@echo "Client:  $(BUILD_DIR)/$(CLIENT_BINARY)"
	@echo "Commands:"
	@for cmd in $(SYMLINKS); do \
		echo "  - $(BUILD_DIR)/$$cmd"; \
	done
	@echo ""
	@echo "Usage:"
	@echo "  1. Start server: ./$(BUILD_DIR)/$(SERVER_BINARY)"
	@echo "  2. Use commands: ./$(BUILD_DIR)/ll-cli list"
	@echo "                   ./$(BUILD_DIR)/killall firefox"

# Create build directory
$(BUILD_DIR):
	@mkdir -p $(BUILD_DIR)

# Build server
server: $(BUILD_DIR)
	@echo "Building server..."
	@$(GO) build $(GOMODFLAGS) $(TRIMPATH) $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(SERVER_BINARY) $(CMD_SERVER)

# Build client
client: $(BUILD_DIR)
	@echo "Building client..."
	@$(GO) build $(GOMODFLAGS) $(TRIMPATH) $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(CLIENT_BINARY) $(CMD_CLIENT)

# Create symlinks for allowed commands
symlinks: client
	@echo "Creating command symlinks..."
	@cd $(BUILD_DIR) && \
	for cmd in $(SYMLINKS); do \
		rm -f $$cmd; \
		ln -s $(CLIENT_BINARY) $$cmd; \
		echo "  Created symlink: $$cmd -> $(CLIENT_BINARY)"; \
	done

# Build release artifacts into OUTDIR with GOOS/GOARCH suffixes
OUTDIR ?= out
release:
	@echo "Building release artifacts (optimized)..."
	@mkdir -p $(OUTDIR)
	@if [ -z "$(GOOS)" ] || [ -z "$(GOARCH)" ]; then \
		echo "Error: GOOS/GOARCH must be set (e.g. GOOS=linux GOARCH=amd64)"; \
		exit 2; \
	fi
	@echo "Building server with flags: -trimpath -ldflags '$(RELEASE_LDFLAGS)' -tags '$(RELEASE_TAGS)'"
	@CGO_ENABLED=1 $(GO) build $(GOMODFLAGS) -trimpath -ldflags "$(RELEASE_LDFLAGS)" -tags "$(RELEASE_TAGS)" $(GOFLAGS) -o $(OUTDIR)/$(SERVER_BINARY)-$(GOOS)-$(GOARCH) $(CMD_SERVER)
	@echo "Building client with flags: -trimpath -ldflags '$(RELEASE_LDFLAGS)' -tags '$(RELEASE_TAGS)'"
	@CGO_ENABLED=1 $(GO) build $(GOMODFLAGS) -trimpath -ldflags "$(RELEASE_LDFLAGS)" -tags "$(RELEASE_TAGS)" $(GOFLAGS) -o $(OUTDIR)/$(CLIENT_BINARY)-$(GOOS)-$(GOARCH) $(CMD_CLIENT)
	@echo "Build artifacts:"
	@ls -lh $(OUTDIR)/$(SERVER_BINARY)-$(GOOS)-$(GOARCH) $(OUTDIR)/$(CLIENT_BINARY)-$(GOOS)-$(GOARCH) 2>/dev/null || true

# Run tests
test:
	@echo "Running tests..."
	@$(GO) test ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"


# Show help
help:
	@echo "LinyapsManager Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  make           - Build everything (default)"
	@echo "  make server    - Build server only"
	@echo "  make client    - Build client only"
	@echo "  make symlinks  - Create command symlinks"
	@echo "  make release   - Build GOOS/GOARCH artifacts into OUTDIR (default out/)"
	@echo "  make test      - Run all tests"
	@echo "  make clean     - Remove build artifacts"
	@echo "  make install   - Install to /usr/local/bin (requires root)"
	@echo "  make uninstall - Remove from /usr/local/bin (requires root)"
	@echo "  make help      - Show this help message"
