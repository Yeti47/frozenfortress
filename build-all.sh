#!/bin/bash

# Build script for all Frozen Fortress components
# This script builds both the WebUI and CLI applications
# Usage: ./build-all.sh [--debug] [--notesseract]

set -e  # Exit on any error

DEBUG_MODE=false
NO_TESSERACT=false
BUILD_FLAGS=""
BUILD_TAGS=""
WEBUI_BINARY="webui"
CLI_BINARY="ffcli"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --debug)
            DEBUG_MODE=true
            BUILD_FLAGS="-gcflags=all=-N -l"
            WEBUI_BINARY="webui-debug"
            CLI_BINARY="ffcli-debug"
            shift
            ;;
        --notesseract)
            NO_TESSERACT=true
            BUILD_TAGS="notesseract"
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--debug] [--notesseract]"
            exit 1
            ;;
    esac
done

echo "Building all Frozen Fortress components..."

if [ "$DEBUG_MODE" = true ]; then
    echo "Building in DEBUG MODE (debug symbols enabled, optimizations disabled)"
fi

if [ "$NO_TESSERACT" = true ]; then
    echo "Building without Tesseract OCR support..."
fi

# Create bin directory if it doesn't exist
mkdir -p bin

# Construct the build command base
BUILD_CMD_BASE="go build"

if [ -n "$BUILD_TAGS" ]; then
    BUILD_CMD_BASE="$BUILD_CMD_BASE -tags $BUILD_TAGS"
fi

if [ "$DEBUG_MODE" = true ]; then
    BUILD_CMD_BASE="$BUILD_CMD_BASE $BUILD_FLAGS"
fi

# Build WebUI
echo "Compiling WebUI application..."
WEBUI_BUILD_CMD="$BUILD_CMD_BASE -o bin/$WEBUI_BINARY ./webui"
echo "Running: $WEBUI_BUILD_CMD"
eval $WEBUI_BUILD_CMD

# Build CLI
echo "Compiling CLI application..."
CLI_BUILD_CMD="$BUILD_CMD_BASE -o bin/$CLI_BINARY ./cli"
echo "Running: $CLI_BUILD_CMD"
eval $CLI_BUILD_CMD

# Copy webui assets to bin directory (only if WebUI was built)
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

echo ""
echo "Build completed successfully!"
echo "Binaries created:"
echo "  - WebUI: bin/$WEBUI_BINARY"
echo "  - CLI:   bin/$CLI_BINARY"
echo ""
echo "To run:"
echo "  WebUI: ./bin/$WEBUI_BINARY"
echo "  CLI:   ./bin/$CLI_BINARY --help"
