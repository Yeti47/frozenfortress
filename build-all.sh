#!/bin/bash

# Build script for all Frozen Fortress components
# This script builds both the WebUI and CLI applications

set -e  # Exit on any error

echo "Building all Frozen Fortress components..."

# Create bin directory if it doesn't exist
mkdir -p bin

# Build WebUI
echo "Compiling WebUI application..."
go build -o bin/webui ./webui

# Build CLI
echo "Compiling CLI application..."
go build -o bin/ffcli ./cli

echo ""
echo "Build completed successfully!"
echo "Binaries created:"
echo "  - WebUI: bin/webui"
echo "  - CLI:   bin/ffcli"
echo ""
echo "To run:"
echo "  WebUI: ./bin/webui"
echo "  CLI:   ./bin/ffcli --help"
