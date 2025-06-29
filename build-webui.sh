#!/bin/bash

# Build script for Frozen Fortress WebUI
# This script builds the webui application and places it in the bin directory
# Usage: ./build-webui.sh [--debug] [--notesseract] [--version VERSION]

set -e  # Exit on any error

DEBUG_MODE=false
NO_TESSERACT=false
BUILD_FLAGS=""
BUILD_TAGS=""
VERSION=""
OUTPUT_BINARY="webui"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --debug)
            DEBUG_MODE=true
            BUILD_FLAGS="-gcflags=all=\"-N -l\""
            OUTPUT_BINARY="webui-debug"
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
                echo "Usage: $0 [--debug] [--notesseract] [--version VERSION]"
                exit 1
            fi
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--debug] [--notesseract] [--version VERSION]"
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

if [ "$NO_TESSERACT" = true ]; then
    echo "Building without Tesseract OCR support..."
fi

if [ -n "$VERSION" ]; then
    echo "Building with version: $VERSION"
fi

# Create bin directory if it doesn't exist
mkdir -p bin

# Build the webui application
echo "Compiling webui application..."

# Construct the build command
BUILD_CMD="go build"

if [ -n "$BUILD_TAGS" ]; then
    BUILD_CMD="$BUILD_CMD -tags $BUILD_TAGS"
fi

if [ "$DEBUG_MODE" = true ]; then
    BUILD_CMD="$BUILD_CMD $BUILD_FLAGS"
fi

# Add ldflags for version if specified
if [ -n "$VERSION" ]; then
    BUILD_CMD="$BUILD_CMD -ldflags \"-X github.com/Yeti47/frozenfortress/frozenfortress/core/ccc.AppVersion=$VERSION\""
fi

BUILD_CMD="$BUILD_CMD -o bin/$OUTPUT_BINARY ./webui"

# Execute the build command
echo "Running: $BUILD_CMD"
eval $BUILD_CMD

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
echo "WebUI binary created at: bin/$OUTPUT_BINARY"
echo ""
echo "To run the application:"
echo "  ./bin/$OUTPUT_BINARY"
echo ""
echo "Or use the run script:"
echo "  ./run-webui.sh"
