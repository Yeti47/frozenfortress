# GitHub Actions for Frozen Fortress

This directory contains GitHub Actions workflows for automated building, testing, and releasing of Frozen Fortress.

## Workflows

### 1. Release Workflow (`release.yml`)

**Trigger:** When a new tag starting with `v` is pushed to the master branch

**What it does:**
- Automatically builds Frozen Fortress for Linux AMD64
- Creates a GitHub release with the version from the tag
- Uploads the release archive (same format as `release-linux.sh`)
- Includes detailed release notes

**Usage:**

**Option 1: Via GitHub Web Interface (Recommended):**
1. Go to your repository on GitHub
2. Click on "Releases" in the right sidebar
3. Click "Create a new release"
4. Choose "Create new tag" and enter version (e.g., `v1.0.0`)
5. Make sure the target is set to `master` branch
6. Add release title and description
7. Click "Publish release"

**Option 2: Via Command Line:**
```bash
# Create and push a new version tag
git tag v1.0.0
git push origin v1.0.0
```

**Requirements:**
- Tag must start with `v` (e.g., `v1.0.0`, `v2.1.3`)
- Tag must be pushed to the master branch
- Repository must have GitHub Actions enabled

**Output:**
- Creates a GitHub release at: `https://github.com/your-repo/releases`
- Release includes a ZIP file with the same structure as manual releases
- Automatic release notes with build information

### 2. CI Workflow (`ci.yml`)

**Trigger:** Push to master/main/develop branches or pull requests

**What it does:**
- Tests building the application on Ubuntu
- Runs tests (if any exist)
- Builds with and without Tesseract support
- Verifies binary creation and executability

**Purpose:**
- Validates that code changes don't break the build
- Tests different build configurations
- Provides feedback on pull requests

## Dependencies Installed

Both workflows automatically install the following system dependencies:
- Go 1.24.3
- Tesseract OCR with English language pack
- Tesseract development libraries
- Leptonica development libraries  
- Redis server
- zip utility (for releases)

## Secrets Required

The release workflow requires the following GitHub secret:
- `GITHUB_TOKEN` - Automatically provided by GitHub Actions

## File Structure

The release workflow creates the same file structure as the manual `release-linux.sh` script:

```
frozenfortress-release-linux-amd64-v1.0.0.zip
├── README.txt
├── ff-setup.sh
└── frozenfortress-linux-amd64-v1.0.0.zip
    ├── ffcli
    ├── ffwebui
    ├── img/
    ├── views/
    └── README.txt
```

## Customization

### Adding More Architectures

To build for multiple architectures, modify the release workflow to use a matrix strategy:

```yaml
strategy:
  matrix:
    arch: [amd64, arm64, 386]
```

### Adding Different Build Modes

To create debug builds or builds without Tesseract:

```yaml
- name: Create debug release
  run: |
    ./release-linux.sh --arch amd64 --version ${{ steps.extract_version.outputs.version }} --debug

- name: Create no-tesseract release  
  run: |
    ./release-linux.sh --arch amd64 --version ${{ steps.extract_version.outputs.version }} --notesseract
```

### Adding More Operating Systems

To build for Windows or macOS, add additional jobs with different runners:

```yaml
jobs:
  release-linux:
    runs-on: ubuntu-latest
    # ... existing linux build
    
  release-windows:
    runs-on: windows-latest
    # ... windows build steps
    
  release-macos:
    runs-on: macos-latest
    # ... macOS build steps
```

## Troubleshooting

### Release Not Created
- Ensure the tag starts with `v`
- Check that the tag was pushed to the master branch
- Verify GitHub Actions are enabled for the repository

### Build Failures
- Check the Actions tab for detailed error logs
- Ensure all required files (build scripts) are committed
- Verify Go module dependencies are properly defined

### Permission Errors
- The `GITHUB_TOKEN` is automatically provided
- For custom tokens, ensure proper repository permissions

## Testing Locally

To test the build process locally before pushing tags:

```bash
# Test the build process
./build-all.sh --arch amd64 --version "test-1.0.0"

# Test the release creation
./release-linux.sh --arch amd64 --version "test-1.0.0"
```
