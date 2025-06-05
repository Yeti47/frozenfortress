#!/bin/bash

# Run script for Frozen Fortress WebUI
# This script builds and runs the webui application
# Usage: ./run-webui.sh [--debug]

set -e  # Exit on any error

DEBUG_MODE=false
BINARY_NAME="webui"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --debug)
            DEBUG_MODE=true
            BINARY_NAME="webui-debug"
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
    echo "Starting Frozen Fortress WebUI (DEBUG MODE)..."
    export FF_LOG_LEVEL=debug
else
    echo "Starting Frozen Fortress WebUI..."
fi

# First, stop any existing webui processes
echo ""
echo "Checking for existing webui processes..."
if command -v ./stop-webui.sh >/dev/null 2>&1; then
    ./stop-webui.sh
else
    echo "Warning: stop-webui.sh not found, proceeding without cleanup"
fi

# Build first
echo ""
if [ "$DEBUG_MODE" = true ]; then
    echo "Building application with debug symbols..."
    ./build-webui.sh --debug
else
    echo "Building application..."
    ./build-webui.sh
fi

echo ""
if [ "$DEBUG_MODE" = true ]; then
    echo "Starting WebUI server in DEBUG mode..."
    echo "Debug logging enabled, debug symbols included"
else
    echo "Starting WebUI server..."
fi
echo "Press Ctrl+C to stop the server"
echo ""

# Change to bin directory and run the webui application
# This ensures the webui can find its assets (views, img, static) that were copied during build
cd bin && ./$BINARY_NAME
