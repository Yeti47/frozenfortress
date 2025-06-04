#!/bin/bash

# Run script for Frozen Fortress WebUI
# This script builds and runs the webui application

set -e  # Exit on any error

echo "Starting Frozen Fortress WebUI..."

# Build first
./build-webui.sh

echo ""
echo "Starting WebUI server..."
echo "Press Ctrl+C to stop the server"
echo ""

# Run the webui application
./bin/webui
