#!/bin/bash
# Build script for LinyapsManager
# Creates server binary and client with symlinks for allowed commands

set -e

BUILD_DIR="build"
CLIENT_BINARY="linyaps-client"
SERVER_BINARY="linyaps-dbus-server"

# Allowed command symlinks
SYMLINKS=(
    "ll-cli"
    "killall"
    "kill"
    "pkexec"
)

echo "=== Building LinyapsManager ==="

# Create build directory
mkdir -p "$BUILD_DIR"

# Build server
echo "Building server..."
go build -o "$BUILD_DIR/$SERVER_BINARY" ./cmd/server

# Build client
echo "Building client..."
go build -o "$BUILD_DIR/$CLIENT_BINARY" ./cmd/client

# Create symlinks for allowed commands
echo "Creating command symlinks..."
cd "$BUILD_DIR"
for cmd in "${SYMLINKS[@]}"; do
    rm -f "$cmd"
    ln -s "$CLIENT_BINARY" "$cmd"
    echo "  Created symlink: $cmd -> $CLIENT_BINARY"
done
cd ..

echo ""
echo "=== Build complete ==="
echo "Server:  $BUILD_DIR/$SERVER_BINARY"
echo "Client:  $BUILD_DIR/$CLIENT_BINARY"
echo "Commands:"
for cmd in "${SYMLINKS[@]}"; do
    echo "  - $BUILD_DIR/$cmd"
done
echo ""
echo "Usage:"
echo "  1. Start server: ./$BUILD_DIR/$SERVER_BINARY"
echo "  2. Use commands: ./$BUILD_DIR/ll-cli list"
echo "                   ./$BUILD_DIR/killall firefox"
