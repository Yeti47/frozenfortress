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
RELEASE_NAME="frozenfortress-release-linux-${ARCHITECTURE}-v${VERSION}"
if [ "$DEBUG_MODE" = true ]; then
    RELEASE_NAME="${RELEASE_NAME}-debug"
fi
if [ "$NO_TESSERACT" = true ]; then
    RELEASE_NAME="${RELEASE_NAME}-notesseract"
fi

# Create the main release directory
RELEASE_ROOT_DIR="${RELEASE_DIR}/${RELEASE_NAME}"
# Create subdirectory for the actual binaries (keep original naming for the zip content)
BINARIES_DIR_NAME="frozenfortress-linux-${ARCHITECTURE}-v${VERSION}"
TARGET_DIR="${RELEASE_ROOT_DIR}/${BINARIES_DIR_NAME}"

echo "Creating release directory: ${RELEASE_ROOT_DIR}"
mkdir -p "${RELEASE_ROOT_DIR}"
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

if [ ! -f "bin/ffwebui" ] && [ ! -f "bin/ffwebui-debug" ]; then
    echo "Error: WebUI binary not found in bin/ directory"
    exit 1
fi

echo "Binaries verified successfully"

# Step 4: Create the release package
echo ""
echo "Step 4: Creating release package..."

# Copy bin directory contents to binaries directory
cp -r bin/* "${TARGET_DIR}/"

# Copy setup script to the main release directory
echo "Copying setup script..."
cp ff-setup.sh "${RELEASE_ROOT_DIR}/"
chmod +x "${RELEASE_ROOT_DIR}/ff-setup.sh"

# Create README for the main release directory
echo "Creating main README for release..."
cat > "${RELEASE_ROOT_DIR}/README.txt" << EOF
Frozen Fortress v${VERSION} - linux ${ARCHITECTURE}

This package contains the Frozen Fortress application release for linux ${ARCHITECTURE}.

Contents:
- ${BINARIES_DIR_NAME}.zip: Application binaries and assets
- ff-setup.sh: Automated setup script
- README.txt: This file

Quick Start:
1. Run the setup script: ./ff-setup.sh
   This will install dependencies, extract the application, and optionally configure nginx.

Manual Installation:
1. Unzip ${BINARIES_DIR_NAME}.zip to your preferred location
2. Install dependencies: tesseract-ocr, leptonica, redis-server
3. Run ./ffcli setup to configure the application
4. Start with ./ffwebui

For more information, visit: https://github.com/Yeti47/frozenfortress

Build Information:
- Version: ${VERSION}
- Platform: linux
- Architecture: ${ARCHITECTURE}
- Build Date: $(date)
- Debug Mode: ${DEBUG_MODE}
- Tesseract OCR: $([ "$NO_TESSERACT" = true ] && echo "Disabled" || echo "Enabled")
EOF

# Create detailed README for the binaries directory
echo "Creating detailed README for binaries..."
cat > "${TARGET_DIR}/README.txt" << EOF
Frozen Fortress v${VERSION} - Application Binaries

Contents:
- ffcli / ffcli-debug: Command Line Interface
- ffwebui / ffwebui-debug: Web User Interface
- img/: Image assets
- views/: Web UI templates
- static/: Static web assets (if present)

Usage:
1. Web UI: ./ffwebui
2. CLI: ./ffcli --help

Configuration:
Run './ffcli setup' to configure the application before first use.

Important Directories (created after first run):
- Database: ~/.config/frozenfortress/
- Logs: ~/.config/frozenfortress/logs/
- Backups: ~/.config/frozenfortress/backups/
- Session keys: ~/.config/frozenfortress/ (or custom FF_KEY_DIR)

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

# Create ZIP of the binaries directory first
BINARIES_ZIP_NAME="${BINARIES_DIR_NAME}.zip"

if command -v zip > /dev/null 2>&1; then
    echo "Creating ${BINARIES_ZIP_NAME}..."
    cd "${RELEASE_NAME}"
    zip -r "${BINARIES_ZIP_NAME}" "${BINARIES_DIR_NAME}/"
    cd ..
    echo "Binaries ZIP archive created: ${RELEASE_ROOT_DIR}/${BINARIES_ZIP_NAME}"
    
    # Remove the unzipped binaries directory since we now have the zip
    rm -rf "${RELEASE_NAME}/${BINARIES_DIR_NAME}/"
    
    # Now create the main release zip (same name as the folder)
    RELEASE_ZIP_NAME="${RELEASE_NAME}.zip"
    
    echo "Creating main release archive: ${RELEASE_ZIP_NAME}..."
    zip -r "${RELEASE_ZIP_NAME}" "${RELEASE_NAME}/"
    echo "Release ZIP archive created: ${RELEASE_DIR}/${RELEASE_ZIP_NAME}"
    
    # Remove the release folder since we only need the zip for distribution
    rm -rf "${RELEASE_NAME}/"
else
    echo "Warning: 'zip' command not found. Creating tar.gz instead..."
    BINARIES_TAR_NAME="${BINARIES_DIR_NAME}.tar.gz"
    cd "${RELEASE_NAME}"
    tar -czf "${BINARIES_TAR_NAME}" "${BINARIES_DIR_NAME}/"
    cd ..
    echo "Binaries TAR archive created: ${RELEASE_ROOT_DIR}/${BINARIES_TAR_NAME}"
    
    # Remove the unzipped binaries directory since we now have the tar
    rm -rf "${RELEASE_NAME}/${BINARIES_DIR_NAME}/"
    
    # Now create the main release tar (same name as the folder)
    RELEASE_TAR_NAME="${RELEASE_NAME}.tar.gz"
    
    echo "Creating main release archive: ${RELEASE_TAR_NAME}..."
    tar -czf "${RELEASE_TAR_NAME}" "${RELEASE_NAME}/"
    echo "Release TAR archive created: ${RELEASE_DIR}/${RELEASE_TAR_NAME}"
    
    # Remove the release folder since we only need the tar for distribution
    rm -rf "${RELEASE_NAME}/"
fi

cd "${SCRIPT_DIR}"

# Step 6: Display summary
echo ""
echo "=========================================="
echo "Release Creation Complete!"
echo "=========================================="
echo "Release name: ${RELEASE_NAME}"

if command -v zip > /dev/null 2>&1; then
    echo "Main release archive: ${RELEASE_DIR}/${RELEASE_ZIP_NAME}"
    echo "Archive size: $(du -h "${RELEASE_DIR}/${RELEASE_ZIP_NAME}" | cut -f1)"
else
    echo "Main release archive: ${RELEASE_DIR}/${RELEASE_TAR_NAME}"
    echo "Archive size: $(du -h "${RELEASE_DIR}/${RELEASE_TAR_NAME}" | cut -f1)"
fi

echo ""
echo "Release structure inside archive:"
echo "${RELEASE_NAME}/"
echo "├── README.txt"
echo "├── ff-setup.sh"
if command -v zip > /dev/null 2>&1; then
    echo "└── ${BINARIES_ZIP_NAME}"
else
    echo "└── ${BINARIES_TAR_NAME}"
fi
echo ""
if command -v zip > /dev/null 2>&1; then
    echo "To distribute: Share ${RELEASE_ZIP_NAME}"
    echo "Users extract and run: cd ${RELEASE_NAME} && ./ff-setup.sh"
else
    echo "To distribute: Share ${RELEASE_TAR_NAME}"
    echo "Users extract and run: cd ${RELEASE_NAME} && ./ff-setup.sh"
fi
echo ""
echo "Release ready for distribution!"
