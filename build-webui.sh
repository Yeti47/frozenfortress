#!/bin/bash

# Build script for Frozen Fortress WebUI
# This script builds the webui application and places it in the bin directory
# Usage: ./build-webui.sh [--debug]

set -e  # Exit on any error

DEBUG_MODE=false
BUILD_FLAGS=""
OUTPUT_BINARY="webui"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --debug)
            DEBUG_MODE=true
            BUILD_FLAGS="-gcflags=all=-N -l"
            OUTPUT_BINARY="webui-debug"
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--debug]"
            exit 1
            ;;
    esac
done

if [ "$DEBUG_MODE" = true ]; then
    echo "Building Frozen Fortress WebUI (DEBUG MODE)..."
    echo "Debug symbols enabled, optimizations disabled"
else
    echo "Building Frozen Fortress WebUI (RELEASE MODE)..."
fi

# Create bin directory if it doesn't exist
mkdir -p bin

# Build the webui application
echo "Compiling webui application..."
if [ "$DEBUG_MODE" = true ]; then
    go build $BUILD_FLAGS -o bin/$OUTPUT_BINARY ./webui
else
    go build -o bin/$OUTPUT_BINARY ./webui
fi

# Copy webui assets to bin directory
echo "Copying webui assets..."

# Copy img directory
if [ -d "webui/img" ]; then
    cp -r webui/img bin/
    echo "  Copied img directory"
fi

# Copy static directory
if [ -d "webui/static" ]; then
    cp -r webui/static bin/
    echo "  Copied static directory"
fi

# Copy views directory (excluding .go files)
if [ -d "webui/views" ]; then
    # Create views directory in bin
    mkdir -p bin/views
    
    # Copy all files except .go files using rsync
    rsync -av --exclude="*.go" webui/views/ bin/views/
    
    echo "  Copied views directory (excluding .go files)"
fi

echo "Build completed successfully!"
echo "WebUI binary created at: bin/webui"
echo ""
echo "To run the application:"
echo "  ./bin/webui"
echo ""
echo "Or use the run script:"
echo "  ./run-webui.sh"
