#!/bin/bash

# Build script for Frozen Fortress CLI
# This script builds the CLI application and places it in the bin directory
# Usage: ./build-cli.sh [--debug] [--notesseract] [--version VERSION]

set -e  # Exit on any error

DEBUG_MODE=false
NO_TESSERACT=false
BUILD_FLAGS=""
BUILD_TAGS=""
VERSION=""
OUTPUT_BINARY="ffcli"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --debug)
            DEBUG_MODE=true
            BUILD_FLAGS="-gcflags=all=\"-N -l\""
            OUTPUT_BINARY="ffcli-debug"
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
    echo "Building Frozen Fortress CLI (DEBUG MODE)..."
    echo "Debug symbols enabled, optimizations disabled"
else
    echo "Building Frozen Fortress CLI (RELEASE MODE)..."
fi

if [ "$NO_TESSERACT" = true ]; then
    echo "Building without Tesseract OCR support..."
fi

if [ -n "$VERSION" ]; then
    echo "Building with version: $VERSION"
fi

# Create bin directory if it doesn't exist
mkdir -p bin

# Build the CLI application
echo "Compiling CLI application..."

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

BUILD_CMD="$BUILD_CMD -o bin/$OUTPUT_BINARY ./cli"

# Execute the build command
echo "Running: $BUILD_CMD"
eval $BUILD_CMD

echo "Build completed successfully!"
echo "CLI binary created at: bin/$OUTPUT_BINARY"
echo ""
echo "To run the CLI:"
echo "  ./bin/$OUTPUT_BINARY --help"
