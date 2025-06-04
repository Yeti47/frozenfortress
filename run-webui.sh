#!/bin/bash

# Run script for Frozen Fortress WebUI
# This script builds and runs the webui application

set -e  # Exit on any error

echo "Starting Frozen Fortress WebUI..."

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
echo "Building application..."
./build-webui.sh

echo ""
echo "Starting WebUI server..."
echo "Press Ctrl+C to stop the server"
echo ""

# Run the webui application
./bin/webui
