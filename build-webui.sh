#!/bin/bash

# Build script for Frozen Fortress WebUI
# This script builds the webui application and places it in the bin directory

set -e  # Exit on any error

echo "Building Frozen Fortress WebUI..."

# Create bin directory if it doesn't exist
mkdir -p bin

# Build the webui application
echo "Compiling webui application..."
go build -o bin/webui ./webui

echo "Build completed successfully!"
echo "WebUI binary created at: bin/webui"
echo ""
echo "To run the application:"
echo "  ./bin/webui"
echo ""
echo "Or use the run script:"
echo "  ./run-webui.sh"
