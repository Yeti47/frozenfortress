#!/bin/bash

# Release script for Frozen Fortress Linux builds
# This script creates a clean build and packages it for distribution
# Usage: ./release-linux.sh --arch ARCHITECTURE --version VERSION [--debug] [--notesseract] 

set -e  # Exit on any error

ARCHITECTURE=""
VERSION=""
DEBUG_MODE=false
NO_TESSERACT=false
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_DIR="releases"

# Function to show usage
show_usage() {
    echo "Usage: $0 --arch ARCHITECTURE --version VERSION [--debug] [--notesseract]"
    echo ""
    echo "Required parameters:"
    echo "  --arch ARCHITECTURE    Target architecture (e.g., amd64, 386, arm64)"
    echo "  --version VERSION      Version string (e.g., 1.0.0)"
    echo ""
    echo "Optional parameters:"
    echo "  --debug               Build with debug symbols"
    echo "  --notesseract         Build without Tesseract OCR support"
    echo ""
    echo "Examples:"
    echo "  $0 --arch amd64 --version 1.0.0"
    echo "  $0 --arch 386 --version 1.0.0 --debug"
    echo "  $0 --arch arm64 --version 1.0.0 --notesseract"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --arch)
            if [[ -n "$2" && "$2" != --* ]]; then
                ARCHITECTURE="$2"
                shift 2
            else
                echo "Error: --arch requires an architecture string"
                show_usage
                exit 1
            fi
            ;;
        --version)
            if [[ -n "$2" && "$2" != --* ]]; then
                VERSION="$2"
                shift 2
            else
                echo "Error: --version requires a version string"
                show_usage
                exit 1
            fi
            ;;
        --debug)
            DEBUG_MODE=true
            shift
            ;;
        --notesseract)
            NO_TESSERACT=true
            shift
            ;;
        --platform)
            echo "Error: --platform is no longer supported. The target platform is always 'linux'."
            exit 1
            ;;
        --help|-h)
            show_usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Validate required parameters
if [ -z "$ARCHITECTURE" ]; then
    echo "Error: --arch parameter is required"
    show_usage
    exit 1
fi

if [ -z "$VERSION" ]; then
    echo "Error: --version parameter is required"
    show_usage
    exit 1
fi

# Validate architecture
case "$ARCHITECTURE" in
    amd64|386|arm64|arm)
        ;;
    x86)
        ARCHITECTURE="386"
        echo "Note: Converting x86 to 386 (Go architecture name)"
        ;;
    x64)
        ARCHITECTURE="amd64"
        echo "Note: Converting x64 to amd64 (Go architecture name)"
        ;;
    *)
        echo "Warning: Uncommon architecture '$ARCHITECTURE'. Supported: amd64, 386, arm64, arm"
        echo "Continuing anyway..."
        ;;
esac

echo "=========================================="
echo "Creating Frozen Fortress Release"
echo "=========================================="
echo "Platform: linux"
echo "Architecture: $ARCHITECTURE"
echo "Version: $VERSION"
echo "Debug mode: $DEBUG_MODE"
echo "No Tesseract: $NO_TESSERACT"
echo "=========================================="

# Create release directory structure
RELEASE_NAME="frozenfortress-linux-${ARCHITECTURE}-v${VERSION}"
if [ "$DEBUG_MODE" = true ]; then
    RELEASE_NAME="${RELEASE_NAME}-debug"
fi
if [ "$NO_TESSERACT" = true ]; then
    RELEASE_NAME="${RELEASE_NAME}-notesseract"
fi

TARGET_DIR="${RELEASE_DIR}/${RELEASE_NAME}"

echo "Creating release directory: ${TARGET_DIR}"
mkdir -p "${TARGET_DIR}"

# Step 1: Clean previous builds
echo ""
echo "Step 1: Cleaning previous builds..."
./clean.sh

# Step 2: Build the application
echo ""
echo "Step 2: Building application..."
BUILD_ARGS="--arch ${ARCHITECTURE} --version ${VERSION} --platform linux"
if [ "$DEBUG_MODE" = true ]; then
    BUILD_ARGS="${BUILD_ARGS} --debug"
fi
if [ "$NO_TESSERACT" = true ]; then
    BUILD_ARGS="${BUILD_ARGS} --notesseract"
fi

echo "Running: ./build-all.sh ${BUILD_ARGS}"
./build-all.sh ${BUILD_ARGS}

# Step 3: Verify binaries were created
echo ""
echo "Step 3: Verifying binaries..."
if [ ! -f "bin/ffcli" ] && [ ! -f "bin/ffcli-debug" ]; then
    echo "Error: CLI binary not found in bin/ directory"
    exit 1
fi

if [ ! -f "bin/webui" ] && [ ! -f "bin/webui-debug" ]; then
    echo "Error: WebUI binary not found in bin/ directory"
    exit 1
fi

echo "Binaries verified successfully"

# Step 4: Create the release package
echo ""
echo "Step 4: Creating release package..."

# Copy bin directory contents to release directory
cp -r bin/* "${TARGET_DIR}/"

# Create additional release files
echo "Creating README for release..."
cat > "${TARGET_DIR}/README.txt" << EOF
Frozen Fortress v${VERSION} - linux ${ARCHITECTURE}

This package contains the Frozen Fortress application binaries for linux ${ARCHITECTURE}.

Contents:
- ffcli / ffcli-debug: Command Line Interface
- webui / webui-debug: Web User Interface
- img/: Image assets
- views/: Web UI templates
- static/: Static web assets (if present)

Usage:
1. Web UI: ./webui
2. CLI: ./ffcli --help

For more information, visit: https://github.com/Yeti47/frozenfortress

Build Information:
- Version: ${VERSION}
- Platform: linux
- Architecture: ${ARCHITECTURE}
- Build Date: $(date)
- Debug Mode: ${DEBUG_MODE}
- Tesseract OCR: $([ "$NO_TESSERACT" = true ] && echo "Disabled" || echo "Enabled")
EOF

# Step 5: Create ZIP archive
echo ""
echo "Step 5: Creating ZIP archive..."
cd "${RELEASE_DIR}"
ZIP_NAME="${RELEASE_NAME}.zip"

if command -v zip > /dev/null 2>&1; then
    echo "Creating ${ZIP_NAME}..."
    zip -r "${ZIP_NAME}" "${RELEASE_NAME}/"
    echo "ZIP archive created: ${RELEASE_DIR}/${ZIP_NAME}"
else
    echo "Warning: 'zip' command not found. Creating tar.gz instead..."
    TAR_NAME="${RELEASE_NAME}.tar.gz"
    tar -czf "${TAR_NAME}" "${RELEASE_NAME}/"
    echo "TAR archive created: ${RELEASE_DIR}/${TAR_NAME}"
fi

cd "${SCRIPT_DIR}"

# Step 6: Display summary
echo ""
echo "=========================================="
echo "Release Creation Complete!"
echo "=========================================="
echo "Release name: ${RELEASE_NAME}"
echo "Release directory: ${TARGET_DIR}"
if command -v zip > /dev/null 2>&1; then
    echo "Archive: ${RELEASE_DIR}/${ZIP_NAME}"
    echo "Archive size: $(du -h "${RELEASE_DIR}/${ZIP_NAME}" | cut -f1)"
else
    echo "Archive: ${RELEASE_DIR}/${TAR_NAME}"
    echo "Archive size: $(du -h "${RELEASE_DIR}/${TAR_NAME}" | cut -f1)"
fi
echo ""
echo "Files in release:"
ls -la "${TARGET_DIR}/"
echo ""
echo "Release ready for distribution!"
