#!/bin/bash

# Build script for all Frozen Fortress components
# This script builds both the WebUI and CLI applications
# Usage: ./build-all.sh [--debug] [--notesseract] [--version VERSION] [--arch ARCHITECTURE] [--platform PLATFORM]

set -e  # Exit on any error

DEBUG_MODE=false
NO_TESSERACT=false
BUILD_FLAGS=""
BUILD_TAGS=""
VERSION=""
ARCHITECTURE=""
PLATFORM=""
WEBUI_BINARY="ffwebui"
CLI_BINARY="ffcli"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --debug)
            DEBUG_MODE=true
            BUILD_FLAGS="-gcflags=all=\"-N -l\""
            WEBUI_BINARY="ffwebui-debug"
            CLI_BINARY="ffcli-debug"
            shift
            ;;
        --notesseract)
            NO_TESSERACT=true
            BUILD_TAGS="notesseract"
            shift
            ;;
        --version)
            if [[ -n "$2" && "$2" != --* ]]; then
                VERSION="$2"
                shift 2
            else
                echo "Error: --version requires a version string"
                echo "Usage: $0 [--debug] [--notesseract] [--version VERSION] [--arch ARCHITECTURE] [--platform PLATFORM]"
                exit 1
            fi
            ;;
        --arch)
            if [[ -n "$2" && "$2" != --* ]]; then
                ARCHITECTURE="$2"
                shift 2
            else
                echo "Error: --arch requires an architecture string (e.g., amd64, 386, arm64)"
                echo "Usage: $0 [--debug] [--notesseract] [--version VERSION] [--arch ARCHITECTURE] [--platform PLATFORM]"
                exit 1
            fi
            ;;
        --platform)
            if [[ -n "$2" && "$2" != --* ]]; then
                PLATFORM="$2"
                shift 2
            else
                echo "Error: --platform requires a platform string (e.g., linux, windows, darwin)"
                echo "Usage: $0 [--debug] [--notesseract] [--version VERSION] [--arch ARCHITECTURE] [--platform PLATFORM]"
                exit 1
            fi
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--debug] [--notesseract] [--version VERSION] [--arch ARCHITECTURE] [--platform PLATFORM]"
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

if [ -n "$VERSION" ]; then
    echo "Building with version: $VERSION"
fi

if [ -n "$ARCHITECTURE" ]; then
    echo "Building for architecture: $ARCHITECTURE"
fi

if [ -n "$PLATFORM" ]; then
    echo "Building for platform: $PLATFORM"
fi

# Create bin directory if it doesn't exist
mkdir -p bin

# Construct the build command base
BUILD_CMD_BASE="go build"

# Set environment variables for cross-compilation if architecture is specified
if [ -n "$ARCHITECTURE" ]; then
    export GOARCH="$ARCHITECTURE"
fi

# Set environment variables for cross-compilation if platform is specified
if [ -n "$PLATFORM" ]; then
    export GOOS="$PLATFORM"
fi

if [ -n "$BUILD_TAGS" ]; then
    BUILD_CMD_BASE="$BUILD_CMD_BASE -tags $BUILD_TAGS"
fi

if [ "$DEBUG_MODE" = true ]; then
    BUILD_CMD_BASE="$BUILD_CMD_BASE $BUILD_FLAGS"
fi

# Add ldflags for version if specified
if [ -n "$VERSION" ]; then
    BUILD_CMD_BASE="$BUILD_CMD_BASE -ldflags \"-X github.com/Yeti47/frozenfortress/frozenfortress/core/ccc.AppVersion=$VERSION\""
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
