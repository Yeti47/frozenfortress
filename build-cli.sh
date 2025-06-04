#!/bin/bash

# Build script for Frozen Fortress CLI
# This script builds the CLI application and places it in the bin directory

set -e  # Exit on any error

echo "Building Frozen Fortress CLI..."

# Create bin directory if it doesn't exist
mkdir -p bin

# Build the CLI application
echo "Compiling CLI application..."
go build -o bin/ffcli ./cli

echo "Build completed successfully!"
echo "CLI binary created at: bin/ffcli"
echo ""
echo "To run the CLI:"
echo "  ./bin/ffcli --help"
