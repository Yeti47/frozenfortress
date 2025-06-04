#!/bin/bash

# Cleanup script for Frozen Fortress
# This script removes all compiled binaries from the bin directory

set -e  # Exit on any error

echo "Cleaning up Frozen Fortress binaries..."

# Check if bin directory exists
if [ -d "bin" ]; then
    echo "Removing all files from bin/ directory..."
    rm -rf bin/*
    echo "Cleanup completed successfully!"
    echo "bin/ directory is now empty"
else
    echo "bin/ directory does not exist - nothing to clean"
fi

echo "Done."
